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
