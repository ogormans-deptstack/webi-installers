# Installer Notes — Package Structure Inspection

Tracking package format evolution and install structure for each inspected package.

## sd

Single binary CLI tool for regex find-and-replace.

### Format Evolution

| Era | Versions | Linux | Darwin | Windows |
|-----|----------|-------|--------|---------|
| Current | v1.0.0+ (2023-11) | .tar.gz | .tar.gz | .zip |
| Legacy | v0.6.5 and earlier | .zip | .zip | .zip |

### Archive Structure

**v1.0.0+ (current)**:
```
sd-v1.1.0-{triplet}/
├── sd*              (binary)
├── sd.1             (man page)
├── completions/
│   ├── sd.bash
│   ├── sd.fish
│   ├── sd.elv
│   ├── _sd          (zsh)
│   └── _sd.ps1      (powershell)
├── CHANGELOG.md
├── LICENSE
└── README.md
```
Triplets: `aarch64-apple-darwin`, `x86_64-unknown-linux-gnu`, `x86_64-pc-windows-gnu`

**v0.6.5 (legacy)**:
```
{triplet}/
└── sd*              (binary only — no completions, no man page, no docs)
```

### Format Change
- v0.7.6 → v1.0.0: switched from zip to tar.gz (Linux/Mac), added completions + man page
- v0.6.5 and earlier: zip only, bare binary in triplet-named directory

### Install Script (v1.0.0+)
```sh
# Unpack
tar xzf sd-v${VERSION}-${TRIPLET}.tar.gz

# Install binary
mkdir -p ~/.local/opt/sd-${SORT_VER}/bin
mv sd-v${VERSION}-${TRIPLET}/sd ~/.local/opt/sd-${SORT_VER}/bin/sd

# Install completions
mkdir -p ~/.local/opt/sd-${SORT_VER}/share/bash-completion/completions
mv sd-v${VERSION}-${TRIPLET}/completions/sd.bash ~/.local/opt/sd-${SORT_VER}/share/bash-completion/completions/sd

mkdir -p ~/.local/opt/sd-${SORT_VER}/share/fish/vendor_completions.d
mv sd-v${VERSION}-${TRIPLET}/completions/sd.fish ~/.local/opt/sd-${SORT_VER}/share/fish/vendor_completions.d/sd.fish

mkdir -p ~/.local/opt/sd-${SORT_VER}/share/zsh/site-functions
mv sd-v${VERSION}-${TRIPLET}/completions/_sd ~/.local/opt/sd-${SORT_VER}/share/zsh/site-functions/_sd

# Install man page
mkdir -p ~/.local/opt/sd-${SORT_VER}/share/man/man1
mv sd-v${VERSION}-${TRIPLET}/sd.1 ~/.local/opt/sd-${SORT_VER}/share/man/man1/sd.1

# Symlinks
ln -sf ~/.local/opt/sd-${SORT_VER} ~/.local/opt/sd
ln -sf ~/.local/opt/sd/bin/sd ~/.local/bin/sd
```

## ollama

LLM inference server with CLI interface.

### Format Evolution

| Era | Versions | Linux | Darwin | Windows |
|-----|----------|-------|--------|---------|
| tar.zst | v0.14.0+ (2025-10) | .tar.zst | — | — |
| No GitHub Linux | v0.4.0–v0.13.x | — | — | — |
| Bare binary | v0.1.0–v0.3.6 | bare binary | — | — |
| DMG added | v0.9.4+ | — | .zip + .dmg | — |
| Win zip+arm64 | v0.4.0+ | — | — | .zip (amd64+arm64) |
| Win zip | v0.1.33–v0.3.6 | — | — | .zip (amd64 only) |
| Win exe only | v0.1.25–v0.1.32 | — | — | .exe only |
| Darwin always | v0.0.1+ | — | .zip (Ollama.app) | — |

Darwin zip always contains `Ollama.app/` (macOS GUI app).
Darwin bare binary `ollama-darwin` is CLI-only (dropped at v0.5.8).
OllamaSetup.exe is a Windows installer (not a bare binary).

### Archive Structure

**Linux tar.zst (v0.14.0+)**:
```
bin/ollama*                      (CLI binary)
lib/ollama/
├── libggml-base.so*
├── libggml-cpu-{variant}.so*    (alderlake, haswell, icelake, sandybridge, skylakex, sse42, x64)
├── cuda_v12/                    (CUDA 12 GPU libs)
│   ├── libcublas.so.12*
│   ├── libcublasLt.so.12*
│   ├── libcudart.so.12*
│   └── libggml-cuda.so*
├── cuda_v13/                    (CUDA 13 GPU libs)
│   └── ...
├── mlx_cuda_v13/                (MLX+CUDA GPU libs)
│   └── ...
└── vulkan/                      (Vulkan GPU libs)
    └── ...
```
66 files total. ~1.7GB compressed.

**Darwin zip (all versions)**:
```
Ollama.app/
├── Contents/
│   ├── MacOS/Ollama*            (GUI app binary)
│   ├── Resources/
│   │   ├── ollama*              (CLI binary — embedded)
│   │   ├── libggml-*.so         (CPU inference libs)
│   │   ├── libmlx.dylib         (Apple MLX framework)
│   │   ├── mlx.metallib         (Metal shaders, 107MB)
│   │   ├── icon.icns
│   │   └── ollama*.png          (menu bar icons)
│   ├── Library/LaunchAgents/
│   │   └── com.ollama.ollama.plist
│   ├── Frameworks/
│   │   └── Squirrel.framework/  (auto-updater)
│   └── Info.plist
```
48 files, 336MB uncompressed (v0.17.7). Earlier v0.0.x used Electron (275 files).

**Windows arm64 zip**:
```
ollama.exe                       (CLI binary)
vc_redist.arm64.exe             (VC++ redistributable)
```
Windows amd64 zip is ~1.9GB (includes CUDA libs like Linux).

**Linux bare binary (v0.1.0–v0.3.6)**:
Single `ollama-linux-{amd64,arm64}` file, 300-400MB. Self-contained with bundled GPU libs.

### Key Observations

1. **Ollama is NOT a simple single-binary install.** Modern versions need `lib/ollama/` for GPU acceleration.
2. **Darwin .zip is a macOS .app** — not suitable for `~/.local/bin` install. The CLI binary lives inside at `Ollama.app/Contents/Resources/ollama`.
3. **The darwin bare binary** (`ollama-darwin`) was the CLI-only option but was dropped at v0.5.8.
4. **Linux switched distribution models 3 times**: bare binary → no GitHub release (use install.sh) → tar.zst archive.
5. **rocm and jetpack variants** exist for Windows/Linux (excluded from analysis — GPU-vendor-specific).
6. **Windows arm64 is tiny (22MB)** vs amd64 (1.9GB) because arm64 has no CUDA/GPU libs bundled.

### Install Script (Linux tar.zst, v0.14.0+)
```sh
# Unpack
tar --zstd -xf ollama-linux-amd64.tar.zst -C ~/.local/opt/ollama-${SORT_VER}/

# Already has bin/ollama and lib/ollama/ layout
# Symlinks
ln -sf ~/.local/opt/ollama-${SORT_VER} ~/.local/opt/ollama
ln -sf ~/.local/opt/ollama/bin/ollama ~/.local/bin/ollama
```
Note: ollama needs `OLLAMA_LIB_DIR` or `LD_LIBRARY_PATH` set to find `lib/ollama/`.

### Install Script (Darwin — extract CLI from .app)
```sh
# Unpack
unzip Ollama-darwin.zip -d /tmp/ollama-extract/

# Install CLI binary
mkdir -p ~/.local/opt/ollama-${SORT_VER}/bin
cp /tmp/ollama-extract/Ollama.app/Contents/Resources/ollama ~/.local/opt/ollama-${SORT_VER}/bin/ollama
chmod +x ~/.local/opt/ollama-${SORT_VER}/bin/ollama

# Install shared libs
mkdir -p ~/.local/opt/ollama-${SORT_VER}/lib/ollama
cp /tmp/ollama-extract/Ollama.app/Contents/Resources/libggml-*.so ~/.local/opt/ollama-${SORT_VER}/lib/ollama/
cp /tmp/ollama-extract/Ollama.app/Contents/Resources/libmlx*.dylib ~/.local/opt/ollama-${SORT_VER}/lib/ollama/
cp /tmp/ollama-extract/Ollama.app/Contents/Resources/mlx.metallib ~/.local/opt/ollama-${SORT_VER}/lib/ollama/

# Symlinks
ln -sf ~/.local/opt/ollama-${SORT_VER} ~/.local/opt/ollama
ln -sf ~/.local/opt/ollama/bin/ollama ~/.local/bin/ollama
```

### Install Script (Windows zip)
```powershell
# Unpack
Expand-Archive ollama-windows-arm64.zip -DestinationPath $env:USERPROFILE\.local\opt\ollama-$SORT_VER\bin\

# Symlinks
New-Item -ItemType SymbolicLink -Path "$env:USERPROFILE\.local\opt\ollama" -Target "$env:USERPROFILE\.local\opt\ollama-$SORT_VER"
New-Item -ItemType SymbolicLink -Path "$env:USERPROFILE\.local\bin\ollama.exe" -Target "$env:USERPROFILE\.local\opt\ollama\bin\ollama.exe"
```

---

## Batch Inspection Results

Inspected latest version of every downloadable package. Categorized by archive structure pattern.

### Pattern A: Bare Binary in Archive (no subdirectory)

Archive contains the binary (and maybe LICENSE/README) at the top level, no wrapper directory.

| Package | Archive | Contents |
|---------|---------|----------|
| awless | `awless-linux-amd64.tar.gz` | `awless` |
| bun | `bun-macos-x64.zip` | `bun` |
| caddy | `caddy_2.9.1_linux_amd64.tar.gz` | `caddy`, LICENSE, README.md |
| cilium | `cilium-linux-amd64.tar.gz` | `cilium` |
| curlie | `curlie_1.8.2_linux_amd64.tar.gz` | `curlie`, LICENSE, README.md |
| dashmsg | `dashmsg_0.9.1_Linux_x86_64.tar.gz` | `dashmsg`, LICENSE, README.md |
| dotenv | `dotenv_1.0.0_linux_x86_64.tar.gz` | `dotenv`, LICENSE, README.md |
| dotenv-linter | `dotenv-linter-linux-x86_64.tar.gz` | `dotenv-linter` |
| ffuf | `ffuf_2.1.0_linux_amd64.tar.gz` | `ffuf`, CHANGELOG.md, LICENSE, README.md |
| fzf | `fzf-0.70.0-linux_amd64.tar.gz` | `fzf` |
| gitdeploy | `gitdeploy_0.9.4_linux_x86-64.tar.gz` | `gitdeploy`, LICENSE, README.md |
| gprox | `gprox_0.3.3_linux_amd64.tar.gz` | `gprox`, LICENSE, README.md |
| grype | `grype_0.99.1_linux_amd64.tar.gz` | `grype`, CHANGELOG.md, LICENSE, README.md |
| hugo | `hugo_0.99.1_Linux-64bit.tar.gz` | `hugo`, LICENSE, README.md |
| hugo-extended | `hugo_extended_0.99.1_Linux-64bit.tar.gz` | `hugo`, LICENSE, README.md |
| keypairs | `keypairs_0.7.0-pre.2_linux_x86-64.tar.gz` | `keypairs`, LICENSE, README.md |
| koji | `koji-x86_64-unknown-linux-musl.tar.gz` | `koji` |
| lf | `lf-linux-amd64.tar.gz` | `lf` |
| monorel | `monorel_0.6.6_Linux_x86_64.tar.gz` | `monorel`, README.md |
| ots | `ots-v1.1.0-linux-amd64.tar.gz` | `ots`, LICENSE, README.md |
| runzip | `runzip_1.0.1_linux_amd64.tar.gz` | `runzip`, LICENSE, README.md |
| sclient | `sclient_1.5.1_linux_amd64.tar.xz` | `sclient`, LICENSE, README.md |
| sqlc | `sqlc_1.9.0-beta1_linux_amd64.tar.gz` | `sqlc` |
| sqlpkg | `sqlpkg-cli_0.3.0_linux_amd64.tar.gz` | `sqlpkg`, LICENSE, README.md |
| sttr | `sttr_0.2.9_linux_amd64.tar.gz` | `sttr`, LICENSE, README.md |
| uuidv7 | `uuidv7_v1.0.1-next_linux_amd64.tar.gz` | `uuidv7`, LICENSE, README.md |
| xcaddy | `xcaddy_0.4.5_linux_amd64.tar.gz` | `xcaddy`, LICENSE, README.md |

**Install pattern**: extract, move binary to `~/.local/opt/{pkg}-{ver}/bin/{binary}`, symlink.

### Pattern B: Subdirectory with Binary Only

Archive contains a version-named directory wrapping the binary and docs.

| Package | Archive | Directory | Contents |
|---------|---------|-----------|----------|
| delta | tar.gz | `delta-{ver}-{triplet}/` | `delta`, LICENSE, README.md |
| hexyl | tar.gz | `hexyl-{ver}-{triplet}/` | `hexyl`, CHANGELOG.md, LICENSE-*, README.md |
| kubectx | tar.gz | flat | `kubens`, LICENSE |
| kubens | tar.gz | flat | `kubens`, LICENSE |
| pathman | tar.gz | bare name | `pathman-v0.6.0-linux-amd64_v1` (bare binary, no dir) |
| shellcheck | tar.xz | `shellcheck-{ver}/` | `shellcheck`, LICENSE.txt, README.txt |
| trip | tar.gz | `trippy-{ver}-{triplet}/` | `trip` |
| watchexec | tar.xz | `watchexec-{ver}-{triplet}/` | `watchexec`, LICENSE, README.md, completions/, man page |
| xsv | tar.gz | `xsv-{ver}-{triplet}/` | `xsv`, UNLICENSE, README.md |
| yq | tar.gz | flat | `yq_linux_amd64` (binary named with platform suffix) |

**Install pattern**: extract, find binary in subdirectory, move to opt, symlink.
Note: `pathman` is just a bare binary named with the full release tag. `yq` binary is named with platform suffix — needs rename.

### Pattern C: Subdirectory with Binary + Completions/Man Pages

Archive includes shell completions and/or man pages alongside the binary.

| Package | Archive | Completions | Man | Notes |
|---------|---------|-------------|-----|-------|
| bat | tar.gz | `autocomplete/` (dir exists but contents vary) | `bat.1` | |
| fd | tar.gz | `autocomplete/{fd.fish,fd.bash,fd.ps1,_fd}` | `fd.1` | |
| goreleaser | tar.gz | `completions/{.bash,.fish,.zsh}` | `manpages/goreleaser.1.gz` | |
| lsd | tar.gz | `autocomplete/{_lsd,lsd.fish,_lsd.ps1,lsd.bash-completion}` | `lsd.1` | |
| rg/ripgrep | tar.gz | `complete/{_rg.ps1,_rg,rg.fish,rg.bash}` | `doc/rg.1` | Also has doc/{FAQ,GUIDE,CHANGELOG}.md |
| sd | tar.gz | `completions/{sd.bash,sd.fish,sd.elv,_sd,_sd.ps1}` | `sd.1` | |
| watchexec | tar.xz | `completions/{bash,fish,zsh,powershell,nu,elvish}` | `watchexec.1` | |
| zoxide | tar.gz | `completions/{zoxide.bash,.fish,.elv,.nu,.ts,_zoxide,_zoxide.ps1}` | `man/man1/zoxide*.1` | Most complete |

**Install pattern**: extract, install binary, install completions to standard dirs, install man pages.

### Pattern D: Subdirectory with Binary + Libraries

Complex packages that bundle shared libraries alongside the binary.

| Package | Archive | Layout |
|---------|---------|--------|
| ollama (Linux) | tar.zst | `bin/ollama` + `lib/ollama/{cuda_v12,cuda_v13,vulkan}/` (66 files) |
| pg/postgres/psql | tar.gz | `psql-{ver}-{triplet}/bin/psql` + `lib/{libpq,libz,libzstd,...}.so` + `include/` (75 files) |
| sass | tar.gz | `dart-sass/sass` (wrapper script) + `dart-sass/src/{dart,sass.snapshot}` |
| syncthing | tar.gz | `syncthing-{triplet}-{ver}/syncthing` + `etc/{systemd,upstart,desktop,runit,...}/` (45 files) |
| xz | tar.gz | `xz-{ver}-{triplet}/xz` + `xz-{ver}-{triplet}/unxz` (two binaries) |

**Install pattern**: extract entire directory tree into opt, symlink binary.

### Pattern E: FHS-like Layout (binary in bin/)

Archive already follows `bin/` + `share/` layout.

| Package | Archive | Layout |
|---------|---------|--------|
| gh | tar.gz | `gh_{ver}_{os}_{arch}/bin/gh` + `share/man/man1/*.1` (129 files of man pages) |
| ollama (Linux) | tar.zst | `bin/ollama` + `lib/ollama/` |

**Install pattern**: extract directly into opt (already correct layout).

### Pattern F: Renamed Binary

Binary in archive doesn't match expected binary name.

| Package | Binary in Archive | Expected Name |
|---------|-------------------|---------------|
| pathman | `pathman-v0.6.0-linux-amd64_v1` | `pathman` |
| yq | `yq_linux_amd64` | `yq` |

### Packages Not Inspected (special/large/known)

| Package | Reason |
|---------|--------|
| node, go, zig, flutter, julia | Well-known SDK layouts — documented elsewhere |
| mariadb | Full database server — complex layout |
| gpg | macOS-only .dmg (GnuPG for OS X) |
| iterm2 | macOS-only .zip (.app bundle) |
| cmake | 61MB, well-known layout (bin/ + share/ + doc/) |
| dashcore | 58MB, cryptocurrency daemon + CLI tools |
| deno | 34MB zip, single binary |
| k9s | 36MB, single binary (goreleaser pattern) |
| mutagen | 56MB, file sync tool |
| pandoc | 33MB, academic document converter (bin/ + share/) |
| pwsh | 74MB, .NET runtime + many DLLs |
| terraform | HashiCorp zip, single binary |
| terramate | 45MB, single binary |
| tinygo | 56MB, compiler toolchain |
| ffmpeg | Special: uses separate bare binary downloads |
| chromedriver | Chrome-versioned zip, single binary |
| arc, atomicparsley, comrak, fish, git, gitea, jq, kind, shfmt | Bare binaries only (no archives to inspect) |
| crabz | Archive contains source code, not binary |
| vim-* plugins | Git clone only, no binary |
| aliasman, serviceman, duckdns.sh | Source-only releases |

### Emerging Patterns

1. **Most common**: Pattern A (bare binary + optional docs in archive) — ~28 packages
2. **Rust tools** tend to use Pattern B/C: subdirectory named `{tool}-{ver}-{triplet}/` with completions
3. **Go tools** (goreleaser-built) tend to use Pattern A: flat archive with binary + LICENSE + README
4. **Completion directory naming varies**: `autocomplete/`, `completions/`, `complete/` — no standard
5. **Completion file naming varies**: `_tool` (zsh), `tool.bash`/`tool.bash-completion`, `tool.fish`
6. **Man page location varies**: `tool.1`, `doc/tool.1`, `manpages/tool.1.gz`, `man/man1/tool.1`
7. **pathman and yq**: need binary rename during install
8. **crabz**: classified archive is source code, not binary — classifier bug or mismatch

---

## Format Changes Over Time

Analyzed all packages for format/OS changes year by year. Most changes are just
adding/removing OS support (e.g. adding linux:.deb). Below are the structurally
significant changes that affect install scripts.

### Packages with Structural Format Changes

| Package | When | What Changed |
|---------|------|-------------|
| sd | 2023 | zip → tar.gz, added completions + man page |
| ollama | 2025–2026 | bare binary → no GitHub release → tar.zst with lib/ |
| caddy | 2019–2020 | v1 (zip) → v2 (tar.gz, different naming) |
| deno | 2020–2021 | .gz (gzipped binary) → .zip |
| fzf | 2021–2025 | darwin: tar.gz → zip → tar.gz |
| gh | 2024 | darwin: tar.gz → .pkg |
| hugo | 2017–2018 | zip → tar.gz (naming also changed) |
| bun | 2022 | restructured archive formats |
| sclient | 2023 | tar.gz → tar.xz |
| watchexec | 2019–2020 | tar.gz → tar.xz |
| pathman | 2023 | tar.gz + zip → tar.gz + tar.xz (dropped windows zip) |

### Packages with Stable Formats (no structural change)

These packages have maintained the same archive structure since their first release.
Only the binary name or OS/arch list changed — install script stays the same.

awless, bat, chromedriver, cilium, cmake, curlie, dashmsg, delta, dotenv,
dotenv-linter, fd, ffuf, gitdeploy, goreleaser, gprox, grype, hexyl,
k9s, keypairs, koji, kubectx, kubens, lf, lsd, monorel, node, ots,
rclone, rg/ripgrep, runzip, sass, shellcheck, sqlc, sqlpkg, sttr,
syncthing, terraform, trip, uuidv7, xcaddy, xsv, xz, yq, zig, zoxide

### Per-Package Format Change Notes

**sd**: v0.6.5 and earlier used zip with bare binary in triplet directory.
v1.0.0+ uses tar.gz with binary + completions + man page. Install script
needs two eras.

**ollama**: Three distinct eras on Linux:
1. v0.1.0–v0.3.6: bare binary (self-contained, 300-400MB)
2. v0.4.0–v0.13.x: no GitHub release (use official install script)
3. v0.14.0+: tar.zst with `bin/ollama` + `lib/ollama/` (1.7GB)
Darwin always has .zip with Ollama.app; bare CLI binary dropped at v0.5.8.

**caddy**: v1 used simple `caddy_{os}_{arch}.tar.gz`. v2 (2020+) uses
`caddy_{ver}_{os}_{arch}.tar.gz` with same flat structure. Internal layout
didn't actually change — just the naming convention.

**deno**: Switched from `deno-{triplet}.gz` (gzipped bare binary) to
`deno-{triplet}.zip` in 2020-2021. Both contain just the binary.

**gh**: Darwin switched from tar.gz to .pkg in 2024. Linux tar.gz structure
(FHS-like with bin/ and share/man/) stayed the same.

**hugo**: Early releases (2014) were bare binaries. Switched to zip (2014-2017),
then tar.gz (2017+). Internal structure always been flat (hugo, LICENSE, README).
