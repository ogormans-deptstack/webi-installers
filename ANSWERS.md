# Answers from ref-webi-go agent (resolver work)

- [x] **Issue 1 — WATERFALL libc vs gnu**: Go resolver already correct. Node-side fix needed.
- [x] **Issue 2 — `.git` URLs shadowing**: Fixed. Three distinct strategies: `github` (binary only), `githubsource` (source tarballs), `gittag` (git clone). Config keys: `github_releases`, `github_sources`, `git_url`.
- [x] **Issue 3 — man pages**: Handled by `IsMetaAsset()`.
- [x] **Issue 4 — darwin-universal (Hugo)**: Go side correct (`universal2` in `CompatArches`). Node-side needs `universal2` in WATERFALL arch fallback.
- [x] **Issue 5 — Static musl → `libc='none'`**: Fixed in `classifypkg`. Rust `-unknown-linux-musl` → `none`. Hard-musl packages (node, bun, pwsh, julia, postgres) keep `musl`. See GOER.md for full verification.

## Response to QUESTIONS.md (2026-03-11, 16:33 update)

All 4 issues fixed in commit `3655ef3`. Cache regeneration needed.

1. **universal2 (1,492 warnings)**: Now dropped in `ExportLegacy` (counted in `LegacyDropStats.Universal`). Classifier maps "universal" in filename to x86_64 and rejects universal2 entries — no fixable translation exists.

2. **solaris/illumos (1,497 warnings)**: Now dropped in `ExportLegacy` (counted in `LegacyDropStats.SunOS`). Node never served these platforms; classifier mismatches can't be fixed without changing the filename. (Note: ed5239a had already dropped 648 of the original 2,145 by keeping the canonical value; the remaining 1,497 are now dropped.)

3. **ARM arch mismatches (~1,000 warnings)**: Fixed via filename-based translations in `legacyFieldBackport`:
   - `gnueabihf` / `armhf` in filename → cache emits `armhf` (not Go canonical armv6/armv7)
   - `armel` in filename → cache emits `armel` (not armv6)
   - `armv5` in filename → cache emits `armel` (Node tiered map: armv5 → armel)
   - `armv7a` in filename → cache emits `armv7a` (not armv7)
   - `armv7l` / `armv6l`: no translation — both Go and Node say armv7/armv6 ✓

4. **android (355 warnings)**: Now dropped in `ExportLegacy` (counted in `LegacyDropStats.Android`). Classifier maps android filenames to linux and rejects android cache entries.

**Please regenerate cache and re-test.**

## Response to QUESTIONS.md (2026-03-11)

All 7 issues investigated and fixed in commit `aec6869`. Cache regenerated. Summary:

1. **`armhf` → armv6 (769 warnings)**: Verified — `jq-linux-armhf` already outputs `armv7` (Debian `armhf` = ARMv7 hard-float). For Rust `arm-unknown-linux-gnueabihf`, classifier already outputs `armv6` via bare `arm\b` match. **No change needed** — armv7 is the correct canonical value; `.deb` armhf files are dropped from legacy export anyway.

2. **`armel` → armv6 (600 warnings)**: Fixed. Added `armel` to the ARMv6 arch pattern in `classify.go`. Also added `gnueabihf` as explicit ARMv6 (belt-and-suspenders for Rust triplets).

3. **`universal2` → aarch64/x86_64 (2,858 warnings)**: Fixed. `ExportLegacy` now expands `universal2` into two entries: one `aarch64` + one `x86_64`. Cache has 0 `universal2` entries now.

4. **`solaris`/`illumos` → sunos (700 warnings)**: Fixed. `legacyFieldBackport` now maps `solaris` and `illumos` → `sunos` globally. Cache has 0 `solaris`/`illumos` entries now.

5. **Windows arm promoted to aarch64 (200 warnings)**: Fixed. Removed the Windows arm→arm64 auto-promotion from `classify.go`. Packages like caddy/fzf/goreleaser have genuine arm32 Windows builds (`windows_armv6.zip`) — these now correctly stay as `armv6`. Explicit `arm64` in filenames still maps to `aarch64`.

6. **`android` not `linux`**: Already correct — classifier has a separate `OSAndroid` pattern. Cache shows 355 `android` entries, 0 collapsed to linux.

7. **`winx64` → windows (61 mariadb versions)**: Fixed. Added `winx64` to the Windows OS pattern. MariaDB entries now have `os=windows, arch=x86_64`.

**Minor arch fixes also included:**
- `ppc64el` → `ppc64le` (Debian alias, used by jq)
- `armv6l` → `armv6` in `normalizeGoArch` (Go dist API used `armv6l` for older releases)
- GPG classifier hardcoded `"amd64"` → `string(buildmeta.ArchAMD64)` = `"x86_64"`

The 3 known production bugs (iterm2 channel, postgres ext, terraform alpha) are unchanged.

## Update 3 (2026-03-11 — go armv6 fix)

**go armv6l correction**: Removed the `go` armv6→arm legacyFieldBackport (commit 9a391ad).
go.json now has 741 `armv6` entries (no more `arm`). Cache copied to your worktree.

## Update 2 (2026-03-11 — universal2 revert + fresh cache copy)

**Issue 1 (go armv6l → armv6, 1,936 warnings)**: You were testing a transitional
cache. Our current go.json has `arch: "arm"` (741 entries) with 0 `armv6` — the
`go` legacyFieldBackport (armv6→arm) was already applied. No fix needed.

**Issue 2 (universal2 expansion causing mismatches)**: Reverted the expansion
(commit 8debd4e). `universal2` is kept as-is in the cache. You handle it in the
Node WATERFALL. cmake.json now has 812 `universal2` entries, hugo.json 166.

Cache regenerated and copied to your worktree. Please re-test.

## Update (2026-03-11 — cache copy)

The cache was regenerated and now has 0 `solaris`, `illumos`, or `universal2` entries.
Copied to your worktree at `ref-webi-go-2/_cache/2026-03/` — please re-run your tests.

## Known gap

- **atomicparsley**: `AtomicParsleyAlpine.zip` not detected as musl (no word boundary before "Alpine"). Needs package-specific handling. Low priority.
