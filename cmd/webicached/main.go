// Command webicached is the release cache daemon. It fetches releases
// from upstream sources, classifies build assets, and writes them to
// the _cache/ directory in the format the Node.js server expects.
//
// This is the Go replacement for the Node.js release-fetching pipeline.
// It reads releases.conf files to discover packages, fetches from the
// configured source, classifies assets, and writes to fsstore.
//
// Default mode: classify all from existing rawcache on startup, then
// fetch+refresh one package per tick (round-robin, 15m default).
//
// Usage:
//
//	go run ./cmd/webicached                # default: round-robin, one per tick
//	go run ./cmd/webicached -eager         # fetch all packages on startup
//	go run ./cmd/webicached -once -no-fetch # classify from rawcache and exit
//	go run ./cmd/webicached bat goreleaser # only these packages
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

	"github.com/webinstall/webi-installers/internal/classifypkg"
	"github.com/webinstall/webi-installers/internal/installerconf"
	"github.com/webinstall/webi-installers/internal/rawcache"
	"github.com/webinstall/webi-installers/internal/releases/chromedist"
	"github.com/webinstall/webi-installers/internal/releases/flutterdist"
	"github.com/webinstall/webi-installers/internal/releases/gitea"
	"github.com/webinstall/webi-installers/internal/releases/github"
	"github.com/webinstall/webi-installers/internal/releases/githubish"
	"github.com/webinstall/webi-installers/internal/releases/gittag"
	"github.com/webinstall/webi-installers/internal/releases/golang"
	"github.com/webinstall/webi-installers/internal/releases/gpgdist"
	"github.com/webinstall/webi-installers/internal/releases/hashicorp"
	"github.com/webinstall/webi-installers/internal/releases/iterm2dist"
	"github.com/webinstall/webi-installers/internal/releases/juliadist"
	"github.com/webinstall/webi-installers/internal/releases/mariadbdist"
	"github.com/webinstall/webi-installers/internal/releases/nodedist"
	"github.com/webinstall/webi-installers/internal/releases/zigdist"
	"github.com/webinstall/webi-installers/internal/storage/fsstore"
)

// WebiCache holds the configuration for the cache daemon.
type WebiCache struct {
	ConfDir   string          // root directory with {pkg}/releases.conf files
	Store     *fsstore.Store  // classified asset storage
	RawDir    string          // raw upstream response cache
	Client    *http.Client    // HTTP client for upstream calls
	Auth      *githubish.Auth // GitHub API auth (optional)
	Shallow   bool            // fetch only the first page of releases
	NoFetch   bool            // skip fetching, classify from existing raw data only
	PageDelay time.Duration   // delay between paginated API requests
}

// delayTransport wraps an http.RoundTripper to add a delay between requests.
type delayTransport struct {
	base  http.RoundTripper
	delay time.Duration
	last  time.Time
}

func (t *delayTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !t.last.IsZero() && t.delay > 0 {
		if wait := t.delay - time.Since(t.last); wait > 0 {
			time.Sleep(wait)
		}
	}
	t.last = time.Now()
	return t.base.RoundTrip(req)
}

func main() {
	confDir := flag.String("conf", ".", "root directory containing {pkg}/releases.conf files")
	cacheDir := flag.String("cache", "_cache", "output cache directory (fsstore root)")
	rawDir := flag.String("raw", "_cache/raw", "raw cache directory for upstream responses")
	token := flag.String("token", os.Getenv("GITHUB_TOKEN"), "GitHub API token")
	once := flag.Bool("once", false, "run once then exit (no periodic refresh)")
	noFetch := flag.Bool("no-fetch", false, "skip fetching, classify from existing raw data only")
	shallow := flag.Bool("shallow", false, "fetch only the first page of releases (latest)")
	eager := flag.Bool("eager", false, "fetch all packages on startup (default: one per tick)")
	interval := flag.Duration("interval", 15*time.Minute, "refresh interval")
	pageDelay := flag.Duration("page-delay", 2*time.Second, "delay between paginated API requests")
	flag.Parse()

	store, err := fsstore.New(*cacheDir)
	if err != nil {
		log.Fatalf("fsstore: %v", err)
	}

	var auth *githubish.Auth
	if *token != "" {
		auth = &githubish.Auth{Token: *token}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	if *pageDelay > 0 {
		client.Transport = &delayTransport{
			base:  http.DefaultTransport,
			delay: *pageDelay,
		}
	}

	wc := &WebiCache{
		ConfDir:   *confDir,
		Store:     store,
		RawDir:    *rawDir,
		Client:    client,
		Auth:      auth,
		Shallow:   *shallow,
		NoFetch:   *noFetch,
		PageDelay: *pageDelay,
	}

	filterPkgs := flag.Args()

	if *eager {
		// Eager mode: fetch+classify all packages upfront.
		wc.Run(filterPkgs)
		if *once {
			return
		}
	} else if *once {
		// Once mode without eager: classify all from existing rawcache.
		wc.Run(filterPkgs)
		return
	} else {
		// Default: classify all from existing rawcache first, then
		// fetch+refresh one package per tick.
		saved := wc.NoFetch
		wc.NoFetch = true
		wc.Run(filterPkgs)
		wc.NoFetch = saved
	}

	// Discover the full package list for round-robin.
	packages, err := discover(wc.ConfDir)
	if err != nil {
		log.Fatalf("discover: %v", err)
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

	// Filter out aliases — only real packages get round-robin refreshed.
	var real []pkgConf
	for _, pkg := range packages {
		if pkg.conf.AliasOf == "" {
			real = append(real, pkg)
		}
	}

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	log.Printf("running every %s, %d packages round-robin (ctrl-c to stop)", *interval, len(real))

	idx := 0
	for range ticker.C {
		if len(real) == 0 {
			continue
		}
		pkg := real[idx%len(real)]
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		if err := wc.refreshPackage(ctx, pkg); err != nil {
			log.Printf("  ERROR %s: %v", pkg.name, err)
		}
		cancel()
		idx++
	}
}

// Run discovers packages and refreshes each one.
func (wc *WebiCache) Run(filterPkgs []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	packages, err := discover(wc.ConfDir)
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

	// Skip aliases and symlinked dirs — the legacy cache doesn't
	// generate entries for them. They're resolved at request time.
	var real []pkgConf
	for _, pkg := range packages {
		if pkg.conf.AliasOf != "" {
			continue
		}
		real = append(real, pkg)
	}

	log.Printf("refreshing %d packages", len(real))
	runStart := time.Now()

	for _, pkg := range real {
		if err := wc.refreshPackage(ctx, pkg); err != nil {
			log.Printf("  ERROR %s: %v", pkg.name, err)
		}
	}

	log.Printf("refreshed %d packages in %s", len(real), time.Since(runStart))
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
		pkgDir := filepath.Dir(path)
		name := filepath.Base(pkgDir)
		if strings.HasPrefix(name, "_") {
			continue
		}

		// If the package directory is a symlink, treat it as an alias
		// of the symlink target (e.g. rust.vim → vim-rust).
		fi, err := os.Lstat(filepath.Join(dir, name))
		if err != nil {
			log.Printf("warning: %s: %v", name, err)
			continue
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(filepath.Join(dir, name))
			if err != nil {
				log.Printf("warning: readlink %s: %v", name, err)
				continue
			}
			packages = append(packages, pkgConf{
				name: name,
				conf: &installerconf.Conf{AliasOf: target},
			})
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
func (wc *WebiCache) refreshPackage(ctx context.Context, pkg pkgConf) error {
	pkgStart := time.Now()
	name := pkg.name
	conf := pkg.conf

	// Step 1: Fetch raw upstream data to rawcache (unless -no-fetch).
	if !wc.NoFetch {
		fetchStart := time.Now()
		if err := wc.fetchRaw(ctx, pkg); err != nil {
			return fmt.Errorf("fetch: %w", err)
		}
		log.Printf("  %s: fetch %s", name, time.Since(fetchStart))
	}

	// Step 2: Classify raw data into assets, tag variants, apply config.
	classifyStart := time.Now()
	d, err := rawcache.Open(filepath.Join(wc.RawDir, name))
	if err != nil {
		return fmt.Errorf("rawcache open: %w", err)
	}

	assets, err := classifypkg.Package(name, conf, d)
	if err != nil {
		return fmt.Errorf("classify: %w", err)
	}
	classifyDur := time.Since(classifyStart)

	// Step 3: Write to fsstore.
	writeStart := time.Now()
	tx, err := wc.Store.BeginRefresh(ctx, name)
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
	writeDur := time.Since(writeStart)

	log.Printf("  %s: %d assets (classify %s, write %s, total %s)",
		name, len(assets), classifyDur, writeDur, time.Since(pkgStart))
	return nil
}

// --- Fetch raw ---

func (wc *WebiCache) fetchRaw(ctx context.Context, pkg pkgConf) error {
	switch pkg.conf.Source {
	case "github":
		return wc.fetchGitHub(ctx, pkg.name, pkg.conf)
	case "nodedist":
		return wc.fetchNodeDist(ctx, pkg.name, pkg.conf)
	case "gittag":
		return wc.fetchGitTag(ctx, pkg.name, pkg.conf)
	case "gitea":
		return wc.fetchGitea(ctx, pkg.name, pkg.conf)
	case "chromedist":
		return fetchChromeDist(ctx, wc.Client, wc.RawDir, pkg.name)
	case "flutterdist":
		return fetchFlutterDist(ctx, wc.Client, wc.RawDir, pkg.name)
	case "golang":
		return fetchGolang(ctx, wc.Client, wc.RawDir, pkg.name)
	case "gpgdist":
		return fetchGPGDist(ctx, wc.Client, wc.RawDir, pkg.name)
	case "hashicorp":
		return fetchHashiCorp(ctx, wc.Client, wc.RawDir, pkg.name, pkg.conf)
	case "iterm2dist":
		return fetchITerm2Dist(ctx, wc.Client, wc.RawDir, pkg.name)
	case "juliadist":
		return fetchJuliaDist(ctx, wc.Client, wc.RawDir, pkg.name)
	case "mariadbdist":
		return fetchMariaDBDist(ctx, wc.Client, wc.RawDir, pkg.name)
	case "zigdist":
		return fetchZigDist(ctx, wc.Client, wc.RawDir, pkg.name)
	default:
		log.Printf("  %s: source %q not yet supported, skipping", pkg.name, pkg.conf.Source)
		return nil
	}
}

func (wc *WebiCache) fetchGitHub(ctx context.Context, pkgName string, conf *installerconf.Conf) error {
	owner, repo := conf.Owner, conf.Repo
	if owner == "" || repo == "" {
		return fmt.Errorf("missing owner or repo")
	}

	d, err := rawcache.Open(filepath.Join(wc.RawDir, pkgName))
	if err != nil {
		return err
	}

	tagPrefix := conf.TagPrefix
	for batch, err := range github.Fetch(ctx, wc.Client, owner, repo, wc.Auth) {
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
		if wc.Shallow {
			break
		}
	}
	return nil
}

func (wc *WebiCache) fetchNodeDist(ctx context.Context, pkgName string, conf *installerconf.Conf) error {
	baseURL := conf.BaseURL
	if baseURL == "" {
		return fmt.Errorf("missing url")
	}

	d, err := rawcache.Open(filepath.Join(wc.RawDir, pkgName))
	if err != nil {
		return err
	}

	// Fetch from primary URL. Tag with "official/" prefix so unofficial
	// entries for the same version don't overwrite.
	for batch, err := range nodedist.Fetch(ctx, wc.Client, baseURL) {
		if err != nil {
			return err
		}
		for _, entry := range batch {
			data, _ := json.Marshal(entry)
			d.Merge("official/"+entry.Version, data)
		}
	}

	// Fetch from unofficial URL if configured (e.g. Node.js unofficial builds
	// which add musl, riscv64, loong64 targets).
	if unofficialURL := conf.Extra["unofficial_url"]; unofficialURL != "" {
		for batch, err := range nodedist.Fetch(ctx, wc.Client, unofficialURL) {
			if err != nil {
				log.Printf("warning: %s unofficial fetch: %v", pkgName, err)
				break
			}
			for _, entry := range batch {
				data, _ := json.Marshal(entry)
				d.Merge("unofficial/"+entry.Version, data)
			}
		}
	}

	return nil
}

func (wc *WebiCache) fetchGitTag(ctx context.Context, pkgName string, conf *installerconf.Conf) error {
	gitURL := conf.BaseURL
	if gitURL == "" {
		return fmt.Errorf("missing url")
	}

	d, err := rawcache.Open(filepath.Join(wc.RawDir, pkgName))
	if err != nil {
		return err
	}

	repoDir := filepath.Join(wc.RawDir, "_repos")
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
		if wc.Shallow {
			break
		}
	}
	return nil
}

func (wc *WebiCache) fetchGitea(ctx context.Context, pkgName string, conf *installerconf.Conf) error {
	baseURL, owner, repo := conf.BaseURL, conf.Owner, conf.Repo
	if baseURL == "" || owner == "" || repo == "" {
		return fmt.Errorf("missing base_url, owner, or repo")
	}

	d, err := rawcache.Open(filepath.Join(wc.RawDir, pkgName))
	if err != nil {
		return err
	}

	for batch, err := range gitea.Fetch(ctx, wc.Client, baseURL, owner, repo, nil) {
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
		if wc.Shallow {
			break
		}
	}
	return nil
}

func fetchChromeDist(ctx context.Context, client *http.Client, rawDir, pkgName string) error {
	d, err := rawcache.Open(filepath.Join(rawDir, pkgName))
	if err != nil {
		return err
	}

	for batch, err := range chromedist.Fetch(ctx, client) {
		if err != nil {
			return fmt.Errorf("chromedist: %w", err)
		}
		for _, ver := range batch {
			data, _ := json.Marshal(ver)
			d.Merge(ver.Version, data)
		}
	}
	return nil
}

func fetchFlutterDist(ctx context.Context, client *http.Client, rawDir, pkgName string) error {
	d, err := rawcache.Open(filepath.Join(rawDir, pkgName))
	if err != nil {
		return err
	}

	for batch, err := range flutterdist.Fetch(ctx, client) {
		if err != nil {
			return fmt.Errorf("flutterdist: %w", err)
		}
		for _, rel := range batch {
			// Key by version+channel+os for uniqueness.
			key := rel.Version + "-" + rel.Channel + "-" + rel.OS
			data, _ := json.Marshal(rel)
			d.Merge(key, data)
		}
	}
	return nil
}

func fetchGolang(ctx context.Context, client *http.Client, rawDir, pkgName string) error {
	d, err := rawcache.Open(filepath.Join(rawDir, pkgName))
	if err != nil {
		return err
	}

	for batch, err := range golang.Fetch(ctx, client) {
		if err != nil {
			return fmt.Errorf("golang: %w", err)
		}
		for _, rel := range batch {
			data, _ := json.Marshal(rel)
			d.Merge(rel.Version, data)
		}
	}
	return nil
}

func fetchGPGDist(ctx context.Context, client *http.Client, rawDir, pkgName string) error {
	d, err := rawcache.Open(filepath.Join(rawDir, pkgName))
	if err != nil {
		return err
	}

	for batch, err := range gpgdist.Fetch(ctx, client) {
		if err != nil {
			return fmt.Errorf("gpgdist: %w", err)
		}
		for _, entry := range batch {
			data, _ := json.Marshal(entry)
			d.Merge(entry.Version, data)
		}
	}
	return nil
}

func fetchHashiCorp(ctx context.Context, client *http.Client, rawDir, pkgName string, conf *installerconf.Conf) error {
	product := conf.Extra["product"]
	if product == "" {
		product = pkgName
	}

	d, err := rawcache.Open(filepath.Join(rawDir, pkgName))
	if err != nil {
		return err
	}

	for idx, err := range hashicorp.Fetch(ctx, client, product) {
		if err != nil {
			return fmt.Errorf("hashicorp %s: %w", product, err)
		}
		for ver, vdata := range idx.Versions {
			data, _ := json.Marshal(vdata)
			d.Merge(ver, data)
		}
	}
	return nil
}

func fetchITerm2Dist(ctx context.Context, client *http.Client, rawDir, pkgName string) error {
	d, err := rawcache.Open(filepath.Join(rawDir, pkgName))
	if err != nil {
		return err
	}

	for batch, err := range iterm2dist.Fetch(ctx, client) {
		if err != nil {
			return fmt.Errorf("iterm2dist: %w", err)
		}
		for _, entry := range batch {
			key := entry.Version
			if entry.Channel == "beta" {
				key += "-beta"
			}
			data, _ := json.Marshal(entry)
			d.Merge(key, data)
		}
	}
	return nil
}

func fetchJuliaDist(ctx context.Context, client *http.Client, rawDir, pkgName string) error {
	d, err := rawcache.Open(filepath.Join(rawDir, pkgName))
	if err != nil {
		return err
	}

	for batch, err := range juliadist.Fetch(ctx, client) {
		if err != nil {
			return fmt.Errorf("juliadist: %w", err)
		}
		for _, rel := range batch {
			data, _ := json.Marshal(rel)
			d.Merge(rel.Version, data)
		}
	}
	return nil
}

func fetchMariaDBDist(ctx context.Context, client *http.Client, rawDir, pkgName string) error {
	d, err := rawcache.Open(filepath.Join(rawDir, pkgName))
	if err != nil {
		return err
	}

	for batch, err := range mariadbdist.Fetch(ctx, client) {
		if err != nil {
			return fmt.Errorf("mariadbdist: %w", err)
		}
		for _, rel := range batch {
			data, _ := json.Marshal(rel)
			d.Merge(rel.ReleaseID, data)
		}
	}
	return nil
}

func fetchZigDist(ctx context.Context, client *http.Client, rawDir, pkgName string) error {
	d, err := rawcache.Open(filepath.Join(rawDir, pkgName))
	if err != nil {
		return err
	}

	for batch, err := range zigdist.Fetch(ctx, client) {
		if err != nil {
			return fmt.Errorf("zigdist: %w", err)
		}
		for _, rel := range batch {
			data, _ := json.Marshal(rel)
			d.Merge(rel.Version, data)
		}
	}
	return nil
}
