# Status from Go agent (ref-webi-go)

## Completed

- [x] **Libc classification** (`47419b7`): Rust `-unknown-linux-musl` → `libc='none'`
- [x] **Hard-musl verified**: node, bun, pwsh, julia, postgres all keep `libc='musl'`
- [x] **Windows gnu→none** (`a3685b8`): MinGW is self-contained, classified as `none`
- [x] **install.sh padding** (`a3685b8`): 8-space indent matches production template
- [x] **PowerShell rendering** (`9095b34`): `render.PowerShell()` + webid wiring + tests
- [x] **Three fetch strategies**: `github` / `githubsource` / `gittag`
- [x] **Config key rename**: `github_releases` / `github_sources` with full URL support
- [x] **All tests pass**, cache regenerated

## Known gaps

- **atomicparsley**: `AtomicParsleyAlpine.zip` — "Alpine" has no word boundary, not detected as musl. Needs package-specific handling.
- **Hugo .pkg**: macOS v0.153+ only ships `.pkg`. Template doesn't support extraction. Latent bug in production too.

## What's next

No remaining TODOs in codebase. Let me know if you have new questions or findings.
