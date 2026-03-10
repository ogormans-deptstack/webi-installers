package classify_test

import (
	"testing"

	"github.com/webinstall/webi-installers/internal/buildmeta"
	"github.com/webinstall/webi-installers/internal/classify"
)

func TestFilename(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantOS buildmeta.OS
		arch   buildmeta.Arch
		libc   buildmeta.Libc
		format buildmeta.Format
	}{
		// Goreleaser-style
		{
			name:   "goreleaser linux amd64 tar.gz",
			input:  "hugo_0.145.0_linux-amd64.tar.gz",
			wantOS: buildmeta.OSLinux,
			arch:   buildmeta.ArchAMD64,
			format: buildmeta.FormatTarGz,
		},
		{
			name:   "goreleaser darwin arm64 tar.gz",
			input:  "hugo_0.145.0_darwin-arm64.tar.gz",
			wantOS: buildmeta.OSDarwin,
			arch:   buildmeta.ArchARM64,
			format: buildmeta.FormatTarGz,
		},
		{
			name:   "goreleaser windows amd64 zip",
			input:  "hugo_0.145.0_windows-amd64.zip",
			wantOS: buildmeta.OSWindows,
			arch:   buildmeta.ArchAMD64,
			format: buildmeta.FormatZip,
		},
		{
			name:   "goreleaser freebsd",
			input:  "hugo_0.145.0_freebsd-amd64.tar.gz",
			wantOS: buildmeta.OSFreeBSD,
			arch:   buildmeta.ArchAMD64,
			format: buildmeta.FormatTarGz,
		},

		// Rust/cargo-dist style
		{
			name:   "rust linux musl",
			input:  "ripgrep-14.1.1-x86_64-unknown-linux-musl.tar.gz",
			wantOS: buildmeta.OSLinux,
			arch:   buildmeta.ArchAMD64,
			libc:   buildmeta.LibcMusl,
			format: buildmeta.FormatTarGz,
		},
		{
			name:   "rust linux gnu",
			input:  "bat-v0.24.0-x86_64-unknown-linux-gnu.tar.gz",
			wantOS: buildmeta.OSLinux,
			arch:   buildmeta.ArchAMD64,
			libc:   buildmeta.LibcGNU,
			format: buildmeta.FormatTarGz,
		},
		{
			name:   "rust apple darwin",
			input:  "ripgrep-14.1.1-x86_64-apple-darwin.tar.gz",
			wantOS: buildmeta.OSDarwin,
			arch:   buildmeta.ArchAMD64,
			format: buildmeta.FormatTarGz,
		},
		{
			name:   "rust windows msvc",
			input:  "bat-v0.24.0-x86_64-pc-windows-msvc.zip",
			wantOS: buildmeta.OSWindows,
			arch:   buildmeta.ArchAMD64,
			libc:   buildmeta.LibcMSVC,
			format: buildmeta.FormatZip,
		},
		{
			name:   "rust aarch64 linux",
			input:  "ripgrep-14.1.1-aarch64-unknown-linux-gnu.tar.gz",
			wantOS: buildmeta.OSLinux,
			arch:   buildmeta.ArchARM64,
			libc:   buildmeta.LibcGNU,
			format: buildmeta.FormatTarGz,
		},

		// Zig-style
		{
			name:   "zig linux x86_64",
			input:  "zig-linux-x86_64-0.14.0.tar.xz",
			wantOS: buildmeta.OSLinux,
			arch:   buildmeta.ArchAMD64,
			format: buildmeta.FormatTarXz,
		},
		{
			name:   "zig macos aarch64",
			input:  "zig-macos-aarch64-0.14.0.tar.xz",
			wantOS: buildmeta.OSDarwin,
			arch:   buildmeta.ArchARM64,
			format: buildmeta.FormatTarXz,
		},

		// Windows executables
		{
			name:   "bare exe",
			input:  "jq-windows-amd64.exe",
			wantOS: buildmeta.OSWindows,
			arch:   buildmeta.ArchAMD64,
			format: buildmeta.FormatExe,
		},
		{
			name:   "msi installer",
			input:  "caddy_2.9.0_windows_amd64.msi",
			wantOS: buildmeta.OSWindows,
			arch:   buildmeta.ArchAMD64,
			format: buildmeta.FormatMSI,
		},

		// macOS formats
		{
			name:   "dmg installer",
			input:  "MyApp-1.0.0-darwin-arm64.dmg",
			wantOS: buildmeta.OSDarwin,
			arch:   buildmeta.ArchARM64,
			format: buildmeta.FormatDMG,
		},

		// Arch priority: x86_64 must not match x86
		{
			name:   "x86_64 not x86",
			input:  "tool-x86_64-linux.tar.gz",
			wantOS: buildmeta.OSLinux,
			arch:   buildmeta.ArchAMD64,
			format: buildmeta.FormatTarGz,
		},
		{
			name:   "actual x86",
			input:  "tool-x86-linux.tar.gz",
			wantOS: buildmeta.OSLinux,
			arch:   buildmeta.ArchX86,
			format: buildmeta.FormatTarGz,
		},
		{
			name:   "i386",
			input:  "tool-linux-i386.tar.gz",
			wantOS: buildmeta.OSLinux,
			arch:   buildmeta.ArchX86,
			format: buildmeta.FormatTarGz,
		},

		// ARM variants: arm64 must not match armv7/armv6
		{
			name: "aarch64 not armv7",
			input: "tool-aarch64-linux.tar.gz",
			arch:  buildmeta.ArchARM64,
		},
		{
			name: "armv7",
			input: "tool-armv7l-linux.tar.gz",
			arch:  buildmeta.ArchARMv7,
		},
		{
			name: "armv6",
			input: "tool-armv6l-linux.tar.gz",
			arch:  buildmeta.ArchARMv6,
		},

		// ppc64le before ppc64
		{
			name: "ppc64le",
			input: "tool-linux-ppc64le.tar.gz",
			arch:  buildmeta.ArchPPC64LE,
		},
		{
			name: "ppc64",
			input: "tool-linux-ppc64.tar.gz",
			arch:  buildmeta.ArchPPC64,
		},

		// Static linking
		{
			name: "static binary",
			input: "tool-linux-amd64-static.tar.gz",
			libc:  buildmeta.LibcNone,
		},

		// .exe implies Windows
		{
			name:   "exe implies windows",
			input:  "tool-amd64.exe",
			wantOS: buildmeta.OSWindows,
			arch:   buildmeta.ArchAMD64,
			format: buildmeta.FormatExe,
		},

		// Compound extensions
		{
			name:   "tar.zst",
			input:  "tool-linux-amd64.tar.zst",
			format: buildmeta.FormatTarZst,
		},
		{
			name:   "exe.xz",
			input:  "tool-windows-amd64.exe.xz",
			format: buildmeta.FormatExeXz,
		},
		{
			name:   "app.zip",
			input:  "MyApp-1.0.0.app.zip",
			format: buildmeta.FormatAppZip,
		},
		{
			name:   "tgz alias",
			input:  "tool-linux-amd64.tgz",
			format: buildmeta.FormatTarGz,
		},

		// s390x, mips
		{
			name: "s390x",
			input: "tool-linux-s390x.tar.gz",
			arch:  buildmeta.ArchS390X,
		},
		{
			name: "mips64",
			input: "tool-linux-mips64.tar.gz",
			arch:  buildmeta.ArchMIPS64,
		},

		// Unknown / no match
		{
			name:  "checksum file",
			input: "checksums.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classify.Filename(tt.input)
			if tt.wantOS != "" && got.OS != tt.wantOS {
				t.Errorf("OS = %q, want %q", got.OS, tt.wantOS)
			}
			if tt.arch != "" && got.Arch != tt.arch {
				t.Errorf("Arch = %q, want %q", got.Arch, tt.arch)
			}
			if tt.libc != "" && got.Libc != tt.libc {
				t.Errorf("Libc = %q, want %q", got.Libc, tt.libc)
			}
			if tt.format != "" && got.Format != tt.format {
				t.Errorf("Format = %q, want %q", got.Format, tt.format)
			}
		})
	}
}
