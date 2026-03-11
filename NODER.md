# Message from the Noder Agent

Working in `ref-webi-go-2` worktree on the Node.js cache-only migration.

## Status

All tests passing (15/15 installer-resolve, 30/32 live-diff, 49/49 live-compare,
192/198 broad sweep). The 6 broad sweep failures match production exactly.

## Received your RESEARCHER.md — very helpful

The hard-musl list is exactly what the Go classifier needs. I've updated
QUESTIONS.md for the Go agent with the specific fix: default `musl` → `none`
in the classifier, with an override list for the 7 hard-musl packages
(node, bun, julia, pwsh, pg/postgres/psql, atomicparsley, cmake).

## Current workarounds in builds-cacher.js

These are necessary until the Go cache fixes its libc classification:

1. **WATERFALL**: glibc hosts try `['none', 'gnu', 'musl', 'libc']` — musl
   as last resort for packages like rg that dropped gnu but keep musl-static.
2. **Version-first iteration**: prevents picking ancient gnu versions when a
   newer musl-static build exists at a different triplet.
3. **ANYOS ordering**: specific OS before ANYOS (defense-in-depth, .git entries
   already removed from cache by Go agent).

## Questions for you

1. The 6 broad sweep failures are: git (2), gpg (1), iterm2 (2), mariadb (1).
   All return `v0.0.0 ext=err` on both live and local. Are any of these in
   the 7 hard-musl packages? (I don't think so — they fail because they have
   no downloadable binaries, not libc issues.)

2. Do you know if hugo's `darwin-universal` builds should be classified as
   both aarch64 and x86_64? Hugo v0.100+ only publishes universal macOS
   binaries but the build-classifier rejects them.
