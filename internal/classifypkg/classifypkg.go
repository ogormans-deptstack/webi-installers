// Package classifypkg converts raw upstream release data into classified
// [storage.Asset] slices. Each source type (github, nodedist, gittag, etc.)
// has its own classifier that reads JSON from [rawcache.Dir] and produces
// assets with OS, arch, format, and channel fields populated.
//
// This is the second stage of the pipeline: fetch → classify → tag → filter → store.
package classifypkg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/webinstall/webi-installers/internal/classify"
	"github.com/webinstall/webi-installers/internal/installerconf"
	"github.com/webinstall/webi-installers/internal/rawcache"
	"github.com/webinstall/webi-installers/internal/releases/bun"
	"github.com/webinstall/webi-installers/internal/releases/chromedist"
	"github.com/webinstall/webi-installers/internal/releases/fish"
	"github.com/webinstall/webi-installers/internal/releases/gitea"
	"github.com/webinstall/webi-installers/internal/releases/flutterdist"
	"github.com/webinstall/webi-installers/internal/releases/git"
	"github.com/webinstall/webi-installers/internal/releases/golang"
	"github.com/webinstall/webi-installers/internal/releases/gpgdist"
	"github.com/webinstall/webi-installers/internal/releases/hashicorp"
	"github.com/webinstall/webi-installers/internal/releases/iterm2dist"
	"github.com/webinstall/webi-installers/internal/releases/juliadist"
	"github.com/webinstall/webi-installers/internal/releases/lsd"
	"github.com/webinstall/webi-installers/internal/releases/mariadbdist"
	"github.com/webinstall/webi-installers/internal/releases/node"
	"github.com/webinstall/webi-installers/internal/releases/ollama"
	"github.com/webinstall/webi-installers/internal/releases/pwsh"
	"github.com/webinstall/webi-installers/internal/releases/postgres"
	"github.com/webinstall/webi-installers/internal/releases/watchexec"
	"github.com/webinstall/webi-installers/internal/releases/xcaddy"
	"github.com/webinstall/webi-installers/internal/releases/zigdist"
	"github.com/webinstall/webi-installers/internal/storage"
)

// channelFromVersion infers a release channel from the version string.
// Many GitHub releases have pre-release versions (rc, beta, alpha, dev,
// preview) but don't set the prerelease boolean in the API.
func channelFromVersion(version string) string {
	v := strings.ToLower(version)
	switch {
	case strings.Contains(v, "-rc") || strings.Contains(v, ".rc"):
		return "rc"
	case strings.Contains(v, "-beta") || strings.Contains(v, ".beta"):
		return "beta"
	case strings.Contains(v, "-alpha") || strings.Contains(v, ".alpha"):
		return "alpha"
	case strings.Contains(v, "-dev") || strings.Contains(v, ".dev"):
		return "dev"
	case strings.Contains(v, "-preview") || strings.Contains(v, ".preview"):
		return "preview"
	case strings.Contains(v, "-pre") || strings.Contains(v, ".pre"):
		return "beta"
	case strings.Contains(v, "-nightly"):
		return "nightly"
	case strings.Contains(v, "-canary"):
		return "canary"
	}
	return "stable"
}

// Package classifies raw upstream data into assets, tags variants,
// and applies config-driven filters. This is the full classify pipeline
// for a single package.
func Package(pkg string, conf *installerconf.Conf, d *rawcache.Dir) ([]storage.Asset, error) {
	assets, err := classifySource(pkg, conf, d)
	if err != nil {
		return nil, err
	}

	TagVariants(pkg, assets)
	NormalizeVersions(pkg, assets)
	assets = ApplyConfig(assets, conf)
	assets = appendLegacy(pkg, assets)
	return assets, nil
}

// classifySource dispatches to the source-specific classifier.
func classifySource(pkg string, conf *installerconf.Conf, d *rawcache.Dir) ([]storage.Asset, error) {
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

// NormalizeVersions applies package-specific version normalization.
// For example, Git for Windows strips ".windows.N" from version strings.
func NormalizeVersions(pkg string, assets []storage.Asset) {
	switch pkg {
	case "git":
		git.NormalizeVersions(assets)
	case "lf":
		// lf tags are "r1", "r2", etc. Node.js converts to "0.N.0".
		for i := range assets {
			v := assets[i].Version
			if strings.HasPrefix(v, "r") {
				assets[i].Version = "0." + v[1:] + ".0"
			}
		}
	case "postgres", "psql":
		postgres.NormalizeVersions(assets)
	case "watchexec":
		watchexec.NormalizeVersions(assets)
	}
}

// TagVariants applies package-specific variant tags to classified assets.
// Each case delegates to a per-installer package under internal/releases/.
func TagVariants(pkg string, assets []storage.Asset) {
	switch pkg {
	case "bun":
		bun.TagVariants(assets)
	case "fish":
		fish.TagVariants(assets)
	case "git":
		git.TagVariants(assets)
	case "gitea":
		gitea.TagVariants(assets)
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

// appendLegacy adds hardcoded legacy releases for packages that had
// releases from sources that no longer exist (e.g. EnterpriseDB binaries).
func appendLegacy(pkg string, assets []storage.Asset) []storage.Asset {
	switch pkg {
	case "postgres":
		assets = append(assets, postgres.LegacyReleases()...)
	}
	return assets
}

// ApplyConfig applies asset_filter, exclude, and version prefix stripping
// from a package's releases.conf.
func ApplyConfig(assets []storage.Asset, conf *installerconf.Conf) []storage.Asset {
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

// ReadAllRaw reads all non-directory, non-underscore-prefixed files from
// the active generation of a rawcache directory.
func ReadAllRaw(d *rawcache.Dir) (map[string][]byte, error) {
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
	releases, err := ReadAllRaw(d)
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
			if !strings.HasPrefix(version, tagPrefix) {
				continue // skip tags from other packages in monorepos
			}
			version = strings.TrimPrefix(version, tagPrefix)
		}

		channel := "stable"
		if rel.Prerelease {
			channel = "beta"
		} else {
			channel = channelFromVersion(version)
		}

		date := ""
		if len(rel.PublishedAt) >= 10 {
			date = rel.PublishedAt[:10]
		}

		for _, a := range rel.Assets {
			if classify.IsMetaAsset(a.Name) {
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

	releases, err := ReadAllRaw(d)
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
	releases, err := ReadAllRaw(d)
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
			// version prefixed with HEAD so it doesn't sort ahead of
			// real semver tags (e.g. HEAD-2023.10.10 vs v1.2).
			version = "HEAD-" + strings.ReplaceAll(entry.Date[:10], "-", ".")
			filename = repoName + "-" + version
		} else {
			continue
		}

		assets = append(assets, storage.Asset{
			Filename: filename,
			Version:  version,
			Channel:  channelFromVersion(version),
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
	releases, err := ReadAllRaw(d)
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
		} else {
			channel = channelFromVersion(rel.TagName)
		}
		date := ""
		if len(rel.PublishedAt) >= 10 {
			date = rel.PublishedAt[:10]
		}

		for _, a := range rel.Assets {
			if classify.IsMetaAsset(a.Name) {
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

func classifyChromeDist(d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := ReadAllRaw(d)
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

func classifyFlutterDist(d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := ReadAllRaw(d)
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

func classifyGolang(d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := ReadAllRaw(d)
	if err != nil {
		return nil, err
	}

	var assets []storage.Asset
	for _, data := range releases {
		var rel golang.Release
		if err := json.Unmarshal(data, &rel); err != nil {
			continue
		}

		// Strip "go" prefix and pad to 3-part version: "go1.10" → "1.10.0"
		version := strings.TrimPrefix(rel.Version, "go")
		parts := strings.SplitN(version, ".", 3)
		for len(parts) < 3 {
			parts = append(parts, "0")
		}
		version = strings.Join(parts, ".")

		channel := "stable"
		if !rel.Stable {
			channel = "beta"
		}

		for _, f := range rel.Files {
			if f.Kind == "source" {
				continue
			}
			// Skip bootstrap and odd builds.
			if strings.Contains(f.Filename, "bootstrap") || strings.Contains(f.Filename, "-arm6.") {
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

func classifyGPGDist(d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := ReadAllRaw(d)
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

func classifyHashiCorp(d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := ReadAllRaw(d)
	if err != nil {
		return nil, err
	}

	var assets []storage.Asset
	for _, data := range releases {
		var ver hashicorp.Version
		if err := json.Unmarshal(data, &ver); err != nil {
			continue
		}

		channel := channelFromVersion(ver.Version)

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

func classifyITerm2Dist(d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := ReadAllRaw(d)
	if err != nil {
		return nil, err
	}

	var assets []storage.Asset
	for _, data := range releases {
		var entry iterm2dist.Entry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}

		if entry.Version == "" {
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

func classifyJuliaDist(d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := ReadAllRaw(d)
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

func classifyMariaDBDist(d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := ReadAllRaw(d)
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

func classifyZigDist(d *rawcache.Dir) ([]storage.Asset, error) {
	releases, err := ReadAllRaw(d)
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
