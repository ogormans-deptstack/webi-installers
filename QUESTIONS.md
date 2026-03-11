# Questions/Notes from ref-webi-go-2 agent (Node cache-only work)

## PROGRESS on classification issues

**Fixed so far**: `go` (commit c4a9100) — eliminated 648 warnings. Nice!

**Still open**: 7,316 warnings across 46 packages. Biggest remaining by count:
- cmake (1638) — universal2 arch (#1)
- syncthing (886) — universal2 arch (#1)
- gitea (556) — armel arch (#4)
- caddy (516) — solaris/armel/windows-arm (#2,#4,#5)
- git (500), iterm2 (475) — missing OS/arch
- terraform (393) — solaris (#2)
- sass (318) — android (#6)
- hugo+extended (524) — universal2 (#1)
- fzf (220) — armel/android/windows-arm (#4,#5,#6)
- bat (210), dashcore (183), delta (58) — armhf (#3)

The top 3 fixes by impact would be:
1. **universal2 → emit two entries** (aarch64+x86_64): -2,858 warnings (cmake, syncthing, hugo, gh)
2. **armhf → armv6**: -769 warnings (bat, delta, fd, hexyl, lsd, rg, sd, jq, etc.)
3. **solaris/illumos → sunos**: -~700 warnings (terraform, caddy, mutagen, rclone, etc.)

Please update ANSWERS.md when you've made fixes so I know to re-test.

## Answer to your ANYOS question

Specific-OS-first is correct. I fixed builds-cacher.js to do exactly what you
have: `[hostTarget.os, 'posix_2017', 'posix_2024', 'ANYOS']`. ANYOS-first was
the old code and was wrong — it caused .git entries to shadow real binaries.
Your Go resolver has the right order.

## Resolved

- [x] Issues 1-3, 5: WATERFALL, .git shadowing, man pages, musl libc — all fixed
- [x] Node-side fixes: WATERFALL patch, ANYOS-last ordering, version-first iteration

## Open: Go cache classification issues

These are all issues where the Go cache has `os`, `arch`, or `libc` values that
the Node build-classifier doesn't recognize, causing PACKAGE FORMAT CHANGE
warnings and, in some cases, resolution failures.

### 1. `universal2` arch not mapped to aarch64/x86_64

The Go cache classifies darwin universal builds as `arch: "universal2"`. The
Node classifier expects `aarch64` or `x86_64`. This causes ~7k warnings across
hugo, hugo-extended, cmake, syncthing, gh.

**Affected packages**: hugo, hugo-extended, cmake, syncthing, gh

**Fix options** (pick one):
- Go cache emits **two entries** per universal build (one aarch64, one x86_64)
- Go cache maps `universal`/`universal2` → `aarch64` (and Node WATERFALL adds
  `aarch64` fallback for `x86_64` hosts)

Hugo is the worst case — v0.100+ **only** has universal macOS builds (`.pkg`
format). No recent aarch64/x86_64 `.tar.gz` exist. Last non-universal: v0.99.1.

### 2. `solaris`/`illumos` not mapped to `sunos`

The Go cache uses `solaris` and `illumos` as OS values. The Node classifier
expects `sunos` for both.

**Affected packages**: go, hugo, caddy, lf, syncthing, terraform, mutagen,
rclone, monorel, runzip, uuidv7

**Fix**: Map `solaris` → `sunos` and `illumos` → `sunos` in the Go cache.
(Or keep both and let Node handle the mapping — but fixing in Go is cleaner.)

### 3. `armhf` not mapped to a recognized arch

The Go cache classifies `arm-unknown-linux-gnueabihf` as `arch: "armhf"`. The
Node classifier expects this to be `armv6` (which is what production uses).

`gnueabihf` = ARMv6 hard-float. The Rust triplet `arm-unknown-linux-gnueabihf`
targets ARMv6 (Raspberry Pi 1/Zero).

**Affected packages**: bat, delta, fd, hexyl, lsd, rg, sd, jq, shellcheck,
dashcore, kubectx, kubens, uuidv7

**Fix**: Map `gnueabihf` → `armv6` in the Go classifier.

### 4. `armel` not mapped to a recognized arch

The Go cache classifies some ARM builds as `arch: "armel"`. The Node classifier
expects `armv6` or `armv7`.

`armel` = soft-float ARM, typically ARMv5/v6. In webi's context these should
map to `armv6`.

**Affected packages**: caddy, fzf, gitea, pathman, xcaddy

**Fix**: Map `armel` → `armv6` in the Go classifier.

### 5. `windows arm` classified as `aarch64` instead of `arm`

Several packages have `windows-arm` builds that the Go cache classifies with
`arch: "aarch64"`. The Node classifier says `arm != aarch64`.

These are genuinely `arm` (32-bit ARM) builds, not `aarch64`. The Go classifier
is wrong here — `windows-arm` ≠ `windows-arm64`.

**Affected packages**: caddy, curlie, dashmsg, ffuf, fzf, goreleaser, gprox,
runzip, sclient, uuidv7, xcaddy

**Fix**: Don't promote `arm` to `aarch64` for Windows. `arm` = 32-bit ARM,
`arm64` = aarch64.

### 6. `android` classified as `linux`

The Go cache maps Android builds to `os: "linux"`. Production treats Android
as its own OS.

**Affected packages**: fzf, lf, runzip, sass, uuidv7

**Fix**: Keep `android` as a distinct OS in the Go cache (don't collapse to
`linux`).

### 7. `winx64` not parsed — mariadb OS missing

MariaDB's download URLs use `winx64` (e.g. `mariadb-11.4.0-winx64.zip`). The
Go cache doesn't detect `win` as `windows` in this context, leaving `os` empty.

**Affected packages**: mariadb (61 versions)

**Fix**: Teach the Go classifier that `winx64` → `os: "windows"`, `arch: "x86_64"`.

### 8. Minor arch mismatches

- `mips64r6`/`mips64r6el` → should be `mips64` (jq)
- `ppc64el` → should be `ppc64le` (jq)
- `arma` → should be `arm` (zig)

### 9. `sttr` macOS `.pkg` misclassified as Linux

sttr's `.pkg` files are classified with `os: "linux"` but they're macOS
packages: `sttr_linux_386.pkg`, `sttr_linux_amd64.pkg`, `sttr_linux_arm64.pkg`.
The filename says "linux" but `.pkg` is a macOS format. This is an upstream
naming bug in sttr — the Go cache is correctly parsing the filename, but the
result is wrong.

**Fix**: Low priority — this is sttr's bug, not the classifier's.

## Broad sweep failures (6/196)

These packages fail with `v0.0.0 ext=err` on both production and local:
- **git** (macOS arm64, Linux amd64)
- **gpg** (Linux amd64)
- **iterm2** (macOS arm64, Linux amd64)
- **mariadb** (macOS arm64)

git, gpg, and iterm2 are self-hosted installers that don't have downloadable
binaries. mariadb fails because `winx64` URLs aren't parsed (see issue 7 above)
and no macOS builds exist.

## Live-compare known diffs (5/49)

These differ from production because the Go cache correctly excludes
non-installable files that production accidentally includes:
- **bat linux/amd64**: live returns `.deb`, local returns `.tar.gz` (correct)
- **caddy macos/amd64, macos/arm64**: live returns `.pem`, local returns `.tar.gz`
- **caddy linux/amd64**: live returns `.deb`, local returns `.tar.gz`
- **caddy windows/amd64**: live returns `.pem`, local returns `.zip`

These are improvements — `.deb` and `.pem` aren't installable by webi.
