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

## Progress: 3,746 warnings remain (was 6,149)

### FIXED: universal2 → x86_64 ✓
### FIXED: solaris/illumos → kept as-is ✓

Both are confirmed working! Cache validation passes 6/6. Great work!

- cmake resolves to v4.2.3 (was v3.19.1), universal2 entries all have arch=x86_64
- hugo resolves to v0.157.0 (was v0.101.0)
- terraform solaris entries correctly have os=solaris (393 entries)
- syncthing solaris entries correctly have os=solaris (401 entries)
- syncthing illumos entries correctly have os=illumos (261 entries)

## Still needs fixing:

### 1. cmake source tarballs (2,478 E_MISSING_OS)

These are source archives with no OS in filename, e.g.:
`cmake-3.22.0-rc1.tar.gz`, `cmake-3.22.0-rc1.zip`

The classifier can't detect an OS → drops them. These should be excluded
from the legacy export entirely — they're source bundles, not installable binaries.

### 2. git entries (500 E_MISSING_OS)

Same issue — source tarballs or entries without OS in filename.

### 3. iterm2 entries (475 E_MISSING_OS)

No OS in filename. These are macOS-only .zip files but the filename doesn't say
darwin/macos. Consider excluding from legacy export or setting os=darwin.

### 4. dashcore entries (127 E_MISSING_OS)

Same pattern — no OS indicator in filename.

### 5. ARM variant mismatches (~100+ dropped)

Remaining mismatches in smaller packages (gitea 54, jq 24, bun 20, sttr 18, etc.):

| Cache value | Classifier detects | Fix |
|---|---|---|
| `armv6` | `armel` | emit `armel` |
| `armv6` | `armv7` | emit `armv7` |
| `armhf` | `armv7` | emit `armv7` |

### 6. Minor

- `sttr` .pkg (18): macOS .pkg files, upstream quirk
- Various small packages with arch mismatches

### Test results (latest cache, 17:19)

- **19/19** installer-resolve ✓
- **49/49** live-compare (5 known) ✓
- **190/196** broad sweep (6 expected) ✓
- **31/31** live-installer-diff (1 known) ✓
- **6/6** cache validation ✓
- **3,746** PACKAGE FORMAT CHANGE warnings (was 6,149, down 39%)

The biggest remaining items are source tarballs being included (cmake 2,478 +
git 500 + iterm2 475 + dashcore 127 = 3,580). Excluding source-only entries
would bring warnings down to ~166.

## Previously resolved

- [x] universal2 → x86_64 (cmake, hugo universal entries)
- [x] solaris/illumos kept as-is (not translated to sunos)
- [x] Hugo macOS arm64 — resolves v0.157.0 .pkg
- [x] go armv6l — cache emits `armv6`
- [x] armel → armv6, winx64 → windows
- [x] Issues 1-3, 5: WATERFALL, .git shadowing, man pages, musl libc
- [x] ANYOS question: Specific-OS-first is correct
- [x] android entries dropped from legacy export
