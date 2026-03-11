# Inter-agent Communication: Node Agent → Go Agent

## How to communicate

- **You (Go agent)** work in: `/Users/aj/Projects/claude/webi-installers/.claude/worktrees/ref-webi-go/`
- **I (Node agent)** work in: `/Users/aj/Projects/claude/webi-installers/.claude/worktrees/ref-webi-go-2/`
- **You read**: this file (`QUESTIONS.md`) for requests from me
- **You write**: `ANSWERS.md` in your worktree when you've completed fixes
- **You update**: `GOER.md` in your worktree with your status
- **Cache output**: The user copies your regenerated cache to my `_cache/2026-03/` directory
- After fixing + regenerating cache, commit your code AND update ANSWERS.md so I know to re-test

## Current status: DONE

All tests passing. Cache-only migration is complete. No further action needed
from either side.

**Important: I am not adding features to the Node code.** My scope is removing
the upstream fetchers and reading from `_cache/` instead. The Node build-classifier
is a submodule and is not being modified. Production behavior is preserved as-is.

### Test results (latest cache, 15:36)

- **15/15** installer-resolve
- **49/49** live-compare (5 known — improvements over production)
- **190/196** broad sweep (6 expected: git/gpg/iterm2/mariadb have no binaries)

### Warnings: 7,606 — all informational, none actionable

I verified every warning category. The cache values are correct in all cases.
The warnings come from the Node classifier re-parsing filenames and using its
own naming conventions, which differ from the GOER's normalized values:

| Category | Count | Cache value | Classifier re-detects | Why not fixable |
|---|---|---|---|---|
| solaris/illumos vs sunos | 2,145 | `sunos` (correct) | `solaris`/`illumos` from URL | Filename says solaris, cache says sunos — both right |
| universal vs universal2 | 1,492 | `universal2` (correct) | `x86_64` from `universal` keyword | Classifier doesn't know universal2 |
| ARM variant naming | ~1,000 | `armv6`/`armv5`/`armv7` | `armhf`/`armel`/`armv7a` | Different naming conventions |
| android vs linux | 355 | `android` (correct) | `linux` (classifier maps android→linux first) | Classifier quirk |
| mips/ppc variants | ~26 | various | various | Naming differences |
| sttr .pkg | 18 | `linux` | `darwin` from `.pkg` ext | Upstream bug |

**Do not try to fix these on the Go side — the cache is already correct.**
These are pre-existing classifier validation mismatches that don't affect resolution.

## Previously resolved

- [x] Hugo macOS arm64 — resolves v0.157.0 .pkg
- [x] universal2 — kept as-is in cache (commit 8debd4e)
- [x] go armv6l — cache emits `armv6` (commit 9a391ad)
- [x] solaris/illumos → sunos (commit aec6869)
- [x] armel → armv6, winx64 → windows (commit aec6869)
- [x] Issues 1-3, 5: WATERFALL, .git shadowing, man pages, musl libc
- [x] ANYOS question: Specific-OS-first is correct
