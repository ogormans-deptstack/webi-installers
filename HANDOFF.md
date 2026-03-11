# Handoff: Node.js Server → Cache-Only Mode

## Context

The Go `webicached` daemon now generates the legacy `_cache/YYYY-MM/{pkg}.json`
files that the Node.js server reads. The next step is to make the Node.js server
read **only** from these cache files and never fetch from upstream APIs itself.

This decouples the two: Go handles all upstream fetching and classification,
Node.js just serves what's in the cache.

## Current Node.js Data Flow

```
Request → builds.js → builds-cacher.js
                         ├── Read _cache/YYYY-MM/{pkg}.json
                         ├── If missing: require({pkg}/releases.js).latest()
                         │   → fetch from GitHub/etc → write cache → return
                         ├── transformAndUpdate() — classify, sort, organize
                         └── Background: if stale (>15min), re-fetch
```

Key files:
- `_webi/builds.js` — entry point, creates BuildsCacher, starts background refresh
- `_webi/builds-cacher.js` — the big one: cache read, fetch, classify, resolve
- `_webi/serve-installer.js` — HTTP handler, calls `Builds.getPackage()`
- `_webi/transform-releases.js` — legacy release API endpoint
- `{pkg}/releases.js` — per-package fetcher (GitHub, git-tag, etc.)

## What Needs to Change

### 1. `_webi/builds.js` — Remove upstream fetching

Current (`builds.js:14-18`):
```js
let bc = BuildsCacher.create({ caches: CACHE_DIR, installers: INSTALLERS_DIR });
bc.freshenRandomPackage(600 * 1000);  // ← DELETE: background refresh timer
```

`Builds.init()` (line 20-35) fetches ALL packages on startup via `bc.getPackages()`.
This triggers `getLatestBuilds()` for any missing cache. Change it to only load
from existing cache files — if a package has no cache file, skip it (don't fetch).

### 2. `_webi/builds-cacher.js` — Cache-only reads

**`getPackages()` (line 320-413):**
Currently falls through to `getLatestBuilds()` when cache file is missing (line
380-382). Remove that fallback — if no cache file exists, return empty/error.

Also remove the background stale check (lines 392-410) that re-fetches when
data is older than 15 minutes.

**`getLatestBuilds()` (line 130-178):**
This function `require()`s `{pkg}/releases.js` and calls `.latest()` to fetch
from upstream. It should be dead code after the changes above. Can be removed
or stubbed.

**`freshenRandomPackage()` (line 418-448):**
The background timer that picks a random package and re-fetches it. Remove entirely.

### 3. Cache file format

The Go cache writes the same JSON format the Node code expects:

```json
{
  "releases": [
    {
      "name": "bat-v0.26.1-x86_64-unknown-linux-musl.tar.gz",
      "version": "0.26.1",
      "lts": false,
      "channel": "stable",
      "date": "2025-01-01",
      "os": "linux",
      "arch": "x86_64",
      "libc": "musl",
      "ext": ".tar.gz",
      "download": "https://github.com/..."
    }
  ]
}
```

Timestamp file: `_cache/YYYY-MM/{pkg}.updated.txt` — float seconds since epoch.

### 4. What should NOT change

- `transformAndUpdate()` — still needed to classify, sort, and organize by triplet
- `selectPackage()` — still needed for format preference
- `findMatchingPackages()` — still needed for version/triplet resolution
- `enumerateLatestVersions()` — still needed
- `_enumerateTriplets()` — still needed for arch fallback
- All the serve/render code (`serve-installer.js`, `transform-releases.js`, etc.)

### 5. `{pkg}/releases.js` files

These become dead code once the Node server stops fetching. Don't delete them
yet — they're still useful as documentation of where each package's releases
come from. But they should no longer be `require()`d at runtime.

## Go Cache State

The Go `webicached` generates cache for 101 packages (all non-alias packages).
Aliases are handled via symlinks in the cache directory (e.g., `ripgrep.json` →
`rg.json`).

To regenerate the cache from existing raw data (no API calls):
```sh
cd .claude/worktrees/ref-webi-go
./webicached -once -no-fetch
```

To fetch fresh + regenerate:
```sh
./webicached -once
```

The raw upstream data lives in `_cache/raw/{pkg}/a/` — one JSON file per release
tag. This is the canonical source of truth.

## Testing

After changes, verify the Node server still works:
1. Start the server (however it's normally started)
2. `curl -sS localhost:PORT/api/releases/bat.json | jq '.releases | length'`
3. `curl -sS localhost:PORT/api/releases/bat.tab | head -5`
4. `curl -A 'curl/8.0' localhost:PORT/bat@stable` — should return bash script

## Files Summary

| File | Action |
|------|--------|
| `_webi/builds.js` | Remove background refresh, make init() cache-only |
| `_webi/builds-cacher.js` | Remove `getLatestBuilds()` fallback and stale re-fetch |
| `{pkg}/releases.js` | Dead code — don't require() at runtime |
| `_webi/serve-installer.js` | No changes needed |
| `_webi/transform-releases.js` | No changes needed |

## Branch Info

- Go rewrite work: branch `ref-webi-go`, worktree at
  `.claude/worktrees/ref-webi-go`
- Node.js changes: should happen on `main` branch in the main repo
  (`/Users/aj/Projects/webi-installers`)
