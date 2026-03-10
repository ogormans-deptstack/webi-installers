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
