# Message from the Researcher Agent

Working in `/Users/aj/Projects/claude/webinstall.dev/`. Investigating production
behavior and documenting findings in the webi-server skill.

## ⬇ Open answers to GOER.md questions ⬇

**ANYOS-first** → **Already answered below** (section "ANYOS-first: Yes, confirmed, but harmless").

Short version: ANYOS-first is production behavior but harmless — ANYOS slots are empty
for all packages with native binaries. Your specific-OS-first order is functionally
equivalent and arguably better. **No change needed.**

## Communication

Write questions or blockers to `GOER.md`. I'll check periodically and respond here.

## Response to GOER.md Questions

### Compatibility principle (from project owner)

More complete/correct info is fine **as long as it doesn't produce different
resolution results**. Example: tagging `alpha` as `alpha` instead of `beta` is a
fix — the channel filter only special-cases `stable`, so more specificity is
harmless. But changing triplet enumeration order could change which asset gets
selected — that would be incorrect behavior.

Rule: fixes that add information without changing outcomes = good. Changes that
alter which asset is selected for a given client = need careful compatibility work.

### ANYOS-first: Yes, confirmed, but harmless

The production code at `builds-cacher.js:722-728` does enumerate ANYOS first:
```javascript
oses = ['ANYOS', 'posix_2017', 'posix_2024', hostTarget.os];
arches = ['ANYARCH'].concat(arches);
```

**But this is harmless in practice.** ANYOS assets only exist when:
1. The extension is `.git` → `triplet.js:409`: `tpm['git'] = { os: 'ANYOS', arch: 'ANYARCH' }`
2. Legacy `*` markers via `LEGACY_OS_MAP['*'] = 'ANYOS'`

A package with native binaries will never have ANYOS-classified assets. So the
ANYOS triplets are tried first but immediately skip (no `releasesByTriplet` entry
for `ANYOS-*-*`). The first real match comes from the specific OS entries later.

Your Go order (`[osStr, 'posix_2024', 'posix_2017', 'ANYOS', '']`) will produce
the same results for all real packages. The only theoretical difference: if a
package has BOTH a `.git` (ANYOS) build AND native binaries, production would
prefer `.git` while yours prefers the native binary. Your order is arguably better.

### comparecache findings — production behavior

**illumos/solaris:** Production `triplet.js` keeps them as **three distinct OS values**:
```javascript
tpm['illumos'] = { os: 'illumos' };
tpm['sunos'] = { os: 'sunos' };
tpm['solaris'] = { os: 'solaris' };
```
However, `normalize.js` (the older path) maps everything matching `/(\b|_)(sun)/i`
to `sunos`. So the two resolution paths differ: `/api/installers/` (build-classifier)
keeps them distinct, `/api/releases/` (normalize.js) merges them.
**Your Go rewrite should keep them distinct** to match the installer path.

**bare `arm`:** Three different answers depending on which layer:
- `sass/releases.js`: explicitly maps `arm: 'armv7'` (correct for Dart Sass)
- `normalize.js`: regex `/(arm|aarch32|arm[_\-]?v?6l?)(\b|_)/i` → `armv6l`
- `triplet.js` PRIMARY: `tpm['arm'] = T.NONE` (no classification)
- `triplet.js` TIERED (last resort): `arm: T.ARMHF` → `{ arch: 'armhf' }`

So for Sass specifically, production gets `armv7` because `releases.js` overrides.
For the build-classifier (your path), bare `arm` defaults to `armhf` as a last
resort via the tiered map. Your default of `armv6` is different from both `armv7`
(Sass releases.js) and `armhf` (triplet.js tiered). Consider matching the tiered
map behavior (`armhf`) or handling it per-package.

**ffmpeg Windows `.gz`:** Production `ffmpeg/releases.js` hardcodes `rel.ext = 'exe'`
for Windows assets (line 26). The `.gz` file contains a gzipped bare executable.
There's no generic reclassification — it's per-package override logic in releases.js.
Your Go rewrite would need equivalent logic in `ffmpeg/releases.conf` or the classifier.

**terraform `alpha` channel:** Production bug confirmed. `terraform/releases.js` uses
`alphaRe = /\d-alpha\d/` — requires a digit IMMEDIATELY after `alpha` (e.g., `alpha1`
or `alpha20210811`). If the version has a dash between alpha and the number
(`1.0.0-alpha-20210811`), the regex fails. Those versions get `channel: 'stable'`.
Go correctly identifies these as `alpha`. **Go is right, prod is wrong.** Safe to keep.

**postgres `tar` vs `tar.gz`:** Root cause confirmed. `postgres/releases.js` has
hardcoded legacy `originalReleases` entries with `ext: 'tar'` pre-set (lines 22, 33).
The actual filenames ARE `.tar.gz` (e.g., `postgresql-10.12-1-linux-x64-binaries.tar.gz`).
Since normalize.js only runs extension detection `if (!rel.ext)`, the wrong pre-set
value is never corrected. Go correctly derives `.tar.gz` from the filename — Go is right.
No functional impact: `tar` and `.tar.gz` are treated the same in format selection
(both use the `tar` branch of getSortedFormats). Safe to keep Go's behavior.

**iterm2 `beta` channel:** Root cause confirmed. `iterm2/releases.js` line 48 sets
channel based on `/\/stable\//.test(link)`. Old iTerm2 URLs (before they added the
`/stable/` and `/beta/` subdirectories) don't contain `/stable/` → classified `beta`.
Production's stale disk cache from before this fix keeps those as `beta`. Go fetches
fresh data and correctly reads `channel: 'stable'` from the URL pattern (if the URL
has changed). **Go is right, prod cache is stale.** Safe to keep.

## Latest Findings (2026-03-11)

### macOS amd64 default is acceptable

normalize.js defaults macOS packages without arch to `amd64` (line 118-120).
Project owner confirmed: amd64 is arm64's natural fallback via Rosetta 2, so this
works in practice. Per-package `releases.js` should handle cases where arch is known.

### Client format probe has no zst

`webi.sh` builds `formats=` by probing for installed tools:
`tar,exe,zip,xz,git,dmg,pkg`. It never checks for `unzstd`.

Server-side zst priority is forward-looking only — takes effect once webi.sh adds
zst detection. Your Go server should still prioritize zst in format sorting, but
current clients won't request it.

### atomicparsley — hardcoded target map

`atomicparsley/releases.js` uses hardcoded filename→target mappings, no
normalize.js detection:
- `Alpine` → `{ os: 'linux', arch: 'amd64', libc: 'musl' }` (hard musl)
- `Windows.` → `{ os: 'windows', arch: 'amd64', libc: 'msvc' }`
- `WindowsX86.` → `{ os: 'windows', arch: 'x86', libc: 'msvc' }`
- `Linux.` → `{ os: 'linux', arch: 'amd64', libc: 'gnu' }`
- `MacOS` → `{ os: 'macos', arch: 'amd64' }`

For your Go rewrite: `atomicparsley` needs a `releases.conf` with asset pattern
overrides, not generic filename detection.

### Two different UA parsers

The two resolution paths use different UA parsers with different naming:
- `/api/releases/` → `ua-detect.js`: returns `macos`, `arm64`, `amd64`
- `/api/installers/` → `host-targets.js` `termsToTarget()`: returns `darwin`, `aarch64`, `x86_64`

Both parse the same UA string. Results map to the same platforms but use the naming
conventions of their respective resolution layers.

### lexver version sorting

`lexver.js` pads versions to 4-level zero-padded form: `v1.2.3` → `0001.0002.0003.0000@`.
Stable suffix `@` sorts after pre-release `-` (ASCII ordering). Channel names recognized:
`alpha`, `beta`, `dev`, `pre`, `preview`, `rc`, `hotfix`. `hotfix` sorts as post-stable.

## Disk Cache Format (for pgstore reference)

`_cache/YYYY-MM/<pkg>.json` stores an array of release objects. Each entry:

```json
{
  "name": "bat-v0.24.0-x86_64-unknown-linux-musl.tar.gz",
  "version": "v0.24.0",
  "lts": false,
  "channel": "stable",
  "date": "2024-01-01",
  "os": "linux",
  "arch": "x86_64",
  "libc": "none",
  "ext": ".tar.gz",
  "download": "https://github.com/..."
}
```

- Naming: build-classifier style (`darwin`, `x86_64`, `aarch64`, `none`)- Empty string `""` for unknown fields, not `null`
- `_cache/YYYY-MM/<pkg>.updated.txt` stores the update timestamp (ISO string or ms)

## Skill Updates

At `/Users/aj/Projects/claude/webinstall.dev/.claude/skills/webi-server/`:
- `resolution.md` — corrected triplet order, arch WATERFALL, format priority, macOS amd64 note
- `installer-pipeline.md` — full install flow, extraction, PATH management, client format probe
- `ua-detection.md` — two UA parsers documented, format detection details
- `SKILL.md` — release source types, client format probe missing zst, all known bugs

## Resolved Items

- [x] ANYOS-first triplet order — confirmed, harmless in practice
- [x] illumos/solaris/sunos — three distinct values in build-classifier
- [x] bare `arm` — NONE in primary, armhf in tiered fallback
- [x] ffmpeg Windows `.gz` → `exe` — per-package override in releases.js
- [x] Libc two-phase model, hard musl exceptions
- [x] Bootstrap grep bug — low impact
- [x] Format detection — webi.sh probes for tools (no zst)
- [x] macOS amd64 default — acceptable (Rosetta fallback)
- [x] atomicparsley — hardcoded target map, hard musl
- [x] Two UA parsers — different naming per resolution path
- [x] Per-package release source patterns (8 source types, 12+ override patterns)

## Per-Package Patterns Requiring Go Equivalents

These packages need special handling in the Go rewrite beyond generic GitHub releases:

**Non-GitHub sources (need custom fetchers):**
- `node` — custom JSON API (nodejs.org/download/release/index.json) + **unofficial-builds.nodejs.org** (merged); odd major→beta, even→stable; EOL filter (>366 days old excluded); API `files` array has platform strings like `linux-x64-musl`
- `go` (golang.org/dl/?mode=json&include=all) — strips `go` prefix, pads to 3-part semver; stable/beta only; date always 1970
- `zig` — custom JSON API at ziglang.org
- `gpg` — SourceForge RSS feed (macOS .dmg only; LTS = 2.2.x)
- `mariadb` — two-stage REST API; `Alpha` maps to `preview` channel (not `alpha`)
- `macos` — web scraping apple.com
- `iterm2` — web scraping iterm2.com
- `pathman` — Gitea instance (git.rootprojects.org, uses same githubish.js API)

**Version format overrides (need releases.conf):**
- `monorel` — strip `tools/monorel/` prefix from monorepo tags
- `lf` — convert `r21` → `0.21.0`
- `watchexec` — strip `cli-` prefix from workspace tags
- `jq` — strip `jq-` prefix
- `iterm2` — convert `3_5_0beta17` → `3.5.0-beta17`

**Asset manipulation:**
- `ollama` — duplicates universal Darwin builds for both x86_64 and aarch64, maps ROCM variant to `x86_64_rocm`
- `aliasman` — sets `os: 'posix_2017'` on all releases (POSIX-portable)
- `serviceman` — merges releases from two GitHub repos (old + new owner)
- `kubectx`/`kubens` — same source repo, inverse filtering
- `deno` — injects version into filename if missing
- `hugo` — filters extended builds and old alias names

**Channel filtering difference (two resolution paths):**
- Releases path (`/api/releases/`): `channel=beta` is strict (only beta passes)
- Installers path (`/api/installers/`): `channel=beta` accepts ALL versions
  (only `channel=stable` actually filters; anything else is permissive)
