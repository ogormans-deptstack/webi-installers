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

## 6,958 warnings = 6,958 dropped entries. Fixes needed:

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

### 2. solaris/illumos (1,497 dropped) — go, syncthing, terraform, hugo, caddy, etc.

**The sunos translation is the bug.** The classifier recognizes `solaris`,
`illumos`, AND `sunos` as distinct OS values. Node.js releases use `sunos` in
filenames and have zero warnings.

The problem: other packages use `solaris`/`illumos` in filenames, but your
legacy export translates them to `sunos`. Filename says `solaris`, cache says
`sunos` → mismatch → dropped.

**Fix**: Keep the original OS values from the filenames. Don't translate to sunos.
- Filename has `solaris` → cache should say `os: "solaris"`
- Filename has `illumos` → cache should say `os: "illumos"`
- Filename has `sunos` → cache should say `os: "sunos"` (node — already correct)

### 3. ARM variant mismatches (~1,000 dropped) — bat, delta, fd, caddy, dashcore

The cache arch must match what the classifier extracts from the filename:

| Filename pattern | Classifier detects | Cache should say | Count |
|---|---|---|---|
| `arm-unknown-linux-gnueabihf` | `armhf` | `armhf` | 443 |
| `linux_armel` | `armel` | `armel` | 424 |
| `linux_armv5` | `armel` | `armel` | 523 |
| various armv7a | `armv7a` | `armv7a` | 14 |

**Fix**: In ExportLegacy, don't normalize ARM arch names. Keep what the
classifier will detect from the filename: `armhf`, `armel`, `armv7a`.

### 4. android (355 dropped) — sass, lf, fzf, runzip, uuidv7

Cache has `os: "android"`. Classifier sees `android` in filename but internally
maps to `linux` first, then sees `android` in cache → `linux != android` → dropped.

**Fix**: Drop android entries from legacy export. Node side doesn't serve android.

### 5. Minor (44 dropped)

- `sttr` .pkg (18): upstream bug, not fixable
- mips64r6/mips64r6el → `mips64` (24): translate in legacy export
- x86_64_v2 → `x86_64` (2): translate in legacy export

### Test results (latest cache, 16:34)

- **15/15** installer-resolve (+ 4 known: cmake/hugo universal2)
- **49/49** live-compare (5 known)
- **190/196** broad sweep (6 expected)
- **6,958** PACKAGE FORMAT CHANGE warnings (= dropped entries)

## Previously resolved

- [x] Hugo macOS arm64 — resolves v0.157.0 .pkg
- [x] go armv6l — cache emits `armv6` (commit 9a391ad)
- [x] armel → armv6, winx64 → windows (commit aec6869)
- [x] Issues 1-3, 5: WATERFALL, .git shadowing, man pages, musl libc
- [x] ANYOS question: Specific-OS-first is correct
