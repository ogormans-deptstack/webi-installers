# Packages Without releases.conf — Categorization

Reference for which packages still need conf files and which don't.

## Directory Symlinks (no conf needed — share with target)

These are directory-level symlinks. They automatically share whatever
releases.conf their target has.

- `msvc-runtime` → `vcruntime`
- `msvcruntime` → `vcruntime`
- `rust.vim` → `vim-rust`
- `vc-redist` → `vcruntime`
- `vc-runtime` → `vcruntime`
- `vc_redist` → `vcruntime`
- `vcredist` → `vcruntime`
- `vcruntime140` → `vcruntime`
- `vim-essential` → `vim-essentials`
- `vim-mouse` → `vim-gui`
- `vps-myip` → `myip`
- `xcode-cli` → `commandlinetools`

## Infrastructure (10 — skip)

`_cache`, `_common`, `_examples`, `_npm`, `_scripts`, `_vim-example`, `_webi`,
`cmd`, `internal`, `test`

## No Upstream Releases — Config/Script-Only (no conf needed)

System tools:
`brew`, `commandlinetools`, `sudo`, `wsl`, `wsl1`, `wsl2`, `setcap-netbind`

Git/SSH config:
`git-config-gpg`, `git-gpg-init`, `gpg-pubkey`, `ssh-adduser`, `ssh-authorize`,
`ssh-harden`, `ssh-prohibit-password`, `ssh-pubkey`, `ssh-setpass`, `ssh-utils`,
`sshd-prohibit-password`

Meta/essentials bundles:
`beyond-shell`, `go-essentials`, `pwsh-essentials`, `vim-essentials`,
`webi`, `webi-essentials`, `vps-utils`

Vim config (no releases, just .vim settings files):
`vim-beyondcode` (meta-package), `vim-gui`, `vim-italics`, `vim-lastplace`,
`vim-leader`, `vim-shell`, `vim-smartcase`, `vim-spell`, `vim-viminfo`,
`vim-whitespace`

iTerm config:
`iterm-color-schemes`, `iterm-themes`, `iterm-utils`,
`iterm2-color-schemes`, `iterm2-themes`, `iterm2-utils`

VPS scripts:
`vps-addswap`, `myip`

npm wrappers (installed via npm, not binary releases):
`jshint`, `prettier`, `redis-commander`

No-release installers (use rustup, pyenv, system package managers, etc.):
`rustlang`, `rust` (alias→rustlang), `pyenv`, `python`, `python2`, `python3`

Other config/wrappers:
`duckdns`, `archiver`, `dashcore-utils`, `psscriptanalyzer`,
`vcruntime` (Windows-only, no binary releases to track),
`nerdfont`, `nerd-font`, `nerd-fonts`, `nerdfonts` (hardcoded font download)

## Aliases with `alias_of` conf (DONE)

- `gnupg` → `gpg`
- `iterm` → `iterm2`
- `mariadb-server` → `mariadb`
- `mariadbd` → `mariadb`
- `postgres-client` → `psql`
- `postgresql` → `postgres`
- `postgresql-client` → `psql`
- `powershell` → `pwsh`
- `ziglang` → `zig`
- `trippy` → `trip`
- `golang` → `go`
- `dashd` → `dashcore`
- `zig.vim` → `vim-zig`

## Reserved/Ambiguous (no conf — intentionally not aliases)

- `mysql` — reserved, prints "did you mean mariadb?"
- `mysqld` — reserved, prints "did you mean mariadb?"

## Vim Plugins with gittag conf (DONE)

- `vim-airline` — vim-airline/vim-airline
- `vim-airline-themes` — vim-airline/vim-airline-themes
- `vim-ale` — dense-analysis/ale
- `vim-commentary` — tpope/vim-commentary
- `vim-devicons` — ryanoasis/vim-devicons
- `vim-go` — fatih/vim-go
- `vim-nerdtree` — preservim/nerdtree
- `vim-prettier` — prettier/vim-prettier
- `vim-rust` — rust-lang/rust.vim
- `vim-sensible` — tpope/vim-sensible
- `vim-shfmt` — z0mbix/vim-shfmt
- `vim-syntastic` — vim-syntastic/syntastic
- `vim-zig` — ziglang/zig.vim

## Dead

- `macos` — dead project (confirmed by user)
