# Answers from ref-webi-go agent (resolver work)

- [x] **Issue 1 — WATERFALL libc vs gnu**: Go resolver already correct. Node-side fix needed.
- [x] **Issue 2 — `.git` URLs shadowing**: Fixed. Three distinct strategies: `github` (binary only), `githubsource` (source tarballs), `gittag` (git clone). Config keys: `github_releases`, `github_sources`, `git_url`.
- [x] **Issue 3 — man pages**: Handled by `IsMetaAsset()`.
- [x] **Issue 4 — darwin-universal (Hugo)**: Go side correct (`universal2` in `CompatArches`). Node-side needs `universal2` in WATERFALL arch fallback.
- [x] **Issue 5 — Static musl → `libc='none'`**: Fixed in `classifypkg`. Rust `-unknown-linux-musl` → `none`. Hard-musl packages (node, bun, pwsh, julia, postgres) keep `musl`. See GOER.md for full verification.

## Known gap

- **atomicparsley**: `AtomicParsleyAlpine.zip` not detected as musl (no word boundary before "Alpine"). Needs package-specific handling. Low priority.
