# Status from Go agent (ref-webi-go)

## Completed

- [x] Libc classification: Rust static musl → `none`, hard-musl packages keep `musl`
- [x] Windows gnu→none: MinGW self-contained
- [x] install.sh 8-space padding, PowerShell rendering
- [x] pg asset_filter: server-only assets (includes client)
- [x] comparecache field-level diffs: os/arch/libc/ext/channel with equivalence matching

## comparecache findings (real diffs after equivalence)

| Package | Field | Issue |
|---------|-------|-------|
| go | os | `illumos`/`solaris` → Go maps both to `sunos`, prod keeps distinct |
| go | arch | bare `arm` ambiguous — Windows=arm64, FreeBSD/Plan9=armv6 |
| sass | arch | bare `arm` = armv7 (Dart), Go defaults to armv6 |
| ffmpeg | ext | Windows `.gz` = gzipped exe, prod says `exe` |
| postgres | ext | legacy EDB: prod says `tar`, Go says `tar.gz` |
| terraform | channel | `alpha` correctly detected by Go, prod misses it |
| iterm2 | channel | old versions differ |

## Question for Researcher

Re: ANYOS-first triplet order — you say production does `['ANYOS', 'posix_2017', 'posix_2024', hostTarget.os]`.
Go currently does specific-OS-first: `[osStr, 'posix_2024', 'posix_2017', 'ANYOS', '']`.

**Is ANYOS-first correct for production?** That would mean a platform-agnostic build (e.g. git repo) is preferred over a native binary. That seems wrong for most packages. Can you double-check builds-cacher.js and confirm the actual iteration order that reaches the resolution logic?

## Known gaps

- **atomicparsley**: `AtomicParsleyAlpine.zip` — needs package-specific musl handling
- **Hugo .pkg**: macOS v0.153+ only ships `.pkg`. Latent bug in both Go and production.
