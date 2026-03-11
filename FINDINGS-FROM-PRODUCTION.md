# Findings from Live Production API

Verified against `https://webinstall.dev/api/releases/` on 2026-03-11 by the
documentation agent working in the webinstall.dev repo.

## Issue 3: Man pages and non-installable files

**Production DOES include `.1` man pages.** Example:
- `jq.1` appears in jq releases (v1.5rc1) with `ext: "1"`

**Production DOES include `.deb` files.** Example:
- `bat-musl_0.26.1_arm64.deb` appears with `ext: "deb"`, `os: "unknown"`
- `bat-musl_0.26.1_musl-linux-amd64.deb` with `ext: "deb"`, `os: "linux"`

**Production DOES include `.txt` files.** Example:
- Checksum files appear in jq releases

**Production does NOT include**: `.sig`, `.sha256`, `.sbom`, `.pem` files (these
are likely filtered by normalize.js or not present in the GitHub release assets
that the old releases.js fetchers returned).

### Implication for Go cache

If the Go cache is being compared against production for compatibility testing,
it should include `.deb` files in the `/api/releases/` listing since production
does. The `.deb` files won't be selected by the installer resolver (the format
preference list is `tar,exe,zip,xz,dmg`), but they should appear in the API
listing.

However, excluding `.deb`/`.rpm` from the cache is arguably an improvement —
webi can't install them, and they clutter the API response. This should be
documented as an intentional difference if you choose to keep the exclusion.

## Libc values in production

Verified with `bat` on production:

| Filename | libc (production) |
|----------|------------------|
| `bat-v0.26.1-aarch64-unknown-linux-musl.tar.gz` | `none` |
| `bat-v0.26.1-aarch64-unknown-linux-gnu.tar.gz` | `gnu` |
| `bat-v0.26.1-x86_64-apple-darwin.tar.gz` | `none` |
| `bat-v0.26.1-x86_64-pc-windows-msvc.zip` | `none` |

Key insight: Production normalize.js DOES produce `libc: "gnu"` for files with
`gnu` in the filename. It's not all collapsed to `none` as previously assumed.
The distinction is: `musl` in filename → `none` (static), `gnu` in filename →
`gnu` (needs glibc).

## serve-releases.js libc query param bug

Line 37 of `webinstall.dev/lib/serve-releases.js` calls `UaDetect.arch()`
instead of `UaDetect.libc()` for the libc query parameter:

```javascript
var libc = UaDetect.arch(req.query.libc || 'libc' /*ua*/);
//         ^^^^^^^^^^^^^^ BUG: should be UaDetect.libc()
```

This means `?libc=musl` on the releases API always evaluates to `'error'`
(arch detection fails on libc strings). Libc filtering via query params is
effectively broken in production. The only reason it doesn't cause visible
issues is that most API consumers don't filter by libc.

## render.go: install.sh padding difference

Production Node.js (`_webi/installers.js`) pads every line of `install.sh` with
8 spaces before injecting it into the template:

```javascript
function padScript(txt) {
  return txt.replace(/^/g, '        ');  // 8 spaces
}
```

The template marker is at line 424 of `package-install.tpl.sh`:
```sh
        # {{ installer }}
```

So in production, the marker (including its leading whitespace) is replaced by
the content of install.sh with each line padded to 8 spaces. The result is that
the install.sh content aligns with the surrounding code inside
`__init_installer()`.

Your Go `render.go` does `strings.Replace(text, "# {{ installer }}", ...)` which
replaces just `# {{ installer }}` but leaves the leading 8 spaces on that line,
and doesn't pad subsequent lines. This means:
1. First line of install.sh gets prepended with the leftover whitespace
2. Subsequent lines of install.sh start at column 0

This is probably fine for shell execution (sh doesn't care about indentation),
but if you want byte-for-byte compatibility with production output, you'll need
to pad install.sh content to 8 spaces per line.

The Node regex for the marker is `/\s*#?\s*{{ installer }}/` which matches
leading whitespace too, so the entire line gets replaced.

## Issue 4: Hugo darwin-universal on production

Production handles `darwin-universal` builds correctly:

```
curl -A 'aarch64/unknown Darwin/24.2.0 libc' \
  'https://webinstall.dev/api/installers/hugo@stable.sh'
```

Returns:
- `WEBI_VERSION='0.157.0'`
- `WEBI_ARCH='aarch64'`
- `WEBI_EXT='pkg'`
- `WEBI_PKG_URL='https://github.com/gohugoio/hugo/releases/download/v0.157.0/hugo_0.157.0_darwin-universal.pkg'`

The releases API shows these as `arch: "amd64"` (normalize.js doesn't recognize
`universal` as an arch, defaults to `amd64` for macOS). The installer serves
them to arm64 hosts via the macOS arm64→amd64 fallback in transform-releases.js.

So the production "fix" is accidental: normalize.js maps `universal` to `amd64`,
and the arm64→amd64 fallback catches it. For the Go cache, the proper fix would
be to classify `universal`/`universal2` as matching both `aarch64` and `x86_64`.

Also note: Hugo only has `.pkg` files for macOS (no `.tar.gz`). The `.pkg`
format is not in the default format preference list (`tar,exe,zip,xz,dmg`) but
gets served because it's the only macOS option available after all preferred
formats are exhausted.

## Hugo .pkg is a latent bug in production

The framework template `package-install.tpl.sh` does NOT support `.pkg`
extraction. The `webi_extract()` function handles: tar.zst, tar.xz, tar.gz,
tar.bz2, tar, zip, app.zip, exe, git, xz. A `.pkg` file hits the `else` branch
and **exits with error code 1**.

Hugo v0.153+ switched macOS releases from `.tar.gz` to `.pkg` only. So
`webi install hugo` on macOS would resolve to the `.pkg` file and fail during
extraction.

This hasn't been widely noticed because:
1. Users who installed Hugo before v0.153 already have it cached
2. The error happens client-side, not server-side

For the Go rewrite, you could:
- Add `.pkg` extraction support (macOS `.pkg` files can be extracted with
  `pkgutil --expand-full` or `xar -xf`)
- Or filter out `.pkg` from format preferences and fall back to older tar.gz
  versions
- Or add `pkg` to the WEBI_FORMATS default list

Hugo macOS releases by format:
- v0.157.0 to v0.153.0: `.pkg` only
- v0.152.2 and earlier: `.tar.gz`

## TSV format details (verified)

With `?pretty=true`:
```
VERSION	LTS	CHANNEL	RELEASE_DATE	OS	ARCH	EXT	HASH	URL	_	LIBC
24.14.0	lts	stable	2026-02-24	aix	ppc64	tar.gz	-	https://...		none
```

- LTS column: string `lts` (not boolean `true`) or `-`
- HASH column: always `-`
- `_` column: always empty (placeholder)
- LIBC column: `none`, `gnu`, `musl`, or empty

Without `?pretty=true`: no header row, same data columns.
