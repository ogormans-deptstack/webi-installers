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

## REGRESSION: amd64_v2 regex matches version numbers (257 new warnings)

Your `amd64[_-]?v2/v3/v4` regex fix in `4f09649` is matching version numbers
in filenames. Example:

```
syncthing-freebsd-amd64-v2.0.5.tar.gz
```

The classifier detects `x86_64` from `amd64`, but the cache now says `x86_64_v2`
because the regex matched `amd64-v2` (where `v2` is the start of the version `v2.0.5`).

**This affects 257 syncthing entries.** The regex needs to be anchored so it only
matches `amd64_v2` / `amd64-v2` when NOT followed by a dot/digit (i.e., not a
version number). Or better: only match when the `v2`/`v3`/`v4` is at a word
boundary or end of the arch segment.

## Progress: 3,977 warnings (was 3,746, regression from syncthing)

### FIXED ✓
- universal2 → x86_64 (cmake, hugo)
- solaris/illumos kept as-is
- mipsle → mipsel, mips64le → mips64el
- ARM variant mismatches (most)
- android dropped from legacy export

### Still needs fixing:

#### 1. syncthing amd64_v2 regression (257 new — priority!)
See above. The `v2` in `v2.0.5` is being captured as an arch micro-version.

#### 2. cmake source tarballs (2,478 E_MISSING_OS)
Source archives with no OS: `cmake-3.22.0-rc1.tar.gz`. Should be excluded.

#### 3. git entries (500 E_MISSING_OS)
Source tarballs / no-OS filenames.

#### 4. iterm2 entries (475 E_MISSING_OS)
macOS-only .zip files but filename has no darwin/macos.

#### 5. dashcore entries (127 E_MISSING_OS)
No OS indicator in filename.

#### 6. Minor (~62 remaining)
gitea 54, jq 24, bun 20, sttr 18, etc.

### Test results (cache 17:27)

- **19/19** installer-resolve ✓
- **49/49** live-compare (5 known) ✓
- **190/196** broad sweep (6 expected) ✓
- **31/31** live-installer-diff (1 known) ✓
- **6/6** cache validation ✓
- **3,977** PACKAGE FORMAT CHANGE warnings (was 3,746 — regression from syncthing)

## Previously resolved

- [x] universal2 → x86_64 (cmake, hugo universal entries)
- [x] solaris/illumos kept as-is (not translated to sunos)
- [x] mipsle → mipsel, mips64le → mips64el
- [x] ARM variant fixes (armhf→armv7, armel, arm-5, arm-7, armv6hf)
- [x] Hugo macOS arm64 — resolves v0.157.0 .pkg
- [x] go armv6l — cache emits `armv6`
- [x] android entries dropped from legacy export
- [x] ANYOS question: Specific-OS-first is correct
