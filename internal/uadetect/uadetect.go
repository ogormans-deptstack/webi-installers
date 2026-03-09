// Package uadetect identifies the requesting system's OS, CPU architecture,
// and libc from its User-Agent string.
//
// Webi's bootstrap scripts send "$(uname -srm)" as the User-Agent, e.g.
// "Darwin 23.1.0 arm64" or "Linux 6.1.0 x86_64". This package parses those
// into [buildmeta.OS], [buildmeta.Arch], and [buildmeta.Libc] values so the
// server can select the correct release artifact.
//
// Also handles non-uname agents like "PowerShell/7.3.0" and "MS AMD64".
package uadetect

import (
	"strings"

	"github.com/webinstall/webi-installers/internal/buildmeta"
)

// Result holds the detected platform info from a User-Agent string.
type Result struct {
	OS   buildmeta.OS
	Arch buildmeta.Arch
	Libc buildmeta.Libc
}

// Parse extracts OS, arch, and libc from a User-Agent string.
func Parse(ua string) Result {
	if ua == "-" {
		return Result{}
	}

	tokens := tokenize(ua)

	return Result{
		OS:   matchOS(tokens),
		Arch: matchArch(tokens),
		Libc: matchLibc(tokens),
	}
}

// tokenize splits a User-Agent into lowercase tokens for matching.
// Splits on whitespace, '/', and ';', since UAs come in various forms:
//
//	"Darwin 23.1.0 arm64"                    (uname -srm)
//	"PowerShell/7.3.0"                       (PowerShell)
//	"MS AMD64"                               (Windows shorthand)
//	"Macintosh; Intel Mac OS X 10_15_7"      (browser)
func tokenize(ua string) []string {
	// Strip xnu kernel info that can mislead arch detection under Rosetta.
	// "xnu-7195.60.75~1/RELEASE_ARM64_T8101" contains ARM64 even when
	// running as x86_64. This only appears in verbose uname output.
	if i := strings.Index(ua, "xnu-"); i >= 0 {
		end := strings.IndexByte(ua[i:], ' ')
		if end < 0 {
			ua = ua[:i]
		} else {
			ua = ua[:i] + ua[i+end:]
		}
	}

	return strings.FieldsFunc(strings.ToLower(ua), func(r rune) bool {
		return r == ' ' || r == '/' || r == ';' || r == '\t'
	})
}

// matchOS identifies the operating system from tokens.
// Order matters: Android before Linux, Linux before Windows (for WSL).
func matchOS(tokens []string) buildmeta.OS {
	has := func(s string) bool {
		for _, t := range tokens {
			if strings.Contains(t, s) {
				return true
			}
		}
		return false
	}

	// Android must be checked before Linux.
	if has("android") {
		return buildmeta.OSAndroid
	}

	if has("darwin") || has("macos") || has("macintosh") || has("iphone") || has("ios") || has("ipad") {
		return buildmeta.OSDarwin
	}
	// "mac" alone (not in "macintosh" which is already matched)
	for _, t := range tokens {
		if t == "mac" {
			return buildmeta.OSDarwin
		}
	}

	// Linux before Windows because WSL UAs contain both "linux" and "microsoft".
	// But exclude Cygwin/msysgit which report Linux-like strings on Windows.
	if has("linux") && !has("cygwin") && !has("msysgit") {
		return buildmeta.OSLinux
	}

	if has("windows") || has("win32") || has("microsoft") || has("powershell") {
		return buildmeta.OSWindows
	}
	for _, t := range tokens {
		if t == "ms" || t == "win" {
			return buildmeta.OSWindows
		}
	}

	// Fallback: curl and wget imply a POSIX system, almost always Linux.
	if has("curl") || has("wget") {
		return buildmeta.OSLinux
	}

	return ""
}

// matchArch identifies the CPU architecture from tokens.
// More specific patterns are checked before less specific ones.
func matchArch(tokens []string) buildmeta.Arch {
	has := func(s string) bool {
		for _, t := range tokens {
			if strings.Contains(t, s) {
				return true
			}
		}
		return false
	}
	exact := func(s string) bool {
		for _, t := range tokens {
			if t == s {
				return true
			}
		}
		return false
	}

	// ARM 64-bit (most specific first)
	if has("aarch64") || has("arm64") || has("armv8") {
		return buildmeta.ArchARM64
	}

	// ARM 32-bit variants
	if has("armv7") || has("arm32") {
		return buildmeta.ArchARMv7
	}
	if has("armv6") {
		return buildmeta.ArchARMv6
	}
	// Bare "arm" without a version qualifier → armv6 (conservative).
	if exact("arm") {
		return buildmeta.ArchARMv6
	}

	// POWER (check before generic 64-bit)
	if has("ppc64le") {
		return buildmeta.ArchPPC64LE
	}
	if has("ppc64") {
		return buildmeta.ArchPPC64
	}

	// MIPS (check before generic 64-bit)
	if has("mips64") {
		return buildmeta.ArchMIPS64
	}
	if has("mips") {
		return buildmeta.ArchMIPS
	}

	// x86-64
	if has("x86_64") || has("amd64") || exact("x64") {
		return buildmeta.ArchAMD64
	}

	// x86 32-bit (after x86_64 to avoid false match)
	if has("i386") || has("i686") || exact("x86") {
		return buildmeta.ArchX86
	}

	return ""
}

// matchLibc identifies the C library from tokens.
func matchLibc(tokens []string) buildmeta.Libc {
	has := func(s string) bool {
		for _, t := range tokens {
			if strings.Contains(t, s) {
				return true
			}
		}
		return false
	}

	if has("musl") {
		return buildmeta.LibcMusl
	}
	if has("msvc") || has("windows") || has("microsoft") {
		return buildmeta.LibcMSVC
	}
	if has("gnu") || has("glibc") || has("linux") {
		return buildmeta.LibcGNU
	}

	return buildmeta.LibcNone
}
