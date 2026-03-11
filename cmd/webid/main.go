// Command webid is the webi HTTP API server. It reads cached release
// data from the filesystem and serves release metadata, installer
// scripts, and bootstrap dispatches.
//
// It never fetches from upstream APIs — that's webicached's job.
// This server is stateless and fast: load from cache, resolve, render.
//
// Usage:
//
//	go run ./cmd/webid
//	go run ./cmd/webid -addr :3001 -cache ./_cache
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/webinstall/webi-installers/internal/buildmeta"
	"github.com/webinstall/webi-installers/internal/resolve"
	"github.com/webinstall/webi-installers/internal/storage"
	"github.com/webinstall/webi-installers/internal/storage/fsstore"
	"github.com/webinstall/webi-installers/internal/uadetect"
)

func main() {
	addr := flag.String("addr", ":3001", "listen address")
	cacheDir := flag.String("cache", "./_cache", "cache directory root")
	installersDir := flag.String("installers", ".", "installers repo root (for install.sh/ps1)")
	flag.Parse()

	store, err := fsstore.New(*cacheDir)
	if err != nil {
		log.Fatalf("fsstore: %v", err)
	}

	srv := &server{
		store:         store,
		installersDir: *installersDir,
		packages:      make(map[string]*packageCache),
	}

	// Pre-load all cached packages.
	srv.loadAll()

	mux := http.NewServeMux()

	// Legacy API routes (Node.js compat).
	mux.HandleFunc("GET /api/releases/{rest...}", srv.handleReleasesAPI)

	// Debug endpoint.
	mux.HandleFunc("GET /api/debug", srv.handleDebug)

	// Health check.
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	httpSrv := &http.Server{
		Addr:         *addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		log.Printf("webid listening on %s", *addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	httpSrv.Shutdown(shutCtx)
}

// server holds the shared state for all HTTP handlers.
type server struct {
	store         *fsstore.Store
	installersDir string

	mu       sync.RWMutex
	packages map[string]*packageCache
}

// packageCache holds a loaded package's assets and catalog.
type packageCache struct {
	assets  []storage.Asset
	dists   []resolve.Dist
	catalog resolve.Catalog
}

// loadAll pre-loads all packages from the cache directory.
func (s *server) loadAll() {
	monthDir := time.Now().Format("2006-01")
	dir := filepath.Join(s.store.Root(), monthDir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("warn: no cache dir %s: %v", dir, err)
		return
	}

	count := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		pkg := strings.TrimSuffix(e.Name(), ".json")
		pd, err := s.store.Load(context.Background(), pkg)
		if err != nil {
			log.Printf("warn: load %s: %v", pkg, err)
			continue
		}
		if pd == nil || len(pd.Assets) == 0 {
			continue
		}

		pc := &packageCache{
			assets: pd.Assets,
			dists:  assetsToDists(pd.Assets),
		}
		pc.catalog = resolve.Survey(pc.dists)

		s.mu.Lock()
		s.packages[pkg] = pc
		s.mu.Unlock()
		count++
	}
	log.Printf("loaded %d packages from cache", count)
}

// getPackage returns the cached package data, or nil if not found.
func (s *server) getPackage(pkg string) *packageCache {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.packages[pkg]
}

// assetsToDists converts storage.Asset slice to resolve.Dist slice.
func assetsToDists(assets []storage.Asset) []resolve.Dist {
	dists := make([]resolve.Dist, len(assets))
	for i, a := range assets {
		dists[i] = resolve.Dist{
			Filename: a.Filename,
			Version:  a.Version,
			LTS:      a.LTS,
			Channel:  a.Channel,
			Date:     a.Date,
			OS:       a.OS,
			Arch:     a.Arch,
			Libc:     a.Libc,
			Format:   a.Format,
			Download: a.Download,
			Extra:    a.Extra,
			Variants: a.Variants,
		}
	}
	return dists
}

// handleReleasesAPI serves /api/releases/{package}@{version}.{format}
func (s *server) handleReleasesAPI(w http.ResponseWriter, r *http.Request) {
	rest := r.PathValue("rest")

	// Parse: {package}@{version}.{json|tab} or {package}.{json|tab}
	pkg, version, format, err := parseReleasePath(rest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pc := s.getPackage(pkg)
	if pc == nil {
		// Check if it's a selfhosted package.
		if s.isSelfHosted(pkg) {
			s.serveEmptyReleases(w, format)
			return
		}
		http.Error(w, fmt.Sprintf("package %q not found", pkg), http.StatusNotFound)
		return
	}

	// Parse query parameters.
	q := r.URL.Query()
	osStr := q.Get("os")
	archStr := q.Get("arch")
	libcStr := q.Get("libc")
	ltsStr := q.Get("lts")
	channelStr := q.Get("channel")
	formatsStr := q.Get("formats")
	limitStr := q.Get("limit")

	// Normalize wildcard "-" to empty (means "any").
	if osStr == "-" {
		osStr = ""
	}
	if archStr == "-" {
		archStr = ""
	}
	if libcStr == "-" {
		libcStr = ""
	}

	// Map Node.js OS/arch names to our canonical names.
	osStr = normalizeQueryOS(osStr)
	archStr = normalizeQueryArch(archStr)

	// Parse LTS.
	lts := ltsStr == "true" || ltsStr == "1"

	// Handle channel selectors in the version field: @stable, @lts, @beta, etc.
	switch strings.ToLower(version) {
	case "stable", "latest":
		version = ""
		if channelStr == "" {
			channelStr = "stable"
		}
	case "lts":
		version = ""
		lts = true
	case "beta", "pre", "preview":
		version = ""
		if channelStr == "" {
			channelStr = "beta"
		}
	case "rc":
		version = ""
		if channelStr == "" {
			channelStr = "rc"
		}
	case "alpha", "dev":
		version = ""
		if channelStr == "" {
			channelStr = "alpha"
		}
	}

	// Parse formats list.
	var formats []string
	if formatsStr != "" {
		formats = strings.Split(formatsStr, ",")
	}

	// Parse limit.
	limit := 1000
	if limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	// Filter matching releases.
	filtered := filterDists(pc.dists, osStr, archStr, libcStr, channelStr, version, formats, lts, limit)

	switch format {
	case "json":
		s.serveJSON(w, r, pc, filtered)
	case "tab":
		s.serveTab(w, filtered)
	default:
		http.Error(w, "unsupported format: "+format, http.StatusBadRequest)
	}
}

// normalizeQueryOS maps Node.js OS names to our canonical names.
func normalizeQueryOS(s string) string {
	switch strings.ToLower(s) {
	case "macos", "mac":
		return "darwin"
	case "win":
		return "windows"
	default:
		return s
	}
}

// normalizeQueryArch maps Node.js arch names to our canonical names.
func normalizeQueryArch(s string) string {
	switch strings.ToLower(s) {
	case "amd64":
		return string(buildmeta.ArchAMD64) // "x86_64"
	case "arm64":
		return string(buildmeta.ArchARM64) // "aarch64"
	case "armv7l":
		return string(buildmeta.ArchARMv7)
	case "armv6l":
		return string(buildmeta.ArchARMv6)
	case "x86", "i386", "i686":
		return string(buildmeta.ArchX86)
	default:
		return s
	}
}

// parseReleasePath parses "{pkg}@{version}.{format}" or "{pkg}.{format}".
func parseReleasePath(rest string) (pkg, version, format string, err error) {
	if strings.HasSuffix(rest, ".json") {
		format = "json"
		rest = strings.TrimSuffix(rest, ".json")
	} else if strings.HasSuffix(rest, ".tab") {
		format = "tab"
		rest = strings.TrimSuffix(rest, ".tab")
	} else {
		return "", "", "", fmt.Errorf("unsupported format (use .json or .tab)")
	}

	if idx := strings.IndexByte(rest, '@'); idx >= 0 {
		pkg = rest[:idx]
		version = rest[idx+1:]
	} else {
		pkg = rest
	}

	if pkg == "" {
		return "", "", "", fmt.Errorf("package name required")
	}

	return pkg, version, format, nil
}

// filterDists filters dists by query parameters, returning all matches
// up to limit. This is for the API listing, not single-best resolution.
func filterDists(dists []resolve.Dist, osStr, archStr, libcStr, channel, version string, formats []string, lts bool, limit int) []resolve.Dist {
	var result []resolve.Dist

	for _, d := range dists {
		if osStr != "" && d.OS != osStr && d.OS != "*" && d.OS != "ANYOS" {
			continue
		}

		if archStr != "" && d.Arch != archStr && d.Arch != "*" && d.Arch != "ANYARCH" {
			continue
		}

		if libcStr != "" && d.Libc != "none" && d.Libc != "" && d.Libc != libcStr {
			continue
		}

		if lts && !d.LTS {
			continue
		}

		if channel != "" && d.Channel != channel {
			continue
		}

		if version != "" {
			// Match with or without "v" prefix:
			// query "0.25" should match version "v0.25.0".
			v := strings.TrimPrefix(d.Version, "v")
			vq := strings.TrimPrefix(version, "v")
			if !strings.HasPrefix(v, vq) {
				continue
			}
		}

		if len(formats) > 0 {
			matched := false
			for _, f := range formats {
				if strings.Contains(d.Format, f) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		result = append(result, d)
		if len(result) >= limit {
			break
		}
	}

	return result
}

// legacyRelease matches the Node.js JSON response format.
type legacyRelease struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	LTS      any    `json:"lts"`
	Channel  string `json:"channel"`
	Date     string `json:"date"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Libc     string `json:"libc,omitempty"`
	Ext      string `json:"ext"`
	Download string `json:"download"`
	Comment  string `json:"comment,omitempty"`
}

type legacyReleasesResponse struct {
	Releases []legacyRelease `json:"releases"`
	OSes     []string        `json:"oses,omitempty"`
	Arches   []string        `json:"arches,omitempty"`
	Libcs    []string        `json:"libcs,omitempty"`
	Formats  []string        `json:"formats,omitempty"`
}

func distsToLegacy(dists []resolve.Dist) []legacyRelease {
	releases := make([]legacyRelease, len(dists))
	for i, d := range dists {
		var lts any = d.LTS
		releases[i] = legacyRelease{
			Name:     d.Filename,
			Version:  d.Version,
			LTS:      lts,
			Channel:  d.Channel,
			Date:     d.Date,
			OS:       d.OS,
			Arch:     d.Arch,
			Libc:     d.Libc,
			Ext:      d.Format,
			Download: d.Download,
		}
	}
	return releases
}

func (s *server) serveJSON(w http.ResponseWriter, r *http.Request, pc *packageCache, filtered []resolve.Dist) {
	resp := legacyReleasesResponse{
		Releases: distsToLegacy(filtered),
		OSes:     pc.catalog.OSes,
		Arches:   pc.catalog.Arches,
		Formats:  pc.catalog.Formats,
	}

	w.Header().Set("Content-Type", "application/json")

	pretty := r.URL.Query().Get("pretty")
	if pretty == "true" || pretty == "1" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(resp)
	} else {
		json.NewEncoder(w).Encode(resp)
	}
}

func (s *server) serveTab(w http.ResponseWriter, filtered []resolve.Dist) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	// Tab format matches Node.js: version,lts,channel,date,os,arch,ext,-,download,name,comment
	for _, d := range filtered {
		lts := "false"
		if d.LTS {
			lts = "true"
		}
		fmt.Fprintf(w, "%s,%s,%s,%s,%s,%s,%s,-,%s,%s,\n",
			d.Version, lts, d.Channel, d.Date, d.OS, d.Arch, d.Format, d.Download, d.Filename)
	}
}

// serveEmptyReleases returns an empty release list for selfhosted packages.
func (s *server) serveEmptyReleases(w http.ResponseWriter, format string) {
	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(legacyReleasesResponse{
			Releases: []legacyRelease{},
		})
	case "tab":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
}

// isSelfHosted checks if a package has install.sh but no releases.conf.
func (s *server) isSelfHosted(pkg string) bool {
	installPath := filepath.Join(s.installersDir, pkg, "install.sh")
	if _, err := os.Stat(installPath); err != nil {
		return false
	}
	confPath := filepath.Join(s.installersDir, pkg, "releases.conf")
	if _, err := os.Stat(confPath); err == nil {
		return false
	}
	return true
}

// handleDebug returns UA detection info for the requesting client.
func (s *server) handleDebug(w http.ResponseWriter, r *http.Request) {
	result := uadetect.FromRequest(r)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"user_agent": r.Header.Get("User-Agent"),
		"os":         string(result.OS),
		"arch":       string(result.Arch),
		"libc":       string(result.Libc),
	})
}
