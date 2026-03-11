# Go Webi — Rewrite Plan

This is the planning and tracking document for rewriting the Webi server in Go.
This is **not a straight port** — we're redesigning internals while preserving the
public API surface.

## Guiding Principles

1. **Incremental migration.** Rewrites fail when they try to replace everything
   at once. We integrate piece by piece, endpoint by endpoint, into the live
   system.
2. **Library over framework.** The Go code should be composable pieces the caller
   controls — not a framework that calls your code.
3. **stdlib + pgx, nothing else.** No third-party SDKs. Dependencies: stdlib,
   `golang.org/x`, `github.com/jackc/pgx`, `github.com/therootcompany/golib`.
4. **Resilient by default.** The HTTP client, caching, and storage layers are
   built for failure — timeouts, retries, circuit breaking, graceful fallback.
5. **Simpler classification.** Standard toolchains (goreleaser, cargo-dist, etc.)
   produce predictable filenames. Match those patterns directly; push esoteric
   naming into release-fetcher tagging/filtering rather than classifier heuristics.

## Repository Layout

```
cmd/
  webid/              # main HTTP server
  webicached/         # release cache daemon (fetches + stores releases)
internal/
  buildmeta/          # OS, arch, libc, format constants and enums + CompatArches
  classify/           # build artifact classification (filename/URL → target)
  classifypkg/        # per-package pipeline: classify → tag → normalize → filter
  installerconf/      # releases.conf parser (key=value, source inferred from key)
  httpclient/         # resilient net/http client with best-practice defaults
  lexver/             # lexicographic version parsing and sorting
  platlatest/         # per-platform latest version index (triplet → version)
  rawcache/           # double-buffered raw upstream API response storage
  releases/           # release fetching — one package per source type
    github/           #   GitHub (thin wrapper over githubish)
    githubish/        #   generic GitHub-compatible API with Link header pagination
    githubsrc/        #   GitHub source archives (tarball/zipball URLs)
    gitea/            #   Gitea/Forgejo (own types, limit param, Link header)
    giteasrc/         #   Gitea source archives
    gitlab/           #   GitLab (own types, X-Total-Pages pagination)
    gitlabsrc/        #   GitLab source archives
    gittag/           #   bare git clone + tag listing
    node/             #   Node.js (official + unofficial builds)
    nodedist/         #   generic Node.js-style dist/index.json API
  render/             # installer script template rendering
  storage/            # release storage interface + implementations
    storage.go        # interface definition
    fsstore/          # filesystem (JSON cache, like current _cache/)
    pgstore/          # PostgreSQL (via sqlc + pgx)
  uadetect/           # User-Agent → OS/arch/libc detection (regex-based)
```

## Public API Surface (Must Remain Stable)

These are the endpoints that clients depend on. The URLs, query parameters, and
response formats must not change.

### Bootstrap (curl-pipe entry point)

```
GET /{package}            # User-Agent dispatch:
GET /{package}@{version}  #   curl/wget/POSIX → bash bootstrap script
                          #   PowerShell      → ps1 bootstrap script
                          #   Browser         → HTML cheat sheet (separate app)
```

### Installer Scripts

```
GET /api/installers/{package}.sh                # POSIX installer
GET /api/installers/{package}@{version}.sh
GET /api/installers/{package}.ps1               # PowerShell installer
GET /api/installers/{package}@{version}.ps1

Query: ?formats=tar,zip,xz,git,dmg,pkg
       &libc=msvc  (ps1 only)
```

### Release Metadata

```
GET /api/releases/{package}.json
GET /api/releases/{package}@{version}.json
GET /api/releases/{package}.tab
GET /api/releases/{package}@{version}.tab

Query: ?os=linux&arch=amd64&libc=musl
       &channel=stable&limit=10&formats=tar,xz
       &pretty=true
```

### Package Assets

```
GET /packages/{package}/README.md
GET /packages/{package}/{filename}
```

### Debug

```
GET /api/debug            # returns detected OS/arch from User-Agent
Query: ?os=...&arch=...   # overrides
```

### Response Formats

**JSON** — `{ oses, arches, libcs, formats, releases: [{ version, date, os,
arch, libc, ext, download, channel, lts, name }] }`

**TSV (.tab)** — `version \t lts \t channel \t date \t os \t arch \t ext \t - \t
download \t name \t comment`

## Architecture

### Two Servers

- **`webid`** — the HTTP API server. Renders templates and serves responses.
  On each request, looks up releases by package name in storage (filesystem
  and/or Postgres, configurable). No package registry — if releases exist in
  storage for that name, it's a valid package. No restart needed when packages
  are added.
- **`webicached`** — the cache daemon. Built with its package set compiled in.
  Periodically fetches releases from upstream sources, classifies builds, and
  writes to both Postgres and the filesystem. Adding a new package means
  rebuilding and redeploying `webicached`.

**Adding a new installer requires rebuilding `webicached`, but not `webid`.** The
API server discovers packages from storage — when the new `webicached` writes a
package's releases to Postgres or the filesystem, `webid` sees it on the next
read. No restart, no config reload.

This means `webid` never blocks on upstream API calls. It serves from whatever is
in storage — always fast, always available.

### Double-Buffer Storage

The storage layer uses a double-buffer strategy so that a full release-history
rewrite never disrupts active downloads:

```
Slot A: [current — being read by webid]
Slot B: [next — being written by webicached]

On completion: atomic swap A ↔ B
```

For **fsstore**: two directories per package, swap via atomic rename.
For **pgstore**: two sets of rows per package (keyed by generation), swap via
updating an active-generation pointer in a single transaction.

### Storage Interface

```go
type Store interface {
    // Read path (used by webid)
    GetPackageMeta(ctx context.Context, name string) (*PackageMeta, error)
    GetReleases(ctx context.Context, name string, filter ReleaseFilter) ([]Release, error)

    // Write path (used by webicached)
    BeginRefresh(ctx context.Context, name string) (RefreshTx, error)
}

type RefreshTx interface {
    PutReleases(ctx context.Context, releases []Release) error
    Commit(ctx context.Context) error   // atomic swap
    Rollback(ctx context.Context) error
}
```

### Resilient HTTP Client (`internal/httpclient`)

A `net/http` client with best-practice defaults, used as the base for all
upstream API calls:

- **Timeouts**: connect, TLS handshake, response header, overall request
- **Connection pooling**: sensible `MaxIdleConns`, `IdleConnTimeout`
- **TLS**: `MinVersion: tls.VersionTLS12`, system cert pool
- **Redirects**: limited redirect depth, no cross-scheme downgrades
- **User-Agent**: identifies as Webi with contact info
- **Retries**: exponential backoff with jitter for transient errors (429, 502,
  503, 504), respects `Retry-After` headers
- **Context**: all calls take `context.Context` for cancellation
- **No global state**: created as instances, not `http.DefaultClient`

### Release Fetchers (`internal/releases/`)

Each upstream source (GitHub, Gitea, git-tag) is a small package that uses
`httpclient` and returns a common `[]Release` slice. No SDK dependencies.

```go
// internal/releases/github/github.go
func FetchReleases(ctx context.Context, client *httpclient.Client,
    owner, repo string, opts ...Option) ([]Release, error)
```

### Build Classification (`internal/classify`)

The classifier is the **80/20 default** — it handles the happy path where
standard toolchains (goreleaser, cargo-dist, Zig, Rust) produce predictable
filenames. It is not the authority; the per-installer config can override
anything it detects.

- Regex-based detection with priority ordering (x86_64 before x86, arm64
  before armv7, amd64v4/v3/v2 before baseline).
- OS-aware fixups: bare "arm" on Windows → ARM64.
- Accepts filenames or full download URLs (signal may be in path segments).
- Undetected fields are empty, not guessed.

Target triplet format: `{os}-{arch}-{libc}`.

### Fallback & Compatibility

Arch and libc fallbacks are **not universal rules**. They vary by OS, package,
and even package version:

- **OS-level arch compat** (`buildmeta.CompatArches`): universal facts like
  "darwin arm64 runs x86_64 via Rosetta 2", "windows arm64 emulates x86_64".
  Includes macOS Universal1 (PPC+x86) and Universal2 (x86_64+ARM64).
- **Libc compat**: per-package, per-version. Musl can be static (runs anywhere)
  or dynamically linked (needs polyfill). Windows GNU can be dependency-free or
  need mingw. This changes between versions of the same package.
- **Arch micro-levels**: amd64v4→v3→v2→v1 fallback is universal, but a package
  may drop specific micro-arch builds between versions.

Per-installer config declares the package-specific rules. The resolver combines
installer config + platlatest + CompatArches to pick the right binary.

### Installer Rendering (`internal/render`)

Replaces `installers.js`. Reads template files, substitutes variables, injects
the per-package `install.sh` / `install.ps1`.

The current template variable set (30+ env vars) is the contract with the
client-side scripts. We must produce identical output for `package-install.tpl.sh`
and `package-install.tpl.ps1`.

### Reworking install.sh / install.ps1

Long-term, the per-package install scripts should feel like library users, not
framework callbacks:

- **Current (framework):** define `pkg_install()`, `pkg_get_current_version()`,
  etc. and the framework calls them.
- **Goal (library):** source a helpers file, call functions like
  `webi_download`, `webi_extract`, `webi_link` explicitly from a linear script.

This is a **separate migration** from the Go rewrite — it changes the client-side
contract. Plan it but don't block the server rewrite on it.

## Migration Strategy

Each phase produces something that works in production alongside the existing
Node.js server.

### Phase 0: Foundation

- [x] `internal/buildmeta` — shared vocabulary (OS, arch, libc, format, channel)
- [x] `internal/buildmeta` — `CompatArches(os, arch)` — OS-level arch compat facts
- [x] `internal/buildmeta` — amd64 micro-arch levels (v1–v4), universal binary types
- [x] `internal/lexver` — version strings → comparable strings
- [x] `internal/httpclient` — resilient HTTP client for upstream API calls
- [x] `internal/uadetect` — User-Agent → OS/arch/libc (regex-based)
- [x] Go module init (`go 1.26.1`, stdlib only)
- [ ] CI setup
- [ ] CPU micro-arch detection in bootstrap scripts (POSIX + PowerShell)

### Phase 1: Release Fetching & Caching

- [x] `internal/releases/githubish` — generic GitHub-compatible API fetcher
- [x] `internal/releases/github` — GitHub releases (thin wrapper)
- [x] `internal/releases/githubsrc` — GitHub source archives
- [x] `internal/releases/gitea` — Gitea/Forgejo releases (own types)
- [x] `internal/releases/giteasrc` — Gitea source archives
- [x] `internal/releases/gitlab` — GitLab releases (own types, X-Total-Pages)
- [x] `internal/releases/gitlabsrc` — GitLab source archives
- [x] `internal/releases/gittag` — git tag listing (bare clone)
- [x] `internal/releases/nodedist` — Node.js-style dist/index.json API
- [x] `internal/releases/node` — Node.js (official + unofficial builds)
- [x] `internal/releases/chromedist` — Chrome/Chromedriver JSON endpoint
- [x] `internal/releases/flutterdist` — Flutter SDK release index
- [x] `internal/releases/golang` — Go downloads page JSON
- [x] `internal/releases/gpgdist` — GnuPG FTP directory scraper
- [x] `internal/releases/hashicorp` — HashiCorp releases API
- [x] `internal/releases/iterm2dist` — iTerm2 downloads page scraper
- [x] `internal/releases/juliadist` — Julia versions JSON API
- [x] `internal/releases/mariadbdist` — MariaDB downloads page
- [x] `internal/releases/zigdist` — Zig downloads JSON
- [x] Per-package releases packages: bun, fish, git, lsd, ollama, postgres,
  pwsh, watchexec, xcaddy (variant tagging, version normalization, legacy data)
- [x] `internal/rawcache` — double-buffered raw upstream response storage
- [x] `internal/classify` — build artifact classifier (80/20, filename→target)
- [x] `internal/platlatest` — per-platform latest version index
- [x] End-to-end: fetch complete histories for all 103 packages
- [x] `internal/installerconf` — key=value config parser (source inferred from key)
- [x] `internal/classifypkg` — full classification pipeline:
  classifySource → TagVariants → NormalizeVersions → processGitTagHEAD →
  ApplyConfig → appendLegacy
- [x] Per-package variant taggers (pwsh, ollama, bun, node)
- [x] Per-package version normalizers (git, lf, go, postgres, watchexec)
- [x] Gittag HEAD handling (tagless→v{datetime}, mixed→exclude from legacy)
- [x] Legacy releases (postgres EnterpriseDB 10.x–12.x via appendLegacy)
- [x] Resolver (platlatest + installer config + CompatArches → pick binary)
- [x] `internal/resolve` — Best() with arch fallback, libc waterfall, format preference
- [x] `internal/resolver` — high-level Resolve() with triplet enumeration
- [x] `internal/storage` — interface definition (Asset, PackageData, Store, RefreshTx)
- [x] `internal/storage/legacy.go` — LegacyAsset/LegacyCache with variant/format filtering
- [x] `internal/storage/fsstore` — filesystem implementation (atomic writes, alias symlinks)
- [x] `cmd/webicached` — cache daemon (round-robin refresh, rate limiting, symlink detection)
- [x] `cmd/comparecache` — Go vs Node.js cache comparison tool
- [x] Legacy cache generation verified for 101 packages

**Integration point:** `webicached` writes the same `_cache/` JSON format. The
Node.js server can read from it. Zero-risk cutover for release fetching.

### Phase 2: Release API

- [x] `cmd/webid` — HTTP server skeleton with middleware
- [x] `GET /api/releases/{package}.json` endpoint
- [x] `GET /api/releases/{package}.tab` endpoint
- [x] `GET /api/debug` endpoint

**Integration point:** reverse proxy specific `/api/releases/` paths to the Go
server. Node.js handles everything else.

### Phase 3: Installer Rendering

- [x] `internal/render` — template engine (Bash + PowerShell)
- [x] `GET /api/installers/{package}.sh` endpoint
- [x] `GET /api/installers/{package}.ps1` endpoint
- [x] Bootstrap endpoint (`GET /{package}`)

**Integration point:** reverse proxy installer paths to Go. Node.js only serves
the website/cheat sheets (if it ever did — that may be a separate app).

### Phase 4: PostgreSQL Storage

- [ ] `internal/storage/pgstore` — sqlc-generated queries, double-buffer
- [ ] Schema design and migrations
- [ ] `webicached` writes to Postgres
- [ ] `webid` reads from Postgres

### Phase 5: Client-Side Rework

- [ ] Design new library-style install.sh helpers
- [ ] Migrate existing packages one at a time
- [ ] Update `package-install.tpl.sh` to support both old and new styles

## Key Design Decisions

### Package Configuration (`releases.conf`)

Each package has a `{pkg}/releases.conf` — a flat `key = value` file parsed by
`internal/installerconf`. This replaces the per-package `releases.js` from Node.js.

The source type is inferred from the primary key:

```
github_repo = BurntSushi/ripgrep
```

```
git_url = https://github.com/tpope/vim-commentary.git
```

```
gitea_repo = root/pathman
base_url = https://git.rootprojects.org
```

```
hashicorp_product = terraform
```

One-off dist sources use an explicit `source` key:

```
source = nodedist
url = https://nodejs.org/download/release
```

Source types: `github`, `gitea`, `gittag`, `hashicorp`, `nodedist`, `chromedist`,
`flutterdist`, `golang`, `gpgdist`, `iterm2dist`, `juliadist`, `mariadbdist`,
`zigdist`.

**Multi-source packages** (like `node`, which merges official + unofficial builds)
are not yet supported by the config format. Current workaround: separate packages
(`node-official`, `node-unofficial`). Redesign needed — see Open Questions.

Unknown keys go into `conf.Extra` (a `map[string]string`). The `alias_of` key
marks a package as a symlink to another (e.g. `alias_of = dashcore`).

### Asset Model and `Extra` Field

`storage.Asset` represents a single downloadable file. Key fields:

```go
type Asset struct {
    Filename string  // "bat-v0.26.1-x86_64-unknown-linux-musl.tar.gz"
    Version  string  // "v0.26.1"
    OS       string  // "linux"
    Arch     string  // "x86_64"
    Libc     string  // "musl"
    Format   string  // ".tar.gz"
    Channel  string  // "stable"
    Extra    string    // extra version info for sorting
    Variants []string  // ["rocm"], ["installer"], ["fxdependent"], etc.
    Download string    // full URL
    ...
}
```

The `Extra` field captures **build variants** — assets that target a specific
hardware or runtime configuration beyond OS/arch/libc:

- `rocm` — AMD GPU compute (ollama)
- `jetpack5`, `jetpack6` — NVIDIA Jetson SDK (ollama)
- `fxdependent`, `fxdependentWinDesktop` — .NET framework-dependent (pwsh)
- `profile` — debug profiling build (bun)
- `source` — source archive, not a binary
- `installer` — `.exe` that is a GUI installer, not the actual tool binary

The resolver **deprioritizes** assets with non-empty `Variants` — they're only
selected when the user explicitly requests that variant (e.g., `?variant=rocm`).
The full API still serves them for broader use cases.

**Not a variant — arch micro-levels**: Bun's "baseline" is actually `amd64` (v1),
and the non-baseline is `amd64v3`. These use the `Arch` field directly, and the
resolver's existing fallback chain (`amd64v3` → `amd64v2` → `amd64`) handles
selection naturally.

### Format Classification

All assets are stored — nothing is dropped at classification time.

Most formats are extractable and need no special tagging — the file extension
is enough signal for the install scripts:

- `.tar.gz`, `.tar.xz`, `.tar.zst`, `.zip`, `.7z` — standard archives
- `.pkg` — macOS: extractable via `pkgutil --expand-full` (flat files, no install)
  See: https://coolaj86.com/articles/how-to-extract-pkg-files-and-payload/
- `.deb` — extractable via `ar x` + `tar xf data.tar.*`
- `.dmg` — mountable via `hdiutil attach`
- `.msi` — extractable via `msiexec /a` or `lessmsi`
- `.AppImage` — self-contained, chmod +x and run

The one ambiguous format is `.exe` — it could be a bare binary (the actual tool)
or a GUI installer (which can't be automated). Assets where `.exe` is an installer
(not the tool itself) get `Variants: ["installer"]` so the resolver skips them
by default.

### Legacy Export Filtering

During migration, `fsstore` writes JSON in the Node.js `_cache/` format. The
Node.js server reads this directly. Two filters apply at export time
(`storage.ExportLegacy`):

1. **Build variants**: Assets with non-empty `Variants` are stripped (Node.js
   doesn't know about rocm/jetpack/fxdependent/head/appimage)
2. **Format**: Assets with formats the Node.js server doesn't recognize are
   stripped (recognized: tar.gz, zip, xz, pkg, msi, exe, dmg, git, etc.)

This keeps the primary Go pipeline complete while the legacy path stays compat.

### Version: Go 1.26+

Using `http.ServeMux` with `PathValue` for routing (available since Go 1.22).
Middleware via `github.com/therootcompany/golib/http/middleware/v2`.

### No ORM

PostgreSQL access via `pgx` + `sqlc`. Queries are hand-written SQL, type-safe
Go code is generated.

### Template Rendering

Use `text/template` or simple string replacement (matching current behavior).
The templates are shell scripts — they need literal `$` and `{}` — so
`text/template` may be the wrong tool. Likely better to stick with the current
regex-replacement approach, ported to Go.

### Error Handling

The current system returns a synthetic "error release" (`version: 0.0.0`,
`channel: error`) when no match is found, rather than an HTTP error. This
behavior must be preserved for backward compatibility.

## Open Questions

- [ ] **Multi-source config**: `node` needs both official + unofficial URLs.
  Current releases.conf only supports one `source`. Node's classifier handles
  this via a special case (`unofficial_url` in Extra). Needs a cleaner design.
- [ ] What's the deployment topology? Single binary serving both roles? Separate
  processes? Kubernetes pods?
- [ ] Per-installer config format: what structure best expresses version-ranged
  libc overrides, arch fallback overrides, and nonstandard asset naming? Go
  struct + TOML/YAML? Go code (compiled into webicached)?
- [ ] CPU micro-arch detection: how should POSIX and PowerShell bootstrap scripts
  detect amd64v1/v2/v3/v4? Check /proc/cpuinfo flags (Linux), sysctl
  hw.optional (macOS), .NET intrinsics (Windows)?
- [ ] **Variant selection API**: How do users request variant builds? Query param
  (`?variant=rocm`)? User-Agent hint? Per-installer default override?

## Resolved Questions

- **Shell out to `node releases.js`?** No — all source types are implemented in
  Go. The Go pipeline fetches and classifies everything directly.
- **Asset.Extra vs Variants?** `Extra` stays as version-related sort info.
  New `Variants []string` field captures build qualifiers (rocm, installer,
  fxdependent, head, appimage, win-version-specific). Resolver deprioritizes
  assets with any variants.
- **Rate limiting for GitHub API calls?** `webicached` uses round-robin refresh
  (one package per tick) with `--page-delay` (default 2s) via a `delayTransport`
  wrapper. No coordination needed — single instance by design.
- **Config format for source/owner/repo?** Collapsed into single keys:
  `github_repo = owner/repo`, `git_url = ...`, `gitea_repo = owner/repo`,
  `hashicorp_product = name`. Source type inferred from the key.
- **Per-package version normalization?** Lives in Go code per-package (e.g.
  `internal/releases/postgres/versions.go`), not in config. Each package that
  needs it implements a `NormalizeVersions([]Asset)` function called from
  `classifypkg.NormalizeVersions`.

## Current Node.js Architecture (Reference)

For context, the current system's key files:

| File | Role |
|------|------|
| `_webi/serve-installer.js` | Main request handler — dispatches to builds + rendering |
| `_webi/builds.js` | Thin wrapper around builds-cacher |
| `_webi/builds-cacher.js` | Release fetching, caching, classification, version matching |
| `_webi/transform-releases.js` | Legacy release API (filter + cache + serve) |
| `_webi/normalize.js` | OS/arch/libc/ext regex detection from filenames |
| `_webi/installers.js` | Template rendering (bash + powershell) |
| `_webi/ua-detect.js` | User-Agent → OS/arch/libc |
| `_webi/projects.js` | Package metadata from README frontmatter |
| `_webi/frontmarker.js` | YAML frontmatter parser |
| `_common/github.js` | GitHub releases fetcher |
| `_common/gitea.js` | Gitea releases fetcher |
| `_common/git-tag.js` | Git tag listing |
| `{pkg}/releases.js` | Per-package release config (fetcher + filters + transforms) |
| `{pkg}/install.sh` | Per-package POSIX installer |
| `{pkg}/install.ps1` | Per-package PowerShell installer |
