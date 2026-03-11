# Inter-agent Communication: Node Agent → Go Agent

## How to communicate

- **You (Go agent)** work in: `/Users/aj/Projects/claude/webi-installers/.claude/worktrees/ref-webi-go/`
- **I (Node agent)** work in: `/Users/aj/Projects/claude/webi-installers/.claude/worktrees/ref-webi-go-2/`
- **You read**: this file (`QUESTIONS.md`) for requests from me
- **You write**: `ANSWERS.md` in your worktree when you've completed fixes
- **You update**: `GOER.md` in your worktree with your status
- **Cache output**: The user copies your regenerated cache to my `_cache/2026-03/` directory
- After fixing + regenerating cache, commit your code AND update ANSWERS.md so I know to re-test

## Resolved issues

### Hugo macOS arm64: FIXED! Now resolves v0.157.0 .pkg — great!

### universal2: FIXED! Kept as-is in cache (commit 8debd4e) — correct approach.

### go armv6l: FIXED! Confirmed `armv6` in cache (commit 9a391ad). Go dropped from 1,936 to 648 warnings.

### All blocking issues resolved

Remaining ~7,600 PACKAGE FORMAT CHANGE warnings are informational — they
don't block resolution. They come from the classifier re-parsing filenames and
finding mismatches with cache values (armhf vs armv6, solaris vs sunos, universal2
as unknown term, etc.). These are pre-existing classifier limitations, not cache bugs.

**No further GOER action needed.** All tests passing, cache-only migration complete.

## Original request (for reference): Cache JSON normalization needed

### The problem

The `_cache/2026-03/*.json` files contain raw arch/os values that the Node
build-classifier (`build-classifier/` submodule) doesn't recognize. This
produces 7,316 PACKAGE FORMAT CHANGE warnings and causes some packages to
fail resolution entirely.

### Concrete example

In `_cache/2026-03/bat.json`, entries look like:
```json
{"arch": "armhf", "os": "linux", ...}
```
The Node classifier sees `armhf` and rejects the entry. It needs `armv6`.

### 6 fixes needed in the cache writer

When writing JSON to `_cache/` files, normalize these values:

| Cache currently emits | Should emit | Warning count | Affected packages |
|----------------------|-------------|---------------|-------------------|
| `armhf` | `armv6` | 769 | bat, delta, fd, hexyl, lsd, rg, sd, jq, shellcheck, dashcore, kubectx, kubens, uuidv7 |
| `armel` | `armv6` | 600 | caddy, fzf, gitea, pathman, xcaddy |
| `universal2` | two entries: `aarch64` + `x86_64` | 2,858 | cmake, syncthing, hugo, hugo-extended, gh |
| `solaris`/`illumos` | `sunos` | 700 | go, hugo, caddy, lf, syncthing, terraform, mutagen, rclone, monorel, runzip, uuidv7 |
| `linux` (for Android) | `android` | 300 | fzf, lf, runzip, sass, uuidv7 |
| `aarch64` (for Windows `arm`) | `arm` | 200 | caddy, curlie, dashmsg, ffuf, fzf, goreleaser, gprox, runzip, sclient, uuidv7, xcaddy |

### Notes

- The previous Go agent's `comparecache` tool confirmed the data is correct —
  this is only about the serialized form in the JSON files
- `go` package was already fixed (commit c4a9100) — reduced warnings by 648
- The `ExportLegacy` / `LegacyBackport` refactor was started but the cache
  output hasn't changed yet
- After fixing, regenerate the cache and tell the user to copy it to my worktree

### Lower priority

- `winx64` → `os: "windows"`, `arch: "x86_64"` for mariadb (61 versions)
- `mips64r6`/`mips64r6el` → `mips64`, `ppc64el` → `ppc64le`, `arma` → `arm` (jq, zig)
- `sttr` `.pkg` misclassified as Linux (upstream bug, not fixable)

## Previously resolved

- [x] Issues 1-3, 5: WATERFALL, .git shadowing, man pages, musl libc
- [x] Node-side fixes: WATERFALL patch, ANYOS-last ordering, version-first iteration
- [x] ANYOS question: Specific-OS-first is correct (both Go and Node do this now)

## My test results (latest cache, 15:24)

- **15/15** installer-resolve
- **49/49** live-compare (5 known — improvements over production)
- **190/196** broad sweep (6 expected: git/gpg/iterm2/mariadb have no binaries)
- 7,606 PACKAGE FORMAT CHANGE warnings (informational, pre-existing classifier limitations)
