# Questions/Notes from ref-webi-go-2 agent (Node cache-only work)

## Issue 1: WATERFALL libc vs gnu (build-classifier submodule)

The Go cache correctly uses `libc='gnu'` for glibc-linked Linux binaries (bat,
rg, node — any Rust or C project with explicit gnu triplets in filenames). This
is more precise than the old `normalize.js` which mapped everything to
`libc='none'`.

However, the build-classifier `host-targets.js` WATERFALL maps:
```
libc: ['none', 'libc']
```

A host reporting `libc` (meaning "I have standard glibc") never tries `gnu`
triplets. So `curl webi.sh/bat` on Linux glibc fails to find a match.

**Fix needed**: Update WATERFALL in `host-targets.js`:
```
libc: ['none', 'gnu', 'libc']
```

This should be safe — a `libc` host (standard glibc) can run `gnu`-linked
binaries. The ordering puts `none` first (static/no-libc-dep preferred), then
`gnu` (glibc-linked), then `libc` as final fallback.

Note: `musl` hosts already work correctly — the WATERFALL has
`musl: ['none', 'musl']` and musl-specific builds are properly classified.

## Issue 2: Go cache includes `.git` source URLs as releases

**This is a regression.** Production jq and caddy work correctly today — the old
`releases.js` fetchers never included `.git` source URLs. The Go cache adds
them, which creates `ANYOS-ANYARCH-none` triplets. Since the triplet enumeration
tries ANYOS/ANYARCH *before* specific OS/arch, the `.git` entry matches first
and shadows all platform-specific binaries.

Verified against production:
- `curl 'https://webinstall.dev/api/releases/jq@stable.json?os=macos&arch=arm64&limit=1'`
  → returns `jq-macos-arm64` (exe, correct)
- Our code → returns `jq.git` (ANYOS, wrong)

**Fix**: The Go cache should not include `.git` source repo URLs as release
assets. They aren't installable binaries.

Affected packages (at minimum): jq, caddy — any package whose GitHub releases
page includes the auto-generated source links that GitHub adds to every release.
