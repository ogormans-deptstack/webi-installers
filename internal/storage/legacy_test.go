package storage_test

import (
	"encoding/json"
	"testing"

	"github.com/webinstall/webi-installers/internal/storage"
)

// TestExportLegacyDrops verifies that ExportLegacy correctly drops and counts
// assets that can't be represented in the Node.js legacy cache format.
func TestExportLegacyDrops(t *testing.T) {
	t.Run("variant_builds_dropped", func(t *testing.T) {
		// Assets with variant tags (rocm, installer, fxdependent, etc.) are
		// dropped because Node.js has no variant-selection logic.
		pd := storage.PackageData{
			Assets: []storage.Asset{
				{Filename: "ollama-linux-amd64-rocm.tgz", OS: "linux", Arch: "x86_64", Format: ".tar.gz", Variants: []string{"rocm"}},
				{Filename: "ollama-linux-amd64.tgz", OS: "linux", Arch: "x86_64", Format: ".tar.gz"},
			},
		}
		lc, stats := storage.ExportLegacy("ollama", pd)
		if stats.Variants != 1 {
			t.Errorf("Variants dropped = %d, want 1", stats.Variants)
		}
		if len(lc.Releases) != 1 {
			t.Errorf("releases = %d, want 1 (baseline only)", len(lc.Releases))
		}
		if lc.Releases[0].Name != "ollama-linux-amd64.tgz" {
			t.Errorf("kept wrong release: %q", lc.Releases[0].Name)
		}
	})

	t.Run("unknown_formats_dropped", func(t *testing.T) {
		// .apk, .AppImage, .deb, .rpm are not in the Node.js format set.
		pd := storage.PackageData{
			Assets: []storage.Asset{
				{Filename: "tool.apk", OS: "android", Format: ".apk"},
				{Filename: "tool.AppImage", OS: "linux", Format: ".AppImage"},
				{Filename: "tool.deb", OS: "linux", Format: ".deb"},
				{Filename: "tool.rpm", OS: "linux", Format: ".rpm"},
				{Filename: "tool-linux-amd64.tar.gz", OS: "linux", Arch: "x86_64", Format: ".tar.gz"},
			},
		}
		lc, stats := storage.ExportLegacy("tool", pd)
		if stats.Formats != 4 {
			t.Errorf("Formats dropped = %d, want 4", stats.Formats)
		}
		if len(lc.Releases) != 1 {
			t.Errorf("releases = %d, want 1 (tar.gz only)", len(lc.Releases))
		}
	})

	t.Run("empty_format_passes_through", func(t *testing.T) {
		// Assets with empty format (e.g. bare binaries, git sources) pass through.
		pd := storage.PackageData{
			Assets: []storage.Asset{
				{Filename: "jq-linux-amd64", OS: "linux", Arch: "x86_64", Format: ""},
			},
		}
		lc, stats := storage.ExportLegacy("jq", pd)
		if stats.Formats != 0 {
			t.Errorf("Formats dropped = %d, want 0", stats.Formats)
		}
		if len(lc.Releases) != 1 {
			t.Errorf("releases = %d, want 1", len(lc.Releases))
		}
	})
}

// TestExportLegacyTranslations verifies that legacyFieldBackport applies the
// correct field translations for Node.js compatibility. Translations are lossy
// in canonical terms but necessary for the legacy resolver.
func TestExportLegacyTranslations(t *testing.T) {
	t.Run("solaris_translated_to_sunos", func(t *testing.T) {
		// Node.js only knows "sunos" for Sun/Oracle platforms.
		pd := storage.PackageData{
			Assets: []storage.Asset{
				{Filename: "go1.20.1.solaris-amd64.tar.gz", OS: "solaris", Arch: "x86_64", Format: ".tar.gz"},
			},
		}
		lc, _ := storage.ExportLegacy("go", pd)
		if len(lc.Releases) != 1 {
			t.Fatalf("releases = %d, want 1", len(lc.Releases))
		}
		if lc.Releases[0].OS != "sunos" {
			t.Errorf("OS = %q, want %q", lc.Releases[0].OS, "sunos")
		}
	})

	t.Run("illumos_translated_to_sunos", func(t *testing.T) {
		pd := storage.PackageData{
			Assets: []storage.Asset{
				{Filename: "caddy_2.9.0_illumos_amd64.tar.gz", OS: "illumos", Arch: "x86_64", Format: ".tar.gz"},
			},
		}
		lc, _ := storage.ExportLegacy("caddy", pd)
		if len(lc.Releases) != 1 {
			t.Fatalf("releases = %d, want 1", len(lc.Releases))
		}
		if lc.Releases[0].OS != "sunos" {
			t.Errorf("OS = %q, want %q", lc.Releases[0].OS, "sunos")
		}
	})

	t.Run("ffmpeg_windows_gz_to_exe", func(t *testing.T) {
		// ffmpeg Windows releases are .gz archives containing a bare .exe.
		// Production releases.js overrides ext to 'exe' for install compatibility.
		pd := storage.PackageData{
			Assets: []storage.Asset{
				{Filename: "ffmpeg-7.0-windows-amd64.gz", OS: "windows", Arch: "x86_64", Format: ".gz"},
				{Filename: "ffmpeg-7.0-linux-amd64.tar.gz", OS: "linux", Arch: "x86_64", Format: ".tar.gz"},
			},
		}
		lc, _ := storage.ExportLegacy("ffmpeg", pd)
		if len(lc.Releases) != 2 {
			t.Fatalf("releases = %d, want 2", len(lc.Releases))
		}
		var windowsExt string
		for _, r := range lc.Releases {
			if r.OS == "windows" {
				windowsExt = r.Ext
			}
		}
		if windowsExt != ".exe" {
			t.Errorf("ffmpeg windows ext = %q, want %q", windowsExt, ".exe")
		}
	})

	t.Run("ffmpeg_translation_not_applied_to_other_packages", func(t *testing.T) {
		// The ffmpeg .gz→.exe translation is package-specific.
		pd := storage.PackageData{
			Assets: []storage.Asset{
				{Filename: "othertool-windows-amd64.gz", OS: "windows", Arch: "x86_64", Format: ".gz"},
			},
		}
		lc, _ := storage.ExportLegacy("othertool", pd)
		if len(lc.Releases) != 1 {
			t.Fatalf("releases = %d, want 1", len(lc.Releases))
		}
		if lc.Releases[0].Ext != ".gz" {
			t.Errorf("ext = %q, want %q (no translation outside ffmpeg)", lc.Releases[0].Ext, ".gz")
		}
	})

	t.Run("universal2_passes_through", func(t *testing.T) {
		// universal2 (x86_64 + ARM64 fat binary) is kept as-is in the legacy
		// cache. The Node.js side handles resolution via its WATERFALL/CompatArches.
		pd := storage.PackageData{
			Assets: []storage.Asset{
				{Filename: "hugo_0.145.0_darwin-universal.tar.gz", OS: "darwin", Arch: "universal2", Format: ".tar.gz"},
			},
		}
		lc, stats := storage.ExportLegacy("hugo", pd)
		if stats.Variants != 0 || stats.Formats != 0 {
			t.Errorf("unexpected drops: variants=%d formats=%d", stats.Variants, stats.Formats)
		}
		if len(lc.Releases) != 1 {
			t.Fatalf("releases = %d, want 1", len(lc.Releases))
		}
		if lc.Releases[0].Arch != "universal2" {
			t.Errorf("arch = %q, want %q", lc.Releases[0].Arch, "universal2")
		}
	})
}

// TestExportLegacyMixed verifies correct counting when multiple drop/translate
// categories appear together in a single export call.
func TestExportLegacyMixed(t *testing.T) {
	pd := storage.PackageData{
		Assets: []storage.Asset{
			// kept: baseline linux build
			{Filename: "tool-linux-amd64.tar.gz", OS: "linux", Arch: "x86_64", Format: ".tar.gz"},
			// dropped: variant build
			{Filename: "tool-linux-amd64-rocm.tar.gz", OS: "linux", Arch: "x86_64", Format: ".tar.gz", Variants: []string{"rocm"}},
			// dropped: .apk format
			{Filename: "tool.apk", OS: "android", Format: ".apk"},
			// kept: illumos (translated to sunos)
			{Filename: "tool-illumos-amd64.tar.gz", OS: "illumos", Arch: "x86_64", Format: ".tar.gz"},
		},
	}
	lc, stats := storage.ExportLegacy("tool", pd)

	if stats.Variants != 1 {
		t.Errorf("Variants = %d, want 1", stats.Variants)
	}
	if stats.Formats != 1 {
		t.Errorf("Formats = %d, want 1", stats.Formats)
	}
	if len(lc.Releases) != 2 {
		t.Errorf("releases = %d, want 2 (linux + sunos)", len(lc.Releases))
	}

	// Verify the illumos → sunos translation was applied.
	var foundSunos bool
	for _, r := range lc.Releases {
		if r.OS == "sunos" {
			foundSunos = true
		}
		if r.OS == "illumos" {
			t.Error("illumos should have been translated to sunos")
		}
	}
	if !foundSunos {
		t.Error("expected a sunos release (translated from illumos)")
	}
}

// TestDecodeLegacyJSON verifies we can parse the exact JSON format
// the Node.js server writes to _cache/.
func TestDecodeLegacyJSON(t *testing.T) {
	// Real data from _cache/2026-03/aliasman.json.
	raw := `{
  "releases": [
    {
      "name": "BeyondCodeBootcamp-aliasman-v1.1.2-0-g0e5e1c1.tar.gz",
      "version": "v1.1.2",
      "lts": false,
      "channel": "stable",
      "date": "2023-02-23",
      "os": "posix_2017",
      "arch": "*",
      "libc": "",
      "ext": "",
      "download": "https://codeload.github.com/BeyondCodeBootcamp/aliasman/legacy.tar.gz/refs/tags/v1.1.2"
    },
    {
      "name": "BeyondCodeBootcamp-aliasman-v1.1.2-0-g0e5e1c1.zip",
      "version": "v1.1.2",
      "lts": false,
      "channel": "stable",
      "date": "2023-02-23",
      "os": "posix_2017",
      "arch": "*",
      "libc": "",
      "ext": "",
      "download": "https://codeload.github.com/BeyondCodeBootcamp/aliasman/legacy.zip/refs/tags/v1.1.2"
    }
  ],
  "download": ""
}`

	var lc storage.LegacyCache
	if err := json.Unmarshal([]byte(raw), &lc); err != nil {
		t.Fatal(err)
	}

	if len(lc.Releases) != 2 {
		t.Fatalf("got %d releases, want 2", len(lc.Releases))
	}

	pd := storage.ImportLegacy(lc)
	if len(pd.Assets) != 2 {
		t.Fatalf("got %d assets, want 2", len(pd.Assets))
	}

	a := pd.Assets[0]
	if a.Filename != "BeyondCodeBootcamp-aliasman-v1.1.2-0-g0e5e1c1.tar.gz" {
		t.Errorf("Filename = %q", a.Filename)
	}
	if a.Version != "v1.1.2" {
		t.Errorf("Version = %q", a.Version)
	}
	if a.OS != "posix_2017" {
		t.Errorf("OS = %q", a.OS)
	}
	if a.Arch != "*" {
		t.Errorf("Arch = %q", a.Arch)
	}
	if a.Download != "https://codeload.github.com/BeyondCodeBootcamp/aliasman/legacy.tar.gz/refs/tags/v1.1.2" {
		t.Errorf("Download = %q", a.Download)
	}

	// Round-trip: export back to legacy and verify JSON shape.
	lc2, _ := storage.ExportLegacy("aliasman", pd)
	data, _ := json.MarshalIndent(lc2, "", "  ")
	var lc3 storage.LegacyCache
	json.Unmarshal(data, &lc3)

	if lc3.Releases[0].Name != a.Filename {
		t.Errorf("round-trip Name = %q, want %q", lc3.Releases[0].Name, a.Filename)
	}
	if lc3.Releases[0].Ext != a.Format {
		t.Errorf("round-trip Ext = %q, want %q", lc3.Releases[0].Ext, a.Format)
	}
}
