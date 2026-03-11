# Status from Go agent (ref-webi-go)

## Current status: Phases 0–4 complete, integration testing clean

Node agent test results (as of 2026-03-11):
- **15/15** installer-resolve
- **49/49** live-compare (5 known — improvements over production)
- **190/196** broad sweep (6 expected: git/gpg/iterm2/mariadb have no binaries)
- ~7,400 PACKAGE FORMAT CHANGE warnings — **informational only**, don't block resolution

## Completed (Phase 4)

- [x] pgstore: PostgreSQL storage (pgx v5, double-buffer, CopyFrom bulk insert)
- [x] storage.Store interface: added ListPackages(ctx); both fsstore and pgstore implement
- [x] webid + webicached: -pg=<dsn> flag to select pgstore over fsstore
- [x] golib middleware: request logging via middleware.WithMux
- [x] Legacy cache generation working for 101 packages (aliases/symlinks excluded)

## Completed (classifier fixes 2026-03-11)

- [x] Removed Windows arm→arm64 auto-promotion (caddy/fzf/goreleaser had explicit armv6 builds being wrongly promoted)
- [x] armel → armv6 (jq, caddy and others)
- [x] gnueabihf → armv6 explicit pattern (belt-and-suspenders for Rust triplets)
- [x] winx64 → windows/x86_64 (MariaDB)
- [x] ppc64el → ppc64le alias (Debian naming, jq)
- [x] armv6l → armv6 in normalizeGoArch (Go dist API used armv6l for older releases)
- [x] GPG classifier: hardcoded "amd64" → buildmeta.ArchAMD64 ("x86_64")
- [x] solaris/illumos → sunos in legacy export (Node.js only knows "sunos")
- [x] universal2: kept as-is in cache; Node agent handles via WATERFALL
- [x] go armv6→arm backport removed; Node classifier expects armv6 for armv6l filenames

## Completed (earlier)

- [x] Libc classification: Rust static musl → `none`, hard-musl packages keep `musl`
- [x] Windows gnu→none: MinGW self-contained
- [x] install.sh 8-space padding, PowerShell rendering
- [x] pg asset_filter: server-only assets (includes client)
- [x] comparecache field-level diffs: os/arch/libc/ext/channel with equivalence matching
- [x] Go dist: use API structured os/arch (illumos/solaris kept distinct in canonical storage)
- [x] sass: bare arm → armv7 in classifier (Dart Sass targets ARMv7)
- [x] LegacyBackport: moved to ExportLegacy (canonical values in pgstore, translations at export)
  - ffmpeg: Windows .gz → exe (prod releases.js override)
- [x] .apk/.AppImage: added to classifier and correctly dropped from legacy export

## Researcher answers (ANYOS-first)

Confirmed harmless. Go specific-OS-first order (`[osStr, posix_2024, posix_2017, ANYOS, ""]`)
produces same results as production. No change needed.

## comparecache status: 3 remaining diffs (all production bugs)

| Package | Field | Count | Go (correct) | Prod (wrong) | Why |
|---------|-------|-------|-------------|-------------|-----|
| iterm2 | channel | 11 | stable | beta | Raw cache says stable (URL path `/stable/`). Prod misclassifies. |
| postgres | ext | 2 | .tar.gz | tar | normalize.js strips compression layer. File IS .tar.gz. |
| terraform | channel | 14 | alpha | stable | Version contains `-alpha-`. Prod regex misses it. |

All other packages (98/101) have zero field-level disagreements.
Not backporting these — Go is correct, production has bugs.

## Known gaps

- **atomicparsley**: `AtomicParsleyAlpine.zip` — needs per-package classifier config (hard musl).
  RESEARCHER confirmed: needs a `releases.conf` with asset pattern overrides, not generic detection.
- **Hugo .pkg**: macOS v0.153+ only ships `.pkg`. Latent bug in both Go and production.
- **~3,200 remaining FORMAT CHANGE warnings** (down from 6,149): all E_MISSING_OS/E_MISSING_ARCH
  from source tarballs (cmake, git), iterm2 no-OS filenames, dashcore, cmake obscure platforms.
  Cannot reduce further without per-package hardcodes or exclusion rules.

## Classifier fixes (2026-03-11, 17:27)

- [x] solaris/illumos/sunos split into 3 separate OS patterns in classify.go
- [x] mipsle→mipsel, mips64le→mips64el in legacyFieldBackport
- [x] ARM priority: armv7 before gnueabihf in legacyARMArchFromFilename
- [x] Gitea arm-5→armel, arm-7→armv7 in legacyARMArchFromFilename
- [x] armv6hf→armhf in legacyARMArchFromFilename
- [x] ArchMIPS64R6/R6EL added to buildmeta.go + classify.go (before mips64)
- [x] amd64[_-]?v2/v3/v4 regex in classify.go (matches underscore form)
