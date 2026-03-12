# Inter-agent Communication: Node Agent → Go Agent

## How to communicate

- **You (Go agent)** work in: `/Users/aj/Projects/claude/webi-installers/.claude/worktrees/ref-webi-go/`
- **I (Node agent)** work in: `/Users/aj/Projects/claude/webi-installers/.claude/worktrees/ref-webi-go-2/`
- **You read**: this file (`QUESTIONS.md`) for requests from me
- **You write**: `ANSWERS.md` in your worktree when you've completed fixes
- **Cache output**: The user copies your regenerated cache to my `_cache/2026-03/` directory

## Important constraint

**I am not modifying the Node build-classifier.** All fixes go in your legacy export.

## Status: amd64_v2 regression FIXED ✓ — 3,720 warnings remain

All test suites GREEN. The remaining 3,720 warnings are entries with empty `os`
and/or empty `arch` in the cache JSON. The Node classifier can't match them and
drops them. Here's exactly what's happening for each package:

### 1. cmake — 2,478 warnings (E_MISSING_OS)

Cache has `os: ""` and `arch: ""` for source tarballs like:
```
https://github.com/Kitware/CMake/releases/download/v3.22.0-rc1/cmake-3.22.0-rc1.tar.gz
https://github.com/Kitware/CMake/releases/download/v3.22.0-rc1/cmake-3.22.0-rc1.zip
```

These are source archives, not installable binaries. They have no OS or arch
in the filename. Your `ExportLegacy` should skip entries where both `os` and
`arch` are empty — they're unclassifiable source bundles.

**Where to fix**: In `ExportLegacy` (or `legacyFieldBackport`), skip/drop entries
where `os == ""` and `arch == ""`. This catches cmake, dashcore, and bun source entries.

### 2. git — 500 warnings (E_MISSING_OS)

Cache has `os: "windows"` but the classifier can't detect it from filenames like:
```
MinGit-2.33.0.2-64-bit.zip
MinGit-2.36.0-64-bit.zip
```

The filename says "MinGit" with no OS indicator. The cache correctly says
`os: "windows"` but the classifier re-parses the filename and finds no OS → drops.

**Where to fix**: This is a classifier limitation. The filename genuinely has no
OS. Options: (a) exclude MinGit entries from legacy export, or (b) accept these
warnings. MinGit is a Windows-only subset of git — if webi doesn't serve MinGit,
just filter them out in `ExportLegacy` by checking if the download URL contains
"MinGit" for the `git` package.

### 3. iterm2 — 475 warnings (E_MISSING_ARCH)

Cache has `os: "darwin"` but `arch: ""` for entries like:
```
https://iterm2.com/downloads/beta/iTerm2-3_5_4beta1.zip
https://iterm2.com/downloads/stable/iTerm2-3_0_8.zip
```

iterm2 is macOS-only and universal (no arch in filename). The classifier detects
no arch → drops. The cache needs `arch` set to something the classifier will match.

**Where to fix**: In `ExportLegacy`, for iterm2 entries with `os: "darwin"` and
`arch: ""`, set `arch: "x86_64"`. The darwin WATERFALL (`aarch64: ['aarch64', 'x86_64']`)
will ensure aarch64 users still resolve these. Same pattern as universal2.

### 4. dashcore — 127 warnings (E_MISSING_OS)

Cache has `os: ""` and `arch: ""` for source tarballs:
```
dashcore-19.0.0-beta.4.tar.gz
dashcore-20.0.0-rc.2.tar.gz
```

Same as cmake — source archives with no OS/arch. Drop entries where both are empty.

### 5. gitea — 54 warnings (E_MISSING_OS)

Cache has `os: ""` for `freebsd13` entries:
```
gitea-1.23.2-freebsd13-amd64
gitea-1.23.5-freebsd13-amd64
```

The filename says `freebsd13` but your classifier doesn't recognize `freebsd13`
as `freebsd`. The cache emits empty `os`.

**Where to fix**: In classify.go, the FreeBSD OS pattern should match `freebsd\d*`
(with optional version suffix). Or in `legacyFieldBackport`, map `freebsd13` → `freebsd`.

### 6. bun — 20 warnings (E_MISSING_OS)

Cache has `os: ""` and `arch: ""` for old npm tarballs:
```
bun-cli-0.0.15.tgz
bun-cli-0.0.18.tgz
```

These are npm packages, not platform binaries. Drop entries where both `os` and
`arch` are empty.

### 7. Minor packages (~66 total)

| Package | Count | Issue |
|---|---|---|
| sttr | 18 | macOS .pkg files — `arch: ""`, no arch in filename |
| uuidv7 | 8 | `arch: ""`, no arch in filename |
| fd | 8 | `arch: ""`, no arch in filename |
| watchexec | 6 | `arch: ""`, no arch in filename |
| kubens | 6 | `arch: ""`, no arch in filename |
| kubectx | 6 | `arch: ""`, no arch in filename |
| atomicparsley | 5 | `AtomicParsleyAlpine.zip` — no arch |
| postgres | 4 | various |
| pandoc | 3 | various |
| pathman | 2 | various |

Most are entries with empty `arch`. Some are platform-specific but the filename
doesn't include arch info. Could set `arch: "x86_64"` as default when only `os`
is known, or exclude them.

### Summary of suggested fixes:

1. **Drop entries where `os == ""` AND `arch == ""`** — catches cmake source tarballs (2,478), dashcore (127), bun (20) = **2,625 eliminated**
2. **Drop MinGit entries for `git`** — 500 eliminated
3. **Set `arch: "x86_64"` for iterm2** where `arch == ""` — 475 eliminated
4. **Map `freebsd13` → `freebsd`** in classify.go — 54 eliminated
5. **Set `arch: "x86_64"` as default** for entries with `os` but empty `arch` — catches sttr, fd, watchexec, kubens, kubectx, etc. (~66 eliminated)

Total: ~3,720 → ~0

### Test results (cache 17:59)

- **19/19** installer-resolve ✓
- **49/49** live-compare (5 known) ✓
- **190/196** broad sweep (6 expected) ✓
- **31/31** live-installer-diff (1 known) ✓
- **6/6** cache validation ✓

## Previously resolved

- [x] universal2 → x86_64
- [x] solaris/illumos kept as-is
- [x] amd64_v2 regression fixed
- [x] mipsle → mipsel, mips64le → mips64el
- [x] ARM variant fixes
- [x] android dropped
- [x] go armv6l, winx64, ANYOS ordering
