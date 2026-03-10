# Classification Rules & Learnings

Tracking classifier decisions and edge cases discovered during batch processing.

## Batch 1 (go, node, hugo, caddy, pathman)

### Vocabulary

All values in the CSV use buildmeta canonical names:
- Arch: `x86_64` (not amd64), `aarch64` (not arm64), `x86` (not 386/i386)
- Format: `.tar.gz` (with leading dot), matching buildmeta.Format constants
- OS: `darwin` (not mac/macos), `dragonfly` (not dragonflybsd)

### Classifier Additions

From this batch, these patterns were added to the generic classifier:

**OS:**
- `mac` (word boundary) → darwin (caddy uses `mac_amd64`)
- `openbsd`, `netbsd`, `dragonfly(?:bsd)?`, `plan9` → new OS types
- `.deb`/`.rpm` → infer Linux when OS undetectable from filename

**Arch:**
- `386` (word boundary) → x86 (Go naming convention)
- `32bit`/`64bit` (no hyphen) → x86/x86_64 (Hugo naming)
- `arm7` → armv7 (old caddy naming)
- `armhf` → armv7 (Debian convention)
- `armv5` → new arch type
- `universal` → universal2 (Hugo fat binaries)
- `riscv64`, `loong64`, `mipsle`, `mips64le` → new arch types

### Per-Source Normalizers

Each upstream API uses different naming. Normalizers convert to buildmeta vocabulary:

| Source    | "amd64"      | "arm64"      | "arm"   |
|-----------|-------------|-------------|---------|
| Go API    | amd64→x86_64 | arm64→aarch64 | -       |
| Node dist | x64→x86_64   | arm64→aarch64 | -       |
| Zig       | x86_64 (same)| aarch64 (same)| -       |
| HashiCorp | amd64→x86_64 | arm64→aarch64 | arm→armv6 |
| Julia     | x86_64 (same)| aarch64 (same)| -       |
| Chrome    | x64→x86_64   | arm64→aarch64 | -       |
| MariaDB   | x86_64 (same)| aarch64 (same)| -       |

### Meta-Asset Filtering

Skipped patterns: checksums (.sha256, .sha512, .md5, checksums.txt),
signatures (.sig, .asc, .pem), SBOMs (.sbom, .spdx, .sigstore),
source archives (`_src.tar.gz`), and `buildable-artifact`.

### Edge Cases (Accepted)

- Caddy v2 beta bare binaries (`caddy2_beta12_macos`) — no arch in filename, shows empty
- Hugo `macOS-all` — means universal but only 2 files, not worth special-casing
- Hugo extended editions — detected via `extended` in filename, tracked in `extra` column? (TODO: not yet)
- Node "odd major = beta" heuristic — v15, v17, v19, v21, v23 are "current" not LTS
- Go version prefix: stripped `go` from `go1.23.6` → `1.23.6` for clean parsing

## Batch 2 (zig, flutter, chromedriver, terraform, julia, iterm2, mariadb, gpg, serviceman, aliasman)

### Zig Fetcher Fix

The zig upstream API returns `"size"` as a JSON string, not a number.
Changed `Platform.Size` from `int64` to `json.Number` to avoid unmarshal failures.
Also changed `Platforms` tag from `json:"-"` to `json:"platforms,omitempty"` so
platform data is preserved in cache.

### Source-Only Packages

serviceman and aliasman have GitHub releases with empty `assets:[]`. These are
source-only repos that install via `go install` or script download, not binary
releases. The classifier correctly produces 0 distributables for them — they
don't belong in the binary CSV.

### Flutter Arch Detection

Early Flutter releases (pre-2020) had no arch-specific builds — single
platform SDK. No arch in filename → empty arch in CSV. This is correct;
the installer would default to x86_64 on supported platforms.

## Batch 3 (25 packages: arc through gitdeploy)

### New Classifier Patterns

- `macosx` → darwin (syncthing uses `macosx`)
- `ia32` → x86 (dart-sass uses `ia32`)
- `.snap` format → Linux-only
- `.appx` format added for PowerShell

### New Meta-Asset Filters

- `.pub` (cosign keys)
- `install.sh`, `install.ps1` (install scripts)
- `compat.json` (syncthing metadata)

## Batch 4 (62 remaining packages) + Full Run

### Hugo/Hugo-Extended Split

hugo-extended shares the same GitHub repo as hugo. Added `asset_filter` and
`asset_exclude` conf keys to split them:
- `hugo/releases.conf`: `asset_exclude = extended` (6,354 assets)
- `hugo-extended/releases.conf`: `asset_filter = extended` (2,193 assets)

User direction: "hugo-extended should be a separate release. I believe the
README covered this. I think it should have been the default."

### Remaining Empty-Field Patterns (Per-Installer Territory)

These have empty OS or arch from the generic classifier and need per-installer
config to resolve:
- Git-for-Windows: `Git-2.x.x-32-bit.tar.bz2` — no OS in filename, always Windows
- CMake: HP-UX, IRIX targets — exotic/dead platforms
- Dashcore: old naming conventions
- Old PowerShell `.msi` files — no arch in filename
- Bare binaries (ollama-darwin, caddy2_beta12_macos) — no arch info

### Full Results

169,867 distributable rows across 116 packages.
3 packages produce 0 rows: serviceman, aliasman (source-only), duckdns.sh.

### TODO

- Consider whether bare binaries (no format extension) should get a format marker
- Per-installer configs for packages with known-but-undetectable OS/arch
- `arm32` classification: leave to per-installer unless pattern emerges
- `arm32` is vague — may mean armv6 or armv7. Leave as per-installer responsibility
  unless a distinct pattern emerges (user direction 2026-03-10)
