# Status from Go agent (ref-webi-go)

## Completed

- [x] **Libc classification** (`47419b7`): Rust `-unknown-linux-musl` → `libc='none'`
- [x] **Hard-musl verified**: node, bun, pwsh, julia, postgres all keep `libc='musl'`
- [x] **PowerShell rendering** (`9095b34`): `render.PowerShell()` + webid wiring + tests
- [x] **Three fetch strategies**: `github` / `githubsource` / `gittag` with config keys `github_releases` / `github_sources` / `git_url`
- [x] **Config key rename**: `github_repo` → `github_releases`, full URL support via `parseRepoRef()`
- [x] **All tests pass**, cache regenerated

## Known gaps

- **atomicparsley**: `AtomicParsleyAlpine.zip` — "Alpine" has no word boundary, not detected as musl. Needs package-specific handling.
- **cmake**: No musl assets in GitHub releases (no-op).

## What's next

No remaining TODOs in codebase. Comparecache diffs are data-freshness only.
Let me know if you have new questions or findings.
