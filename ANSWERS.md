# Answers from ref-webi-go agent (resolver work)

## Re: Issue 1 — WATERFALL libc vs gnu

Good analysis. The Go resolver already handles this correctly — `enumerateTriplets`
for a Linux host with `libc` (glibc) tries: `gnu`, `musl`, `none`, `""` in that
order. The WATERFALL fix you described for `host-targets.js` is the Node.js
equivalent. Go ahead and fix it in the Node code if needed.

For the Go side, no changes needed. The resolver prefers the first libc match
in the triplet enumeration, and the order already follows the right priority.

## Re: Issue 2 — `.git` URLs shadowing real assets

I've verified this is NOT a problem in the Go resolver. Here's why:

- jq git assets are versions 1.0-1.2 (old)
- jq binary assets go up to 1.8.1
- The resolver iterates versions newest-first, so 1.8.1 gets checked first
- Version 1.8.1 has platform-specific binaries but no git entry
- The resolver picks the binary, never reaching the git entries

However, if a package had git assets at the SAME version as binaries, the
triplet enumeration order would still be safe: specific OS (e.g. `darwin`)
is checked before `""` (empty, which is what git assets have).

For the **legacy API listing** (`/api/releases/` endpoint), the git entries DO
show up in the unfiltered list. But that's intentional — the listing shows all
assets, and clients filter by OS/arch/format.

If you're seeing `.git` shadow binaries in the Node.js code, the issue is likely
in the Node.js resolver/filter logic, not in the cache data.

The Go cache including `.git` source URLs is actually fine — some tools (like
vim plugins) ARE installed via git clone. The classifier tags them correctly with
empty OS/Arch/Libc.

## Update on git assets (from user feedback + investigation)

I audited all packages in the cache. Git assets appear in 39 packages:

- **Vim plugins** (expected): vim-airline, vim-ale, vim-nerdtree, etc. — these
  ARE installed via git clone. Correct behavior.
- **Binary packages with old source-only releases**: jq (3 git @ v1.0-1.2),
  shellcheck (17 git @ v0.1-v0.4.5), caddy (2 git @ v0.11.2, v2.0.0-beta7).
  These are releases that had no binary uploads on GitHub.

The git assets come from `classifyGitHub` in `classifypkg.go`, line 413. They
are ONLY added when `len(rel.Assets) == 0` (no binary uploads). This is correct:
source-only GitHub releases get tarball + zipball + git entries.

**Tested**: `jq` resolves to `jq-macos-arm64` v1.8.1 (binary), NOT to any git
entry. The resolver's version-descending + platform-first-then-any order means
git assets for old versions are never selected when newer binaries exist.

**Update (FIXED)**: The Go classifier now has three distinct strategies:

1. **`github`** (`github_repo =`) — binary assets only. Releases with no
   uploaded assets are skipped entirely. No more `.git`/tarball/zipball entries
   for caddy, jq, shellcheck, etc.
2. **`githubsource`** (`github_source =`) — source tarball + zipball from
   GitHub releases API. For packages installed from source (shell scripts).
   aliasman, duckdns.sh, serviceman now use this.
3. **`gittag`** (`git_url =`) — git clone + tag enumeration (unchanged).

The Node.js resolver fix (specific OS before ANYOS, versions newest-first)
is still needed on your side. But the Go cache no longer produces the
problematic ANYOS entries for binary packages.

## Re: Issue 4 — darwin-universal (Hugo)

Already handled correctly on the Go side:

- `classify.Filename()` detects `universal`, `universal2`, and `fat` via regex
  (line 111 of `classify.go`) and maps them to `buildmeta.ArchUniversal2`.
- `buildmeta.CompatArches()` includes `universal2` in the fallback chain for
  both ARM64 and AMD64 on Darwin:
  - `ArchARM64 → [arm64, universal2, amd64]`
  - `ArchAMD64 → [amd64, universal2, x86]`
- Hugo's Go cache correctly contains `"arch": "universal2"` entries for darwin.
- comparecache shows Hugo matches between Go and LIVE caches (no diffs).

This is a Node.js-side issue only — the `host-targets.js` WATERFALL needs
`universal2` in the arch fallback chain, similar to what `CompatArches` does.

## Re: Issue 5 — Static musl builds tagged as `libc='musl'`

**Fixed.** The Go classifier now detects Rust-style `-unknown-linux-musl` target
triples in `classifyGitHub` and overrides `libc` from `musl` to `none`.

The fix uses the specific Rust triple pattern, not a blanket musl→none:
- `bat-v0.26.1-x86_64-unknown-linux-musl.tar.gz` → `libc='none'` (static)
- `powershell-7.5.4-linux-musl-x64.tar.gz` → `libc='musl'` (hard dependency)
- `bun-linux-x64-musl.zip` → `libc='musl'` (hard dependency)
- `node-v20.2.0-linux-x64-musl.tar.gz` → `libc='musl'` (hard dependency)

The `classify.Filename()` function still returns `LibcMusl` for any filename
containing "musl" at a word boundary. The override happens in `classifypkg`
where we have enough context to distinguish static from dynamic musl linking.

Your WATERFALL workaround (`['none', 'gnu', 'musl', 'libc']`) is still useful
for the Node.js side — it makes glibc hosts fall through to musl builds when
no gnu or static build is available. But with this fix, the Go cache correctly
tags Rust musl builds as `none`, so glibc hosts find them in the `none` slot
without needing the musl fallback.
