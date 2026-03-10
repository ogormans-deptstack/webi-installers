// Command webicached is the release cache daemon. It fetches releases
// from upstream sources, classifies build assets, and writes them to
// the _cache/ directory in the format the Node.js server expects.
//
// This is the Go replacement for the Node.js release-fetching pipeline.
// It reads releases.conf files to discover packages, fetches from the
// configured source, classifies assets, and writes to fsstore.
//
// Usage:
//
//	go run ./cmd/webicached
//	go run ./cmd/webicached -conf . -cache ./_cache -raw ./_cache/raw bat goreleaser
//	go run ./cmd/webicached -once          # single pass, no periodic refresh
//	go run ./cmd/webicached -once -no-fetch # classify from existing raw data only
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/webinstall/webi-installers/internal/classify"
	"github.com/webinstall/webi-installers/internal/installerconf"
	"github.com/webinstall/webi-installers/internal/rawcache"
	"github.com/webinstall/webi-installers/internal/releases/gitea"
	"github.com/webinstall/webi-installers/internal/releases/github"
	"github.com/webinstall/webi-installers/internal/releases/githubish"
	"github.com/webinstall/webi-installers/internal/releases/gittag"
	"github.com/webinstall/webi-installers/internal/releases/nodedist"
	"github.com/webinstall/webi-installers/internal/storage"
	"github.com/webinstall/webi-installers/internal/storage/fsstore"
)

func main() {
	confDir := flag.String("conf", ".", "root directory containing {pkg}/releases.conf files")
	cacheDir := flag.String("cache", "_cache", "output cache directory (fsstore root)")
	rawDir := flag.String("raw", "_cache/raw", "raw cache directory for upstream responses")
	token := flag.String("token", os.Getenv("GITHUB_TOKEN"), "GitHub API token")
	once := flag.Bool("once", false, "run once then exit (no periodic refresh)")
	noFetch := flag.Bool("no-fetch", false, "skip fetching, classify from existing raw data only")
	interval := flag.Duration("interval", 15*time.Minute, "refresh interval")
	flag.Parse()

	store, err := fsstore.New(*cacheDir)
	if err != nil {
		log.Fatalf("fsstore: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var auth *githubish.Auth
	if *token != "" {
		auth = &githubish.Auth{Token: *token}
	}

	filterPkgs := flag.Args()

	run := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		packages, err := discover(*confDir)
		if err != nil {
			log.Printf("discover: %v", err)
			return
		}

		if len(filterPkgs) > 0 {
			nameSet := make(map[string]bool, len(filterPkgs))
			for _, a := range filterPkgs {
				nameSet[a] = true
			}
			var filtered []pkgConf
			for _, p := range packages {
				if nameSet[p.name] {
					filtered = append(filtered, p)
				}
			}
			packages = filtered
		}

		log.Printf("refreshing %d packages", len(packages))

		for _, pkg := range packages {
			if alias := pkg.conf.Extra["alias_of"]; alias != "" {
				continue
			}

			err := refreshPackage(ctx, client, store, *rawDir, pkg, auth, *noFetch)
			if err != nil {
				log.Printf("  ERROR %s: %v", pkg.name, err)
			}
		}
	}

	run()
	if *once {
		return
	}

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	log.Printf("running every %s (ctrl-c to stop)", *interval)
	for range ticker.C {
		run()
	}
}

type pkgConf struct {
	name string
	conf *installerconf.Conf
}

func discover(dir string) ([]pkgConf, error) {
	pattern := filepath.Join(dir, "*", "releases.conf")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	var packages []pkgConf
	for _, path := range matches {
		name := filepath.Base(filepath.Dir(path))
		if strings.HasPrefix(name, "_") {
			continue
		}
		conf, err := installerconf.Read(path)
		if err != nil {
			log.Printf("warning: %s: %v", path, err)
			continue
		}
		packages = append(packages, pkgConf{name: name, conf: conf})
	}

	sort.Slice(packages, func(i, j int) bool {
		return packages[i].name < packages[j].name
	})
	return packages, nil
}

// refreshPackage does the full pipeline for one package:
// fetch raw → classify → write to fsstore.
func refreshPackage(ctx context.Context, client *http.Client, store *fsstore.Store, rawDir string, pkg pkgConf, auth *githubish.Auth, skipFetch bool) error {
	name := pkg.name
	conf := pkg.conf

	// Step 1: Fetch raw upstream data to rawcache (unless -no-fetch).
	if !skipFetch {
		if err := fetchRaw(ctx, client, rawDir, pkg, auth); err != nil {
			return fmt.Errorf("fetch: %w", err)
		}
	}

	// Step 2: Classify raw data into assets.
	d, err := rawcache.Open(filepath.Join(rawDir, name))
	if err != nil {
		return fmt.Errorf("rawcache open: %w", err)
	}

	assets, err := classifyPackage(name, conf, d)
	if err != nil {
		return fmt.Errorf("classify: %w", err)
	}

	// Step 3: Apply config transforms.
	assets = applyConfig(assets, conf)

	// Step 4: Write to fsstore.
	tx, err := store.BeginRefresh(ctx, name)
	if err != nil {
		return fmt.Errorf("begin refresh: %w", err)
	}
	if err := tx.Put(assets); err != nil {
		tx.Rollback()
		return fmt.Errorf("put: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	log.Printf("  %s: %d assets", name, len(assets))
	return nil
}

// applyConfig applies version prefix stripping and exclude filters.
func applyConfig(assets []storage.Asset, conf *installerconf.Conf) []storage.Asset {
	excludes := conf.Exclude
	prefixes := conf.VersionPrefixes

	var out []storage.Asset
	for _, a := range assets {
		// Exclude filter.
		skip := false
		for _, ex := range excludes {
			if strings.Contains(a.Filename, ex) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// Version prefix stripping.
		for _, p := range prefixes {
			if strings.HasPrefix(a.Version, p) {
				a.Version = strings.TrimPrefix(a.Version, p)
				break
			}
		}

		out = append(out, a)
	}
	return out
}

// --- Fetch raw ---

func fetchRaw(ctx context.Context, client *http.Client, rawDir string, pkg pkgConf, auth *githubish.Auth) error {
	switch pkg.conf.Source {
	case "github":
		return fetchGitHub(ctx, client, rawDir, pkg.name, pkg.conf, auth)
	case "nodedist":
		return fetchNodeDist(ctx, client, rawDir, pkg.name, pkg.conf)
	case "gittag":
		return fetchGitTag(ctx, rawDir, pkg.name, pkg.conf)
	case "gitea":
		return fetchGitea(ctx, client, rawDir, pkg.name, pkg.conf)
	default:
		// Sources not yet ported — skip silently for now.
		log.Printf("  %s: source %q not yet supported, skipping", pkg.name, pkg.conf.Source)
		return nil
	}
}

func fetchGitHub(ctx context.Context, client *http.Client, rawDir, pkgName string, conf *installerconf.Conf, auth *githubish.Auth) error {
	owner, repo := conf.Owner, conf.Repo
	if owner == "" || repo == "" {
		return fmt.Errorf("missing owner or repo")
	}

	d, err := rawcache.Open(filepath.Join(rawDir, pkgName))
	if err != nil {
		return err
	}

	tagPrefix := conf.TagPrefix
	for batch, err := range github.Fetch(ctx, client, owner, repo, auth) {
		if err != nil {
			return fmt.Errorf("github %s/%s: %w", owner, repo, err)
		}
		for _, rel := range batch {
			if rel.Draft {
				continue
			}
			tag := rel.TagName
			if tagPrefix != "" && !strings.HasPrefix(tag, tagPrefix) {
				continue
			}
			data, _ := json.Marshal(rel)
			d.Merge(tag, data)
		}
	}
	return nil
}

func fetchNodeDist(ctx context.Context, client *http.Client, rawDir, pkgName string, conf *installerconf.Conf) error {
	baseURL := conf.BaseURL
	if baseURL == "" {
		return fmt.Errorf("missing url")
	}

	d, err := rawcache.Open(filepath.Join(rawDir, pkgName))
	if err != nil {
		return err
	}

	for batch, err := range nodedist.Fetch(ctx, client, baseURL) {
		if err != nil {
			return err
		}
		for _, entry := range batch {
			data, _ := json.Marshal(entry)
			d.Merge(entry.Version, data)
		}
	}
	return nil
}

func fetchGitTag(ctx context.Context, rawDir, pkgName string, conf *installerconf.Conf) error {
	gitURL := conf.BaseURL
	if gitURL == "" {
		return fmt.Errorf("missing url")
	}

	d, err := rawcache.Open(filepath.Join(rawDir, pkgName))
	if err != nil {
		return err
	}

	repoDir := filepath.Join(rawDir, "_repos")
	os.MkdirAll(repoDir, 0o755)

	for batch, err := range gittag.Fetch(ctx, gitURL, repoDir) {
		if err != nil {
			return err
		}
		for _, entry := range batch {
			tag := entry.Version
			if tag == "" {
				tag = "HEAD-" + entry.CommitHash
			}
			data, _ := json.Marshal(entry)
			d.Merge(tag, data)
		}
	}
	return nil
}

func fetchGitea(ctx context.Context, client *http.Client, rawDir, pkgName string, conf *installerconf.Conf) error {
	baseURL, owner, repo := conf.BaseURL, conf.Owner, conf.Repo
	if baseURL == "" || owner == "" || repo == "" {
		return fmt.Errorf("missing base_url, owner, or repo")
	}

	d, err := rawcache.Open(filepath.Join(rawDir, pkgName))
	if err != nil {
		return err
	}

	for batch, err := range gitea.Fetch(ctx, client, baseURL, owner, repo, nil) {
		if err != nil {
			return err
		}
		for _, rel := range batch {
			if rel.Draft {
				continue
			}
			data, _ := json.Marshal(rel)
			d.Merge(rel.TagName, data)
		}
	}
	return nil
}

// --- Classify per source ---

func classifyPackage(pkg string, conf *installerconf.Conf, d *rawcache.Dir) ([]storage.Asset, error) {
	switch conf.Source {
	case "github":
		return classifyGitHub(pkg, conf, d)
	case "nodedist":
		return classifyNodeDist(pkg, conf, d)
	case "gittag":
		return classifyGitTag(pkg, d)
	case "gitea":
		return classifyGitea(pkg, conf, d)
	default:
		return nil, nil
	}
}

func readAllRaw(d *rawcache.Dir) (map[string][]byte, error) {
	active, err := d.ActivePath()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(active)
	if err != nil {
		return nil, err
	}
	result := make(map[string][]byte, len(entries))
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), "_") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(active, e.Name()))
		if err != nil {
			return nil, err
		}
		result[e.Name()] = data
	}
	return result, nil
}

// --- GitHub ---

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
	PublishedAt string    `json:"published_at"`
	Assets      []ghAsset `json:"assets"`
	TarballURL  string    `json:"tarball_url"`
	ZipballURL  string    `json:"zipball_url"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

func classifyGitHub(pkg string, conf *installerconf.Conf, d *rawcache.Dir) ([]storage.Asset, error) {
	tagPrefix := conf.TagPrefix
	releases, err := readAllRaw(d)
	if err != nil {
		return nil, err
	}

	var assets []storage.Asset
	for _, data := range releases {
		var rel ghRelease
		if err := json.Unmarshal(data, &rel); err != nil {
			continue
		}
		if rel.Draft {
			continue
		}

		version := rel.TagName
		if tagPrefix != "" {
			version = strings.TrimPrefix(version, tagPrefix)
		}

		channel := "stable"
		if rel.Prerelease {
			channel = "beta"
		}

		date := ""
		if len(rel.PublishedAt) >= 10 {
			date = rel.PublishedAt[:10]
		}

		for _, a := range rel.Assets {
			if isMetaAsset(a.Name) {
				continue
			}

			r := classify.Filename(a.Name)

			assets = append(assets, storage.Asset{
				Filename: a.Name,
				Version:  version,
				Channel:  channel,
				OS:       string(r.OS),
				Arch:     string(r.Arch),
				Libc:     string(r.Libc),
				Format:   string(r.Format),
				Download: a.BrowserDownloadURL,
				Date:     date,
			})
		}

		// Source tarballs for packages with no binary assets.
		if len(rel.Assets) == 0 {
			if rel.TarballURL != "" {
				assets = append(assets, storage.Asset{
					Filename: rel.TagName + ".tar.gz",
					Version:  version,
					Channel:  channel,
					Format:   ".tar.gz",
					Download: rel.TarballURL,
					Date:     date,
					Extra:    "source",
				})
			}
			if rel.ZipballURL != "" {
				assets = append(assets, storage.Asset{
					Filename: rel.TagName + ".zip",
					Version:  version,
					Channel:  channel,
					Format:   ".zip",
					Download: rel.ZipballURL,
					Date:     date,
					Extra:    "source",
				})
			}
		}
	}
	return assets, nil
}

// --- Node.js dist ---

type nodeEntry struct {
	Version string          `json:"version"`
	Date    string          `json:"date"`
	Files   []string        `json:"files"`
	LTS     json.RawMessage `json:"lts"`
}

func classifyNodeDist(pkg string, conf *installerconf.Conf, d *rawcache.Dir) ([]storage.Asset, error) {
	baseURL := conf.BaseURL
	releases, err := readAllRaw(d)
	if err != nil {
		return nil, err
	}

	var assets []storage.Asset
	for _, data := range releases {
		var entry nodeEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}

		lts := string(entry.LTS) != "false" && string(entry.LTS) != ""
		channel := "stable"
		ver := strings.TrimPrefix(entry.Version, "v")
		parts := strings.SplitN(ver, ".", 2)
		if len(parts) > 0 {
			major := 0
			fmt.Sscanf(parts[0], "%d", &major)
			if major%2 != 0 {
				channel = "beta"
			}
		}

		for _, file := range entry.Files {
			if file == "src" || file == "headers" {
				continue
			}
			expanded := expandNodeFile(pkg, entry.Version, channel, entry.Date, lts, baseURL, file)
			assets = append(assets, expanded...)
		}
	}
	return assets, nil
}

func expandNodeFile(pkg, version, channel, date string, lts bool, baseURL, file string) []storage.Asset {
	parts := strings.Split(file, "-")
	if len(parts) < 2 {
		return nil
	}

	osMap := map[string]string{
		"osx": "darwin", "linux": "linux", "win": "windows",
		"sunos": "sunos", "aix": "aix",
	}
	archMap := map[string]string{
		"x64": "x86_64", "x86": "x86", "arm64": "aarch64",
		"armv7l": "armv7", "armv6l": "armv6",
		"ppc64": "ppc64", "ppc64le": "ppc64le", "s390x": "s390x",
	}

	os_ := osMap[parts[0]]
	arch := archMap[parts[1]]
	if os_ == "" || arch == "" {
		return nil
	}

	libc := ""
	pkgType := ""
	if len(parts) > 2 {
		pkgType = parts[2]
	}

	var formats []string
	switch pkgType {
	case "musl":
		libc = "musl"
		formats = []string{".tar.gz", ".tar.xz"}
	case "tar":
		formats = []string{".tar.gz", ".tar.xz"}
	case "zip":
		formats = []string{".zip"}
	case "pkg":
		formats = []string{".pkg"}
	case "msi":
		formats = []string{".msi"}
	case "exe":
		formats = []string{".exe"}
	case "":
		formats = []string{".tar.gz", ".tar.xz"}
	default:
		return nil
	}

	if libc == "" && os_ == "linux" {
		libc = "gnu"
	}

	osPart := parts[0]
	if osPart == "osx" {
		osPart = "darwin"
	}
	archPart := parts[1]
	muslExtra := ""
	if libc == "musl" {
		muslExtra = "-musl"
	}

	var assets []storage.Asset
	for _, format := range formats {
		var filename string
		if format == ".msi" {
			filename = fmt.Sprintf("node-%s-%s%s%s", version, archPart, muslExtra, format)
		} else {
			filename = fmt.Sprintf("node-%s-%s-%s%s%s", version, osPart, archPart, muslExtra, format)
		}

		assets = append(assets, storage.Asset{
			Filename: filename,
			Version:  version,
			Channel:  channel,
			OS:       os_,
			Arch:     arch,
			Libc:     libc,
			Format:   format,
			Download: fmt.Sprintf("%s/%s/%s", baseURL, version, filename),
			LTS:      lts,
			Date:     date,
		})
	}
	return assets
}

// --- Git tag ---

type gitTagEntry struct {
	Version    string `json:"Version"`
	GitTag     string `json:"GitTag"`
	CommitHash string `json:"CommitHash"`
	Date       string `json:"Date"`
}

func classifyGitTag(pkg string, d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := readAllRaw(d)
	if err != nil {
		return nil, err
	}

	var assets []storage.Asset
	for _, data := range releases {
		var entry gitTagEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}

		date := ""
		if len(entry.Date) >= 10 {
			date = entry.Date[:10]
		}

		assets = append(assets, storage.Asset{
			Filename: entry.GitTag,
			Version:  entry.Version,
			Channel:  "stable",
			Format:   ".git",
			Download: entry.GitTag,
			Date:     date,
			Extra:    "commit:" + entry.CommitHash,
		})
	}
	return assets, nil
}

// --- Gitea ---

type giteaRelease struct {
	TagName     string       `json:"tag_name"`
	Prerelease  bool         `json:"prerelease"`
	Draft       bool         `json:"draft"`
	PublishedAt string       `json:"published_at"`
	Assets      []giteaAsset `json:"assets"`
}

type giteaAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

func classifyGitea(pkg string, conf *installerconf.Conf, d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := readAllRaw(d)
	if err != nil {
		return nil, err
	}

	var assets []storage.Asset
	for _, data := range releases {
		var rel giteaRelease
		if err := json.Unmarshal(data, &rel); err != nil {
			continue
		}
		if rel.Draft {
			continue
		}

		channel := "stable"
		if rel.Prerelease {
			channel = "beta"
		}
		date := ""
		if len(rel.PublishedAt) >= 10 {
			date = rel.PublishedAt[:10]
		}

		for _, a := range rel.Assets {
			if isMetaAsset(a.Name) {
				continue
			}
			r := classify.Filename(a.Name)

			assets = append(assets, storage.Asset{
				Filename: a.Name,
				Version:  rel.TagName,
				Channel:  channel,
				OS:       string(r.OS),
				Arch:     string(r.Arch),
				Libc:     string(r.Libc),
				Format:   string(r.Format),
				Download: a.BrowserDownloadURL,
				Date:     date,
			})
		}
	}
	return assets, nil
}

// --- Helpers ---

func isMetaAsset(name string) bool {
	lower := strings.ToLower(name)
	for _, suffix := range []string{
		".sha256", ".sha256sum", ".sha512", ".sha512sum",
		".md5", ".md5sum", ".sig", ".asc", ".pem",
		"checksums.txt", "sha256sums", "sha512sums",
		".sbom", ".spdx", ".json.sig", ".sigstore",
		"_src.tar.gz", "_src.tar.xz", "_src.zip",
		".d.ts", ".pub",
	} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	for _, contains := range []string{
		"checksums", "sha256sum", "sha512sum",
		"buildable-artifact",
	} {
		if strings.Contains(lower, contains) {
			return true
		}
	}
	for _, exact := range []string{
		"install.sh", "install.ps1", "compat.json",
	} {
		if lower == exact {
			return true
		}
	}
	return false
}

