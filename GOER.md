# Status from Go agent (ref-webi-go)

## Completed (Phase 4)

- [x] pgstore: PostgreSQL storage (pgx v5, double-buffer, CopyFrom bulk insert)
- [x] storage.Store interface: added ListPackages(ctx); both fsstore and pgstore implement
- [x] webid + webicached: -pg=<dsn> flag to select pgstore over fsstore

## Completed (earlier)

- [x] Libc classification: Rust static musl → `none`, hard-musl packages keep `musl`
- [x] Windows gnu→none: MinGW self-contained
- [x] install.sh 8-space padding, PowerShell rendering
- [x] pg asset_filter: server-only assets (includes client)
- [x] comparecache field-level diffs: os/arch/libc/ext/channel with equivalence matching
- [x] Go dist: use API structured os/arch (illumos/solaris kept distinct)
- [x] sass: bare arm → armv7 in classifier (Dart Sass targets ARMv7)
- [x] LegacyBackport: separate core classifier (canonical) from legacy cache translation
  - Go dist: armv6 → arm (prod keeps raw API value)
  - ffmpeg: Windows .gz → exe (prod releases.js override)

## comparecache status: 3 remaining diffs (all production bugs)

| Package | Field | Count | Go (correct) | Prod (wrong) | Why |
|---------|-------|-------|-------------|-------------|-----|
| iterm2 | channel | 11 | stable | beta | Raw cache says stable (URL path `/stable/`). Prod misclassifies. |
| postgres | ext | 2 | .tar.gz | tar | normalize.js strips compression layer. File IS .tar.gz. |
| terraform | channel | 14 | alpha | stable | Version contains `-alpha-`. Prod regex misses it. |

All other packages (98/101) have zero field-level disagreements.
Not backporting these — Go is correct, production has bugs.

## Question for Researcher

Re: ANYOS-first triplet order — you say production does `['ANYOS', 'posix_2017', 'posix_2024', hostTarget.os]`.
Go currently does specific-OS-first: `[osStr, 'posix_2024', 'posix_2017', 'ANYOS', '']`.

**Is ANYOS-first correct for production?** That would mean a platform-agnostic build (e.g. git repo) is preferred over a native binary. That seems wrong for most packages. Can you double-check builds-cacher.js and confirm the actual iteration order that reaches the resolution logic?

## Known gaps

- **atomicparsley**: `AtomicParsleyAlpine.zip` — needs package-specific musl handling
- **Hugo .pkg**: macOS v0.153+ only ships `.pkg`. Latent bug in both Go and production.
