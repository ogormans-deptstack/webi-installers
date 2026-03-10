package platlatest_test

import (
	"path/filepath"
	"testing"

	"github.com/webinstall/webi-installers/internal/buildmeta"
	"github.com/webinstall/webi-installers/internal/platlatest"
)

var (
	linuxAMD64 = buildmeta.Target{
		OS: buildmeta.OSLinux, Arch: buildmeta.ArchAMD64, Libc: buildmeta.LibcGNU,
	}
	darwinARM64 = buildmeta.Target{
		OS: buildmeta.OSDarwin, Arch: buildmeta.ArchARM64, Libc: buildmeta.LibcNone,
	}
	windowsAMD64 = buildmeta.Target{
		OS: buildmeta.OSWindows, Arch: buildmeta.ArchAMD64, Libc: buildmeta.LibcMSVC,
	}
)

func TestSetAndGet(t *testing.T) {
	p := filepath.Join(t.TempDir(), "latest.json")
	idx, err := platlatest.Open(p)
	if err != nil {
		t.Fatal(err)
	}

	if got := idx.Get(linuxAMD64); got != "" {
		t.Errorf("Get before Set = %q, want empty", got)
	}

	idx.Set(linuxAMD64, "v0.145.0")
	idx.Set(darwinARM64, "v0.144.1")
	idx.Set(windowsAMD64, "v0.143.0")

	if got := idx.Get(linuxAMD64); got != "v0.145.0" {
		t.Errorf("linux = %q, want v0.145.0", got)
	}
	if got := idx.Get(darwinARM64); got != "v0.144.1" {
		t.Errorf("darwin = %q, want v0.144.1", got)
	}
	if got := idx.Get(windowsAMD64); got != "v0.143.0" {
		t.Errorf("windows = %q, want v0.143.0", got)
	}
}

func TestSaveAndReload(t *testing.T) {
	p := filepath.Join(t.TempDir(), "latest.json")

	idx1, err := platlatest.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	idx1.Set(linuxAMD64, "v0.145.0")
	idx1.Set(darwinARM64, "v0.144.1")
	if err := idx1.Save(); err != nil {
		t.Fatal(err)
	}

	// Reload from disk.
	idx2, err := platlatest.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := idx2.Get(linuxAMD64); got != "v0.145.0" {
		t.Errorf("after reload: linux = %q, want v0.145.0", got)
	}
	if got := idx2.Get(darwinARM64); got != "v0.144.1" {
		t.Errorf("after reload: darwin = %q, want v0.144.1", got)
	}
}

func TestAll(t *testing.T) {
	p := filepath.Join(t.TempDir(), "latest.json")
	idx, err := platlatest.Open(p)
	if err != nil {
		t.Fatal(err)
	}

	idx.Set(linuxAMD64, "v1.0.0")
	idx.Set(darwinARM64, "v0.9.0")

	all := idx.All()
	if len(all) != 2 {
		t.Fatalf("All() returned %d entries, want 2", len(all))
	}
	if all[linuxAMD64.Triplet()] != "v1.0.0" {
		t.Error("missing linux entry")
	}
}

func TestResolveArchFallback(t *testing.T) {
	p := filepath.Join(t.TempDir(), "latest.json")
	idx, err := platlatest.Open(p)
	if err != nil {
		t.Fatal(err)
	}

	// v1.0.0 had per-microarch builds.
	idx.Set(buildmeta.Target{OS: buildmeta.OSLinux, Arch: buildmeta.ArchAMD64v4, Libc: buildmeta.LibcGNU}, "v1.0.0")
	idx.Set(buildmeta.Target{OS: buildmeta.OSLinux, Arch: buildmeta.ArchAMD64v3, Libc: buildmeta.LibcGNU}, "v1.0.0")
	idx.Set(buildmeta.Target{OS: buildmeta.OSLinux, Arch: buildmeta.ArchAMD64v2, Libc: buildmeta.LibcGNU}, "v1.0.0")
	idx.Set(buildmeta.Target{OS: buildmeta.OSLinux, Arch: buildmeta.ArchAMD64, Libc: buildmeta.LibcGNU}, "v1.0.0")

	// v2.0.0 dropped microarch, ships only baseline amd64.
	idx.Set(buildmeta.Target{OS: buildmeta.OSLinux, Arch: buildmeta.ArchAMD64, Libc: buildmeta.LibcGNU}, "v2.0.0")

	// An amd64v4 machine should get v2.0.0 (latest) via baseline fallback,
	// not v1.0.0 (which had a specific v4 build).
	want := buildmeta.Target{OS: buildmeta.OSLinux, Arch: buildmeta.ArchAMD64v4, Libc: buildmeta.LibcGNU}
	ver, matched := idx.Resolve(want)
	if ver != "v2.0.0" {
		t.Errorf("Resolve(amd64v4) version = %q, want v2.0.0", ver)
	}
	if matched.Arch != buildmeta.ArchAMD64 {
		t.Errorf("Resolve(amd64v4) matched arch = %q, want %q", matched.Arch, buildmeta.ArchAMD64)
	}
}

func TestResolveExactMatchPreferred(t *testing.T) {
	p := filepath.Join(t.TempDir(), "latest.json")
	idx, err := platlatest.Open(p)
	if err != nil {
		t.Fatal(err)
	}

	// Both amd64v3 and baseline have the same latest version.
	idx.Set(buildmeta.Target{OS: buildmeta.OSLinux, Arch: buildmeta.ArchAMD64v3, Libc: buildmeta.LibcGNU}, "v2.0.0")
	idx.Set(buildmeta.Target{OS: buildmeta.OSLinux, Arch: buildmeta.ArchAMD64, Libc: buildmeta.LibcGNU}, "v2.0.0")

	// When versions are equal, the more specific arch should win
	// (it appears first in the fallback chain).
	want := buildmeta.Target{OS: buildmeta.OSLinux, Arch: buildmeta.ArchAMD64v3, Libc: buildmeta.LibcGNU}
	ver, matched := idx.Resolve(want)
	if ver != "v2.0.0" {
		t.Errorf("version = %q, want v2.0.0", ver)
	}
	if matched.Arch != buildmeta.ArchAMD64v3 {
		t.Errorf("matched arch = %q, want %q (more specific)", matched.Arch, buildmeta.ArchAMD64v3)
	}
}

func TestResolveLibcFallback(t *testing.T) {
	p := filepath.Join(t.TempDir(), "latest.json")
	idx, err := platlatest.Open(p)
	if err != nil {
		t.Fatal(err)
	}

	// Only a static (LibcNone) build exists.
	idx.Set(buildmeta.Target{OS: buildmeta.OSLinux, Arch: buildmeta.ArchAMD64, Libc: buildmeta.LibcNone}, "v1.0.0")

	// A glibc machine should find it via libc fallback.
	want := buildmeta.Target{OS: buildmeta.OSLinux, Arch: buildmeta.ArchAMD64, Libc: buildmeta.LibcGNU}
	ver, matched := idx.Resolve(want)
	if ver != "v1.0.0" {
		t.Errorf("version = %q, want v1.0.0", ver)
	}
	if matched.Libc != buildmeta.LibcNone {
		t.Errorf("matched libc = %q, want %q", matched.Libc, buildmeta.LibcNone)
	}
}

func TestResolveNoMatch(t *testing.T) {
	p := filepath.Join(t.TempDir(), "latest.json")
	idx, err := platlatest.Open(p)
	if err != nil {
		t.Fatal(err)
	}

	idx.Set(linuxAMD64, "v1.0.0")

	// Darwin target should not match a Linux entry.
	ver, _ := idx.Resolve(darwinARM64)
	if ver != "" {
		t.Errorf("Resolve(darwin) = %q, want empty (no match)", ver)
	}
}

func TestResolveBaselineOnly(t *testing.T) {
	p := filepath.Join(t.TempDir(), "latest.json")
	idx, err := platlatest.Open(p)
	if err != nil {
		t.Fatal(err)
	}

	// amd64v1 machine can't run v2+ binaries.
	idx.Set(buildmeta.Target{OS: buildmeta.OSLinux, Arch: buildmeta.ArchAMD64v2, Libc: buildmeta.LibcGNU}, "v2.0.0")
	idx.Set(buildmeta.Target{OS: buildmeta.OSLinux, Arch: buildmeta.ArchAMD64, Libc: buildmeta.LibcGNU}, "v1.0.0")

	// Baseline machine gets v1.0.0 — it can't run v2's amd64v2 binary.
	want := buildmeta.Target{OS: buildmeta.OSLinux, Arch: buildmeta.ArchAMD64, Libc: buildmeta.LibcGNU}
	ver, _ := idx.Resolve(want)
	if ver != "v1.0.0" {
		t.Errorf("Resolve(amd64 baseline) = %q, want v1.0.0", ver)
	}
}

func TestOpenNonexistent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "does-not-exist.json")
	idx, err := platlatest.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	// Should be empty, not nil.
	if all := idx.All(); len(all) != 0 {
		t.Errorf("new index should be empty, got %v", all)
	}
}
