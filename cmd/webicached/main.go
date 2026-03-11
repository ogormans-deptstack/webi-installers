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
	"github.com/webinstall/webi-installers/internal/releases/bun"
	"github.com/webinstall/webi-installers/internal/releases/chromedist"
	"github.com/webinstall/webi-installers/internal/releases/fish"
	"github.com/webinstall/webi-installers/internal/releases/flutterdist"
	"github.com/webinstall/webi-installers/internal/releases/git"
	"github.com/webinstall/webi-installers/internal/releases/gitea"
	"github.com/webinstall/webi-installers/internal/releases/github"
	"github.com/webinstall/webi-installers/internal/releases/githubish"
	"github.com/webinstall/webi-installers/internal/releases/gittag"
	"github.com/webinstall/webi-installers/internal/releases/golang"
	"github.com/webinstall/webi-installers/internal/releases/gpgdist"
	"github.com/webinstall/webi-installers/internal/releases/hashicorp"
	"github.com/webinstall/webi-installers/internal/releases/iterm2dist"
	"github.com/webinstall/webi-installers/internal/releases/juliadist"
	"github.com/webinstall/webi-installers/internal/releases/lsd"
	"github.com/webinstall/webi-installers/internal/releases/mariadbdist"
	"github.com/webinstall/webi-installers/internal/releases/node"
	"github.com/webinstall/webi-installers/internal/releases/nodedist"
	"github.com/webinstall/webi-installers/internal/releases/ollama"
	"github.com/webinstall/webi-installers/internal/releases/pwsh"
	"github.com/webinstall/webi-installers/internal/releases/xcaddy"
	"github.com/webinstall/webi-installers/internal/releases/zigdist"
	"github.com/webinstall/webi-installers/internal/storage"
	"github.com/webinstall/webi-installers/internal/storage/fsstore"
)

// WebiCache holds the configuration for the cache daemon.
type WebiCache struct {
	ConfDir string          // root directory with {pkg}/releases.conf files
	Store   *fsstore.Store  // classified asset storage
	RawDir  string          // raw upstream response cache
	Client  *http.Client    // HTTP client for upstream calls
	Auth    *githubish.Auth // GitHub API auth (optional)
	Shallow bool            // fetch only the first page of releases
	NoFetch bool            // skip fetching, classify from existing raw data only
}

func main() {
	confDir := flag.String("conf", ".", "root directory containing {pkg}/releases.conf files")
	cacheDir := flag.String("cache", "_cache", "output cache directory (fsstore root)")
	rawDir := flag.String("raw", "_cache/raw", "raw cache directory for upstream responses")
	token := flag.String("token", os.Getenv("GITHUB_TOKEN"), "GitHub API token")
	once := flag.Bool("once", false, "run once then exit (no periodic refresh)")
	noFetch := flag.Bool("no-fetch", false, "skip fetching, classify from existing raw data only")
	shallow := flag.Bool("shallow", false, "fetch only the first page of releases (latest)")
	interval := flag.Duration("interval", 15*time.Minute, "refresh interval")
	flag.Parse()

	store, err := fsstore.New(*cacheDir)
	if err != nil {
		log.Fatalf("fsstore: %v", err)
	}

	var auth *githubish.Auth
	if *token != "" {
		auth = &githubish.Auth{Token: *token}
	}

	wc := &WebiCache{
		ConfDir: *confDir,
		Store:   store,
		RawDir:  *rawDir,
		Client:  &http.Client{Timeout: 30 * time.Second},
		Auth:    auth,
		Shallow: *shallow,
		NoFetch: *noFetch,
	}

	filterPkgs := flag.Args()

	wc.Run(filterPkgs)
	if *once {
		return
	}

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	log.Printf("running every %s (ctrl-c to stop)", *interval)
	for range ticker.C {
		wc.Run(filterPkgs)
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

	log.Printf("refreshing %d packages", len(packages))
	runStart := time.Now()

	for _, pkg := range packages {
		if alias := pkg.conf.Extra["alias_of"]; alias != "" {
			continue
		}

		if err := wc.refreshPackage(ctx, pkg); err != nil {
			log.Printf("  ERROR %s: %v", pkg.name, err)
		}
	}

	log.Printf("refreshed %d packages in %s", len(packages), time.Since(runStart))
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

	// Step 2: Classify raw data into assets.
	classifyStart := time.Now()
	d, err := rawcache.Open(filepath.Join(wc.RawDir, name))
	if err != nil {
		return fmt.Errorf("rawcache open: %w", err)
	}

	assets, err := classifyPackage(name, conf, d)
	if err != nil {
		return fmt.Errorf("classify: %w", err)
	}

	// Step 2.5: Tag build variants.
	tagVariants(name, conf, assets)

	// Step 3: Apply config transforms.
	assets = applyConfig(assets, conf)
	classifyDur := time.Since(classifyStart)

	// Step 4: Write to fsstore.
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

// applyConfig applies asset_filter, exclude, and version prefix stripping.
func applyConfig(assets []storage.Asset, conf *installerconf.Conf) []storage.Asset {
	filter := strings.ToLower(conf.AssetFilter)
	excludes := conf.Exclude
	prefixes := conf.VersionPrefixes

	var out []storage.Asset
	for _, a := range assets {
		lower := strings.ToLower(a.Filename)

		// Include filter: asset must contain this substring.
		if filter != "" && !strings.Contains(lower, filter) {
			continue
		}

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

// --- Classify per source ---

func classifyPackage(pkg string, conf *installerconf.Conf, d *rawcache.Dir) ([]storage.Asset, error) {
	switch conf.Source {
	case "github":
		return classifyGitHub(pkg, conf, d)
	case "nodedist":
		return classifyNodeDist(pkg, conf, d)
	case "gittag":
		return classifyGitTag(pkg, conf, d)
	case "gitea":
		return classifyGitea(pkg, conf, d)
	case "chromedist":
		return classifyChromeDist(d)
	case "flutterdist":
		return classifyFlutterDist(d)
	case "golang":
		return classifyGolang(d)
	case "gpgdist":
		return classifyGPGDist(d)
	case "hashicorp":
		return classifyHashiCorp(d)
	case "iterm2dist":
		return classifyITerm2Dist(d)
	case "juliadist":
		return classifyJuliaDist(d)
	case "mariadbdist":
		return classifyMariaDBDist(d)
	case "zigdist":
		return classifyZigDist(d)
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

			// Normalize .tgz → .tar.gz in the display filename.
			// The download URL still points to the real file.
			name := a.Name
			if strings.HasSuffix(strings.ToLower(name), ".tgz") {
				name = name[:len(name)-4] + ".tar.gz"
			}

			assets = append(assets, storage.Asset{
				Filename: name,
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

		// Source archives for packages with no binary assets.
		// These are installable on any POSIX system (shell scripts, etc.).
		//
		// GitHub has two archive URL formats:
		// Releases with no uploaded assets are source-only — use the
		// API-provided tarball/zipball URLs. These can also be installed
		// via `git clone --branch <tag>` as a fallback.
		//
		// TODO: HEAD-follow the tarball_url at fetch time to get the
		// resolved codeload URL and Content-Disposition filename
		// (Owner-Repo-Tag-0-gCommitHash.ext). For now, use the tag
		// as the filename since the actual download name comes from
		// Content-Disposition anyway.
		if len(rel.Assets) == 0 {
			owner := conf.Owner
			repo := conf.Repo
			tag := rel.TagName
			if rel.TarballURL != "" {
				assets = append(assets, storage.Asset{
					Filename: repo + "-" + tag + ".tar.gz",
					Version:  version,
					Channel:  channel,
					OS:       "posix_2017",
					Arch:     "*",
					Format:   ".tar.gz",
					Download: rel.TarballURL,
					Date:     date,
				})
			}
			if rel.ZipballURL != "" {
				assets = append(assets, storage.Asset{
					Filename: repo + "-" + tag + ".zip",
					Version:  version,
					Channel:  channel,
					OS:       "posix_2017",
					Arch:     "*",
					Format:   ".zip",
					Download: rel.ZipballURL,
					Date:     date,
				})
			}
			// Git clone asset — same as gittag source.
			gitURL := fmt.Sprintf("https://github.com/%s/%s.git", owner, repo)
			assets = append(assets, storage.Asset{
				Filename: repo + "-" + tag,
				Version:  version,
				Channel:  channel,
				Format:   "git",
				Download: gitURL,
				Date:     date,
			})
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
	officialURL := conf.BaseURL
	unofficialURL := conf.Extra["unofficial_url"]

	releases, err := readAllRaw(d)
	if err != nil {
		return nil, err
	}

	var assets []storage.Asset
	for tag, data := range releases {
		var entry nodeEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}

		// Pick the right base URL from the tag prefix.
		baseURL := officialURL
		if strings.HasPrefix(tag, "unofficial_") {
			baseURL = unofficialURL
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
		"riscv64": "riscv64", "loong64": "loong64",
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
	case "7z":
		formats = []string{".7z"}
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

func classifyGitTag(pkg string, conf *installerconf.Conf, d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := readAllRaw(d)
	if err != nil {
		return nil, err
	}

	// Derive repo name from the git URL for filenames.
	// "https://github.com/tpope/vim-commentary.git" → "vim-commentary"
	gitURL := conf.BaseURL
	repoName := pkg
	if gitURL != "" {
		base := filepath.Base(gitURL)
		repoName = strings.TrimSuffix(base, ".git")
	}

	var assets []storage.Asset
	for _, data := range releases {
		var entry gitTagEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}

		version := strings.TrimPrefix(entry.Version, "v")
		date := ""
		if len(entry.Date) >= 10 {
			date = entry.Date[:10]
		}

		var filename string
		if version != "" {
			// Tagged release: "{repo}-{tag}" (e.g. "vim-commentary-v1.2")
			filename = repoName + "-" + entry.GitTag
		} else if len(entry.Date) >= 19 {
			// Tagless repo (HEAD of master/main): synthesize a date-based
			// version like Node.js does: "2023.10.10-18.42.21"
			version = strings.ReplaceAll(entry.Date[:10], "-", ".") +
				"-" + strings.ReplaceAll(entry.Date[11:19], ":", ".")
			filename = repoName + "-v" + version
		} else {
			continue
		}

		assets = append(assets, storage.Asset{
			Filename: filename,
			Version:  version,
			Channel:  "stable",
			Format:   "git",
			Download: gitURL,
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

// --- Chrome for Testing ---

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

func classifyChromeDist(d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := readAllRaw(d)
	if err != nil {
		return nil, err
	}

	var assets []storage.Asset
	for _, data := range releases {
		var ver chromedist.Version
		if err := json.Unmarshal(data, &ver); err != nil {
			continue
		}

		downloads := ver.Downloads["chromedriver"]
		if len(downloads) == 0 {
			continue
		}

		for _, dl := range downloads {
			r := classify.Filename(dl.URL)
			assets = append(assets, storage.Asset{
				Filename: "chromedriver-" + dl.Platform + ".zip",
				Version:  ver.Version,
				Channel:  "stable",
				OS:       string(r.OS),
				Arch:     string(r.Arch),
				Format:   ".zip",
				Download: dl.URL,
			})
		}
	}
	return assets, nil
}

// --- Flutter ---

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

func classifyFlutterDist(d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := readAllRaw(d)
	if err != nil {
		return nil, err
	}

	var assets []storage.Asset
	for _, data := range releases {
		var rel flutterdist.Release
		if err := json.Unmarshal(data, &rel); err != nil {
			continue
		}

		date := ""
		if len(rel.ReleaseDate) >= 10 {
			date = rel.ReleaseDate[:10]
		}

		filename := filepath.Base(rel.Archive)
		r := classify.Filename(filename)

		assets = append(assets, storage.Asset{
			Filename: filename,
			Version:  rel.Version,
			Channel:  rel.Channel,
			OS:       string(r.OS),
			Arch:     string(r.Arch),
			Format:   string(r.Format),
			Download: rel.DownloadURL,
			Date:     date,
		})
	}
	return assets, nil
}

// --- Go (golang.org) ---

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

func classifyGolang(d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := readAllRaw(d)
	if err != nil {
		return nil, err
	}

	var assets []storage.Asset
	for _, data := range releases {
		var rel golang.Release
		if err := json.Unmarshal(data, &rel); err != nil {
			continue
		}

		// Strip "go" prefix from version: "go1.24.1" → "1.24.1"
		version := strings.TrimPrefix(rel.Version, "go")
		channel := "stable"
		if !rel.Stable {
			channel = "beta"
		}

		for _, f := range rel.Files {
			if f.Kind == "source" {
				continue
			}
			// Skip bootstrap and odd builds.
			if strings.Contains(f.Filename, "bootstrap") {
				continue
			}

			r := classify.Filename(f.Filename)

			assets = append(assets, storage.Asset{
				Filename: f.Filename,
				Version:  version,
				Channel:  channel,
				OS:       string(r.OS),
				Arch:     string(r.Arch),
				Libc:     string(r.Libc),
				Format:   string(r.Format),
				Download: "https://dl.google.com/go/" + f.Filename,
			})
		}
	}
	return assets, nil
}

// --- GPG (SourceForge) ---

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

func classifyGPGDist(d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := readAllRaw(d)
	if err != nil {
		return nil, err
	}

	var assets []storage.Asset
	for _, data := range releases {
		var entry gpgdist.Entry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}

		assets = append(assets, storage.Asset{
			Filename: fmt.Sprintf("GnuPG-%s.dmg", entry.Version),
			Version:  entry.Version,
			Channel:  "stable",
			OS:       "darwin",
			Arch:     "amd64",
			Format:   ".dmg",
			Download: entry.URL,
		})
	}
	return assets, nil
}

// --- HashiCorp ---

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

func classifyHashiCorp(d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := readAllRaw(d)
	if err != nil {
		return nil, err
	}

	var assets []storage.Asset
	for _, data := range releases {
		var ver hashicorp.Version
		if err := json.Unmarshal(data, &ver); err != nil {
			continue
		}

		channel := "stable"
		v := ver.Version
		if strings.Contains(v, "-rc") {
			channel = "rc"
		} else if strings.Contains(v, "-beta") {
			channel = "beta"
		} else if strings.Contains(v, "-alpha") {
			channel = "alpha"
		}

		for _, b := range ver.Builds {
			r := classify.Filename(b.Filename)

			assets = append(assets, storage.Asset{
				Filename: b.Filename,
				Version:  ver.Version,
				Channel:  channel,
				OS:       string(r.OS),
				Arch:     string(r.Arch),
				Format:   string(r.Format),
				Download: b.URL,
			})
		}
	}
	return assets, nil
}

// --- iTerm2 ---

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

func classifyITerm2Dist(d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := readAllRaw(d)
	if err != nil {
		return nil, err
	}

	var assets []storage.Asset
	for _, data := range releases {
		var entry iterm2dist.Entry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}

		filename := filepath.Base(entry.URL)

		assets = append(assets, storage.Asset{
			Filename: filename,
			Version:  entry.Version,
			Channel:  entry.Channel,
			OS:       "darwin",
			Format:   ".zip",
			Download: entry.URL,
		})
	}
	return assets, nil
}

// --- Julia ---

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

func classifyJuliaDist(d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := readAllRaw(d)
	if err != nil {
		return nil, err
	}

	osMap := map[string]string{
		"mac": "darwin", "linux": "linux", "winnt": "windows",
		"freebsd": "freebsd",
	}
	archMap := map[string]string{
		"x86_64": "x86_64", "i686": "x86", "aarch64": "aarch64",
		"armv7l": "armv7", "powerpc64le": "ppc64le",
	}

	var assets []storage.Asset
	for _, data := range releases {
		var rel juliadist.Release
		if err := json.Unmarshal(data, &rel); err != nil {
			continue
		}

		channel := "stable"
		if !rel.Stable {
			channel = "beta"
		}

		for _, f := range rel.Files {
			if f.Kind == "installer" {
				continue
			}

			os_ := osMap[f.OS]
			arch := archMap[f.Arch]
			libc := ""
			if os_ == "linux" {
				if strings.Contains(f.URL, "musl") {
					libc = "musl"
				} else {
					libc = "gnu"
				}
			}

			filename := filepath.Base(f.URL)

			assets = append(assets, storage.Asset{
				Filename: filename,
				Version:  rel.Version,
				Channel:  channel,
				OS:       os_,
				Arch:     arch,
				Libc:     libc,
				Format:   "." + f.Extension,
				Download: f.URL,
			})
		}
	}
	return assets, nil
}

// --- MariaDB ---

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

func classifyMariaDBDist(d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := readAllRaw(d)
	if err != nil {
		return nil, err
	}

	channelMap := map[string]string{
		"Stable": "stable", "RC": "rc", "Alpha": "preview",
	}

	var assets []storage.Asset
	for _, data := range releases {
		var rel mariadbdist.Release
		if err := json.Unmarshal(data, &rel); err != nil {
			continue
		}

		channel := channelMap[rel.MajorStatus]
		if channel == "" {
			channel = "preview"
		}

		lts := rel.MajorStatus == "Stable"

		for _, f := range rel.Files {
			// Skip source packages. The API uses OS="Source" and
			// sometimes " " (not empty) for CPU on source tarballs.
			if strings.EqualFold(f.OS, "source") || strings.TrimSpace(f.OS) == "" || strings.TrimSpace(f.CPU) == "" {
				continue
			}
			// Skip debug builds.
			if strings.Contains(strings.ToLower(f.FileName), "debug") {
				continue
			}

			r := classify.Filename(f.FileName)

			assets = append(assets, storage.Asset{
				Filename: f.FileName,
				Version:  rel.ReleaseID,
				Channel:  channel,
				LTS:      lts,
				OS:       string(r.OS),
				Arch:     string(r.Arch),
				Format:   string(r.Format),
				Download: f.FileDownloadURL,
				Date:     rel.DateOfRelease,
			})
		}
	}
	return assets, nil
}

// --- Zig ---

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

func classifyZigDist(d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := readAllRaw(d)
	if err != nil {
		return nil, err
	}

	var assets []storage.Asset
	for _, data := range releases {
		var rel zigdist.Release
		if err := json.Unmarshal(data, &rel); err != nil {
			continue
		}

		channel := "stable"
		if !strings.Contains(rel.Version, ".") {
			// Branch names like "master" have no dots.
			channel = "beta"
		} else if strings.ContainsAny(rel.Version, "+-") {
			channel = "beta"
		}

		for platform, p := range rel.Platforms {
			// Skip source and odd entries.
			if strings.Contains(platform, "bootstrap") || platform == "src" {
				continue
			}
			if strings.Contains(platform, "armv6kz") {
				continue
			}

			// Platform is "arch-os", e.g. "x86_64-linux", "aarch64-macos".
			parts := strings.SplitN(platform, "-", 2)
			if len(parts) != 2 {
				continue
			}

			filename := filepath.Base(p.Tarball)
			r := classify.Filename(filename)

			assets = append(assets, storage.Asset{
				Filename: filename,
				Version:  rel.Version,
				Channel:  channel,
				OS:       string(r.OS),
				Arch:     string(r.Arch),
				Format:   string(r.Format),
				Download: p.Tarball,
				Date:     rel.Date,
			})
		}
	}
	return assets, nil
}

// --- Helpers ---

// isMetaAsset delegates to classify.IsMetaAsset.
var isMetaAsset = classify.IsMetaAsset

// tagVariants applies package-specific variant tags to classified assets.
// Each case delegates to a per-installer package under internal/releases/.
func tagVariants(pkg string, _ *installerconf.Conf, assets []storage.Asset) {
	switch pkg {
	case "bun":
		bun.TagVariants(assets)
	case "fish":
		fish.TagVariants(assets)
	case "git":
		git.TagVariants(assets)
	case "lsd":
		lsd.TagVariants(assets)
	case "node":
		node.TagVariants(assets)
	case "ollama":
		ollama.TagVariants(assets)
	case "pwsh":
		pwsh.TagVariants(assets)
	case "xcaddy":
		xcaddy.TagVariants(assets)
	}
}

