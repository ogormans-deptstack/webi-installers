# Status from Go agent (ref-webi-go)

## Completed

- [x] Libc classification: Rust static musl → `none`, hard-musl packages keep `musl`
- [x] Windows gnu→none: MinGW self-contained
- [x] install.sh 8-space padding, PowerShell rendering
- [x] pg asset_filter: server-only assets (includes client)
- [x] comparecache field-level diffs: os/arch/libc/ext/channel with equivalence matching
- [x] Go dist: use API structured os/arch (illumos/solaris kept distinct)
- [x] sass: bare arm → armv7 (Dart Sass convention, per releases.js)
- [x] ffmpeg: Windows .gz → ext exe (gzipped bare executables, per releases.js)
- [x] Go dist: keep bare arm as-is (matches production)

## comparecache status: 3 remaining diffs (all understood)

| Package | Field | Count | Issue |
|---------|-------|-------|-------|
| iterm2 | channel | 11 | Old 3.0.x versions: prod=beta, Go=stable. Edge case in URL-path detection. |
| postgres | ext | 2 | Legacy EDB: prod=tar, Go=tar.gz. normalize.js quirk. |
| terraform | channel | 14 | Go correctly detects alpha, prod misses it. Production bug. |

All other packages (98/101) have zero field-level disagreements.

## Question for Researcher

Re: ANYOS-first triplet order — you say production does `['ANYOS', 'posix_2017', 'posix_2024', hostTarget.os]`.
Go currently does specific-OS-first: `[osStr, 'posix_2024', 'posix_2017', 'ANYOS', '']`.

**Is ANYOS-first correct for production?** That would mean a platform-agnostic build (e.g. git repo) is preferred over a native binary. That seems wrong for most packages. Can you double-check builds-cacher.js and confirm the actual iteration order that reaches the resolution logic?

## Known gaps

- **atomicparsley**: `AtomicParsleyAlpine.zip` — needs package-specific musl handling
- **Hugo .pkg**: macOS v0.153+ only ships `.pkg`. Latent bug in both Go and production.
