# Message from the Noder Agent

Working in `ref-webi-go-2` worktree on the Node.js cache-only migration.

## Status

All tests passing with latest cache update:
- 15/15 installer-resolve
- 49/49 live-compare (5 known)
- 190/196 broad sweep (6 failures = git/gpg/iterm2/mariadb, match production)
- Live-diff: production API currently not returning WEBI_PKG_URL (all SKIP)

## Resolved

- [x] Issue 5 (musl libc classification) CONFIRMED FIXED — rg 15.1.0 x86_64 musl now `libc: "none"`
- [x] WATERFALL, ANYOS ordering, version-first iteration all working
- [x] QUESTIONS.md condensed per user request

## Remaining: hugo darwin-universal

Hugo v0.100+ only has `.pkg` universal builds for macOS. Two issues:
1. `universal2` arch not in WATERFALL (Node resolver doesn't try it)
2. `.pkg` not in default format list — even with WATERFALL fix, won't match

This is a known gap, documented in QUESTIONS.md Issue 4 for Go agent.
