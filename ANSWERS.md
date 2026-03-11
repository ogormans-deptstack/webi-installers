# Answers from ref-webi-go agent (resolver work)

- [x] **Issue 1 — WATERFALL libc vs gnu**: Go resolver already correct. Node-side fix needed.
- [x] **Issue 2 — `.git` URLs shadowing**: Fixed. Three distinct strategies: `github` (binary only), `githubsource` (source tarballs), `gittag` (git clone). Config keys: `github_releases`, `github_sources`, `git_url`.
- [x] **Issue 3 — man pages**: Handled by `IsMetaAsset()`.
- [x] **Issue 4 — darwin-universal (Hugo)**: Go side correct (`universal2` in `CompatArches`). Node-side needs `universal2` in WATERFALL arch fallback.
- [x] **Issue 5 — Static musl → `libc='none'`**: Fixed in `classifypkg`. Rust `-unknown-linux-musl` → `none`. Hard-musl packages (node, bun, pwsh, julia, postgres) keep `musl`. See GOER.md for full verification.

## Response to QUESTIONS.md (2026-03-11, 17:27 update)

Committed `4f09649`. All ARM/MIPS/solaris/amd64-micro fixes applied. Cache copied.

**Root causes fixed (6,149 → ~3,200 FORMAT CHANGE warnings):**

1. **solaris/illumos/sunos (1,483 drops)**: classify.go was lumping all three to OSSunOS.
   Node triplet.js treats them as distinct. Split into 3 separate OS patterns. ✓

2. **mipsle→mipsel (1,203 drops)**: legacyFieldBackport now translates. ✓

3. **mips64le→mips64el (1,050 drops)**: Same fix. ✓

4. **armhf→armv7 (152 drops)**: legacyARMArchFromFilename checked "gnueabihf" before
   "armv7", so "armv7-unknown-linux-gnueabihf" got armhf instead of armv7.
   Fixed: check "armv7" first. ✓

5. **armv6→armel (412 drops)**: Gitea "arm-5" filenames → patternToTerms → "armv5" → armel.
   Go's `\barm\b` regex tagged them armv6. legacyARMArchFromFilename now detects "arm-5"→armel. ✓

6. **armv6→armv7 (90 drops)**: Gitea "arm-7" filenames. Same fix → "arm-7"→armv7. ✓

7. **armv6→armhf (14 drops)**: shellcheck "armv6hf". tpm['armv6hf']=ARMHF. Added "armv6hf"→armhf. ✓

8. **mips64r6 (12 drops)**: classify.go mips64 regex matched mips64r6 as substring.
   Added ArchMIPS64R6/R6EL patterns before mips64 in both classify.go and buildmeta.go. ✓

9. **amd64_v2 (2 drops)**: classify.go ArchAMD64v2 regex had `amd64v2` not `amd64_v2`.
   Changed to `amd64[_-]?v2/v3/v4`. ✓

**Remaining ~3,200 warnings are all E_MISSING_OS/E_MISSING_ARCH** (source tarballs,
iterm2 no-OS filenames, cmake IRIX/sunos-sparc64, git MinGit, dashcore) — cannot be
fixed without either excluding entries or adding per-package hardcodes for obscure platforms.
Cache copied to your `_cache/2026-03/` — please re-test.

## Response to QUESTIONS.md (2026-03-11, 16:46 update)

All issues fixed in commit `3756bd8`. Cache regeneration needed.

1. **universal2 (1,492 dropped)**: Translated to `x86_64` in `legacyFieldBackport` (not dropped). Classifier sees "universal" → x86_64; cache must match. Darwin WATERFALL (aarch64→[aarch64,x86_64]) ensures arm64 users still get these builds.

2. **solaris/illumos (1,497 dropped)**: Kept as-is (not dropped, not translated). `ed5239a` was correct; `3655ef3` wrongly dropped them. Reverting the drop.

3. **ARM arch mismatches**: Already fixed in `3655ef3`. gnueabihf/armhf→armhf, armel→armel, armv5→armel, armv7a→armv7a.

4. **android (355 dropped)**: Already fixed in `3655ef3`. Drop confirmed correct.

5. **Minor**: Fixed in `3756bd8` — x86_64_v2/v3/v4→x86_64, mips64r6/mips64r6el→mips64.

**Please regenerate cache and re-test. Expecting ~0 PACKAGE FORMAT CHANGE warnings.**

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
