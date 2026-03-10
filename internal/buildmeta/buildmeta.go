// Package buildmeta is the shared vocabulary for Webi's build targets.
//
// Every package that deals with OS, architecture, libc, archive format, or
// release channel imports these types instead of passing raw strings. This
// prevents typos like "darwn" from compiling and gives a single place to
// enumerate what Webi supports.
package buildmeta

// OS represents a target operating system.
type OS string

const (
	OSAny     OS = "ANYOS"
	OSDarwin  OS = "darwin"
	OSLinux   OS = "linux"
	OSWindows OS = "windows"
	OSFreeBSD OS = "freebsd"
	OSSunOS   OS = "sunos"
	OSAIX     OS = "aix"
	OSAndroid OS = "android"

	// POSIX compatibility levels — used when a package is a shell script
	// or otherwise OS-independent for POSIX systems.
	OSPosix2017 OS = "posix_2017"
	OSPosix2024 OS = "posix_2024"
)

// Arch represents a target CPU architecture.
type Arch string

const (
	ArchAny     Arch = "ANYARCH"
	ArchAMD64   Arch = "x86_64"    // baseline (v1)
	ArchAMD64v2 Arch = "x86_64_v2" // +SSE4, +POPCNT, etc.
	ArchAMD64v3 Arch = "x86_64_v3" // +AVX2, +BMI, etc.
	ArchAMD64v4 Arch = "x86_64_v4" // +AVX-512
	ArchARM64   Arch = "aarch64"
	ArchARMv7   Arch = "armv7"
	ArchARMv6   Arch = "armv6"
	ArchX86     Arch = "x86"
	ArchPPC64LE Arch = "ppc64le"
	ArchPPC64   Arch = "ppc64"
	ArchS390X   Arch = "s390x"
	ArchMIPS64  Arch = "mips64"
	ArchMIPS    Arch = "mips"
)

// Libc represents the C library a binary is linked against.
type Libc string

const (
	LibcNone Libc = "none" // statically linked or no libc dependency (Go, Zig, etc.)
	LibcGNU  Libc = "gnu"  // requires glibc (most Linux distros)
	LibcMusl Libc = "musl" // requires musl (Alpine, some Docker images)
	LibcMSVC Libc = "msvc" // Microsoft Visual C++ runtime
)

// Format represents an archive or package format.
type Format string

const (
	FormatTarGz  Format = ".tar.gz"
	FormatTarXz  Format = ".tar.xz"
	FormatTarZst Format = ".tar.zst"
	FormatZip    Format = ".zip"
	FormatGz     Format = ".gz"
	FormatXz     Format = ".xz"
	FormatZst    Format = ".zst"
	FormatExe    Format = ".exe"
	FormatExeXz  Format = ".exe.xz"
	FormatMSI    Format = ".msi"
	FormatDMG    Format = ".dmg"
	FormatPkg    Format = ".pkg"
	FormatAppZip Format = ".app.zip"
	Format7z     Format = ".7z"
	FormatSh     Format = ".sh"
	FormatGit    Format = ".git"
)

// Channel represents a release stability channel.
type Channel string

const (
	ChannelStable  Channel = "stable"
	ChannelLatest  Channel = "latest"
	ChannelRC      Channel = "rc"
	ChannelPreview Channel = "preview"
	ChannelBeta    Channel = "beta"
	ChannelAlpha   Channel = "alpha"
	ChannelDev     Channel = "dev"
)

// Target represents a fully resolved build target.
type Target struct {
	OS   OS
	Arch Arch
	Libc Libc
}

// Triplet returns the canonical "os-arch-libc" string.
func (t Target) Triplet() string {
	return string(t.OS) + "-" + string(t.Arch) + "-" + string(t.Libc)
}

// ArchFallbacks returns the architectures that a machine with the given
// arch can run, ordered from most specific to least. The input arch is
// always first. Returns nil for unknown architectures.
//
// For example, an amd64v4 machine can run v4, v3, v2, and baseline (v1)
// binaries. An armv7 machine can run armv7 and armv6 binaries.
func ArchFallbacks(arch Arch) []Arch {
	switch arch {
	case ArchAMD64v4:
		return []Arch{ArchAMD64v4, ArchAMD64v3, ArchAMD64v2, ArchAMD64}
	case ArchAMD64v3:
		return []Arch{ArchAMD64v3, ArchAMD64v2, ArchAMD64}
	case ArchAMD64v2:
		return []Arch{ArchAMD64v2, ArchAMD64}
	case ArchARMv7:
		return []Arch{ArchARMv7, ArchARMv6}
	default:
		// No fallback chain — exact match only.
		return []Arch{arch}
	}
}

// LibcFallbacks returns the libc variants a machine can use, ordered
// by preference. A musl system can only run musl or static binaries.
// A glibc system can only run glibc or static binaries.
func LibcFallbacks(libc Libc) []Libc {
	switch libc {
	case LibcGNU:
		return []Libc{LibcGNU, LibcNone}
	case LibcMusl:
		return []Libc{LibcMusl, LibcNone}
	case LibcMSVC:
		return []Libc{LibcMSVC, LibcNone}
	default:
		return []Libc{libc}
	}
}
