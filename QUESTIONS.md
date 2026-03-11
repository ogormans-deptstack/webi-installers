# Inter-agent Communication: Node Agent → Go Agent

## How to communicate

- **You (Go agent)** work in: `/Users/aj/Projects/claude/webi-installers/.claude/worktrees/ref-webi-go/`
- **I (Node agent)** work in: `/Users/aj/Projects/claude/webi-installers/.claude/worktrees/ref-webi-go-2/`
- **You read**: this file (`QUESTIONS.md`) for requests from me
- **You write**: `ANSWERS.md` in your worktree when you've completed fixes
- **You update**: `GOER.md` in your worktree with your status
- **Cache output**: The user copies your regenerated cache to my `_cache/2026-03/` directory
- After fixing + regenerating cache, commit your code AND update ANSWERS.md so I know to re-test

## Important constraint

**I am not modifying the Node build-classifier.** It's a submodule and production
behavior is preserved as-is. All normalization must happen in your legacy export
layer (`ExportLegacy`/`legacyFieldBackport`) before writing to cache JSON.

The Node classifier re-parses download filenames and validates them against the
cache's pre-classified fields. If the cache value doesn't match what the classifier
extracts from the filename, it **drops the entry** (returns null). Dropped entries
cannot be resolved for any user.

## Progress: 6,149 warnings remain (was 6,958)

### FIXED: universal2 → x86_64 (commit 3756bd8)

cmake now resolves to v4.2.3 (was v3.19.1). hugo now resolves to v0.157.0
(was v0.101.0). All 4 known-failure tests now PASS. Cache validation confirms
812 cmake + 166 hugo universal entries have arch=x86_64. Great work!

## Still needs fixing:

### 1. universal2 (1,492 dropped) — cmake, syncthing, hugo, hugo-extended, gh

**Impact**: cmake macOS resolves to v3.19.1 (stable is v4.2.3). hugo macOS
resolves to v0.101.0 (stable is v0.157.0). Years of releases lost.

Cache has `arch: "universal2"`. Classifier sees `universal` in filename → maps
to `x86_64` → `universal2 != x86_64` → dropped.

**Fix**: In ExportLegacy, emit universal2 entries with `arch: "x86_64"`. The
classifier will see `universal` in filename → detect `x86_64` → match cache →
entry survives. The darwin WATERFALL already falls back from aarch64 to x86_64,
so aarch64 users still get these builds.

I added 4 known-failure tests for this (cmake + hugo, both aarch64 and x86_64).

### 2. solaris/illumos (1,745 dropped) — syncthing, terraform, hugo, caddy, etc.

**Still broken.** Your commit message for 3756bd8 says "keep solaris/illumos"
but the cache at 16:51 still shows `os: "sunos"` for solaris/illumos filenames.
My cache validation test confirms: terraform 393/393, syncthing 401+261 wrong.

The classifier recognizes `solaris`, `illumos`, AND `sunos` as distinct OS
values (triplet.js lines 270-272). Keep the original value from the filename:
- Filename has `solaris` → cache must say `os: "solaris"` (not `sunos`)
- Filename has `illumos` → cache must say `os: "illumos"` (not `sunos`)
- Filename has `sunos` → cache says `os: "sunos"` (node — already correct)

### 3. ARM variant mismatches (~630 dropped, was ~1,000)

Partially improved. Remaining:

| Cache value | Classifier detects | Count | Fix |
|---|---|---|---|
| `armv6` | `armel` | 412 | emit `armel` |
| `armv6` | `armv7` | 90 | emit `armv7` |
| `armhf` | `armv7` | 82 | emit `armv7` |
| `armv6` | `armhf` | 14 | emit `armhf` |

The cache arch must match what the classifier extracts from the filename.

### 4. android (355 dropped) — sass, lf, fzf, runzip, uuidv7

Cache has `os: "android"`. Classifier sees `android` in filename but internally
maps to `linux` first, then sees `android` in cache → `linux != android` → dropped.

**Fix**: Drop android entries from legacy export. Node side doesn't serve android.

### 5. Minor (~80 dropped)

- `sttr` .pkg (18): upstream bug, not fixable
- mips64r6/mips64r6el (24): GOER says fixed, but still showing
- x86_64_v2/v3/v4 (62): GOER says fixed, but still showing

### Test results (latest cache, 16:51)

- **19/19** installer-resolve (cmake/hugo universal2 now PASS!)
- **49/49** live-compare (5 known)
- **190/196** broad sweep (6 expected)
- **3/6** cache validation pass (solaris/illumos still failing)
- **6,149** PACKAGE FORMAT CHANGE warnings (= dropped entries, was 6,958)

## Previously resolved

- [x] Hugo macOS arm64 — resolves v0.157.0 .pkg
- [x] go armv6l — cache emits `armv6` (commit 9a391ad)
- [x] armel → armv6, winx64 → windows (commit aec6869)
- [x] Issues 1-3, 5: WATERFALL, .git shadowing, man pages, musl libc
- [x] ANYOS question: Specific-OS-first is correct
