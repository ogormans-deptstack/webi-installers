package storage_test

import (
	"encoding/json"
	"testing"

	"github.com/webinstall/webi-installers/internal/storage"
)

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

	t.Run("universal2_dropped", func(t *testing.T) {
		// universal2 fat binaries are dropped. The Node classifier maps "universal"
		// in the filename to x86_64, then rejects the cache entry because it says
		// "universal2". The mismatch can't be fixed without changing the filename.
		pd := storage.PackageData{
			Assets: []storage.Asset{
				{Filename: "hugo_0.145.0_darwin-universal.tar.gz", OS: "darwin", Arch: "universal2", Format: ".tar.gz"},
				{Filename: "hugo_0.145.0_darwin-arm64.tar.gz", OS: "darwin", Arch: "aarch64", Format: ".tar.gz"},
			},
		}
		lc, stats := storage.ExportLegacy("hugo", pd)
		if stats.Universal != 1 {
			t.Errorf("Universal dropped = %d, want 1", stats.Universal)
		}
		if len(lc.Releases) != 1 {
			t.Errorf("releases = %d, want 1 (arm64 only)", len(lc.Releases))
		}
		if len(lc.Releases) > 0 && lc.Releases[0].Arch != "aarch64" {
			t.Errorf("kept arch = %q, want aarch64", lc.Releases[0].Arch)
		}
	})

	t.Run("solaris_dropped", func(t *testing.T) {
		// Solaris entries are dropped: Node never served these platforms and the
		// classifier produces unfixable mismatches for them.
		pd := storage.PackageData{
			Assets: []storage.Asset{
				{Filename: "go1.20.1.solaris-amd64.tar.gz", OS: "solaris", Arch: "x86_64", Format: ".tar.gz"},
				{Filename: "go1.20.1.linux-amd64.tar.gz", OS: "linux", Arch: "x86_64", Format: ".tar.gz"},
			},
		}
		lc, stats := storage.ExportLegacy("go", pd)
		if stats.SunOS != 1 {
			t.Errorf("SunOS dropped = %d, want 1", stats.SunOS)
		}
		if len(lc.Releases) != 1 {
			t.Errorf("releases = %d, want 1 (linux only)", len(lc.Releases))
		}
	})

	t.Run("illumos_dropped", func(t *testing.T) {
		pd := storage.PackageData{
			Assets: []storage.Asset{
				{Filename: "go1.20.1.illumos-amd64.tar.gz", OS: "illumos", Arch: "x86_64", Format: ".tar.gz"},
			},
		}
		lc, stats := storage.ExportLegacy("go", pd)
		if stats.SunOS != 1 {
			t.Errorf("SunOS dropped = %d, want 1", stats.SunOS)
		}
		if len(lc.Releases) != 0 {
			t.Errorf("releases = %d, want 0", len(lc.Releases))
		}
	})

	t.Run("android_dropped", func(t *testing.T) {
		// Android entries are dropped: the classifier maps android filenames to
		// linux OS and then rejects the cache entry that says android.
		pd := storage.PackageData{
			Assets: []storage.Asset{
				{Filename: "fzf-0.57.0-android-arm64.tar.gz", OS: "android", Arch: "aarch64", Format: ".tar.gz"},
				{Filename: "fzf-0.57.0-linux-arm64.tar.gz", OS: "linux", Arch: "aarch64", Format: ".tar.gz"},
			},
		}
		lc, stats := storage.ExportLegacy("fzf", pd)
		if stats.Android != 1 {
			t.Errorf("Android dropped = %d, want 1", stats.Android)
		}
		if len(lc.Releases) != 1 {
			t.Errorf("releases = %d, want 1 (linux only)", len(lc.Releases))
		}
	})

	t.Run("unknown_formats_dropped", func(t *testing.T) {
		// .AppImage, .deb, .rpm are not in the Node.js format set.
		pd := storage.PackageData{
			Assets: []storage.Asset{
				{Filename: "tool.AppImage", OS: "linux", Format: ".AppImage"},
				{Filename: "tool.deb", OS: "linux", Format: ".deb"},
				{Filename: "tool.rpm", OS: "linux", Format: ".rpm"},
				{Filename: "tool-linux-amd64.tar.gz", OS: "linux", Arch: "x86_64", Format: ".tar.gz"},
			},
		}
		lc, stats := storage.ExportLegacy("tool", pd)
		if stats.Formats != 3 {
			t.Errorf("Formats dropped = %d, want 3", stats.Formats)
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
// correct field translations for Node.js compatibility.
func TestExportLegacyTranslations(t *testing.T) {
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

	// ARM arch translations: the Node build-classifier re-parses filenames and
	// rejects cache entries where the arch field doesn't match what it extracts.
	// Go normalizes ARM filenames to canonical arch values, but the Node classifier
	// preserves the original Debian/Rust naming. These translations undo Go's
	// normalization for the legacy cache only.
	t.Run("arm_gnueabihf_to_armhf", func(t *testing.T) {
		// Go: gnueabihf → armv6 (Rust soft-float ABI name)
		// Node classifier: gnueabihf → armhf
		pd := storage.PackageData{
			Assets: []storage.Asset{
				{Filename: "bat-v0.9.0-arm-unknown-linux-gnueabihf.tar.gz", OS: "linux", Arch: "armv6", Format: ".tar.gz"},
			},
		}
		lc, _ := storage.ExportLegacy("bat", pd)
		if len(lc.Releases) != 1 {
			t.Fatalf("releases = %d, want 1", len(lc.Releases))
		}
		if lc.Releases[0].Arch != "armhf" {
			t.Errorf("arch = %q, want armhf", lc.Releases[0].Arch)
		}
	})

	t.Run("arm_armhf_filename_to_armhf", func(t *testing.T) {
		// Go: armhf → armv7 (Debian armhf = ARMv7 hard-float)
		// Node classifier: armhf → armhf
		pd := storage.PackageData{
			Assets: []storage.Asset{
				{Filename: "caddy_linux_armhf.tar.gz", OS: "linux", Arch: "armv7", Format: ".tar.gz"},
			},
		}
		lc, _ := storage.ExportLegacy("caddy", pd)
		if len(lc.Releases) != 1 {
			t.Fatalf("releases = %d, want 1", len(lc.Releases))
		}
		if lc.Releases[0].Arch != "armhf" {
			t.Errorf("arch = %q, want armhf", lc.Releases[0].Arch)
		}
	})

	t.Run("arm_armel_to_armel", func(t *testing.T) {
		// Go: armel → armv6 (Debian soft-float ABI name)
		// Node classifier: armel → armel
		pd := storage.PackageData{
			Assets: []storage.Asset{
				{Filename: "caddy_linux_armel.tar.gz", OS: "linux", Arch: "armv6", Format: ".tar.gz"},
			},
		}
		lc, _ := storage.ExportLegacy("caddy", pd)
		if len(lc.Releases) != 1 {
			t.Fatalf("releases = %d, want 1", len(lc.Releases))
		}
		if lc.Releases[0].Arch != "armel" {
			t.Errorf("arch = %q, want armel", lc.Releases[0].Arch)
		}
	})

	t.Run("arm_armv5_to_armel", func(t *testing.T) {
		// Go: armv5 → armv5
		// Node classifier tiered map: armv5 falls back to armel
		pd := storage.PackageData{
			Assets: []storage.Asset{
				{Filename: "caddy_linux_armv5.tar.gz", OS: "linux", Arch: "armv5", Format: ".tar.gz"},
			},
		}
		lc, _ := storage.ExportLegacy("caddy", pd)
		if len(lc.Releases) != 1 {
			t.Fatalf("releases = %d, want 1", len(lc.Releases))
		}
		if lc.Releases[0].Arch != "armel" {
			t.Errorf("arch = %q, want armel", lc.Releases[0].Arch)
		}
	})

	t.Run("arm_armv7a_to_armv7a", func(t *testing.T) {
		// Go: armv7a → armv7 (Go normalizes armv7a to armv7)
		// Node classifier: armv7a → armv7a (preserves the -a suffix)
		pd := storage.PackageData{
			Assets: []storage.Asset{
				{Filename: "tool-armv7a-linux.tar.gz", OS: "linux", Arch: "armv7", Format: ".tar.gz"},
			},
		}
		lc, _ := storage.ExportLegacy("tool", pd)
		if len(lc.Releases) != 1 {
			t.Fatalf("releases = %d, want 1", len(lc.Releases))
		}
		if lc.Releases[0].Arch != "armv7a" {
			t.Errorf("arch = %q, want armv7a", lc.Releases[0].Arch)
		}
	})

	t.Run("arm_armv7l_unchanged", func(t *testing.T) {
		// armv7l in filename: Go=armv7, Node classifier also extracts armv7. No translation.
		pd := storage.PackageData{
			Assets: []storage.Asset{
				{Filename: "tool-armv7l-linux.tar.gz", OS: "linux", Arch: "armv7", Format: ".tar.gz"},
			},
		}
		lc, _ := storage.ExportLegacy("tool", pd)
		if len(lc.Releases) != 1 {
			t.Fatalf("releases = %d, want 1", len(lc.Releases))
		}
		if lc.Releases[0].Arch != "armv7" {
			t.Errorf("arch = %q, want armv7 (no translation for armv7l)", lc.Releases[0].Arch)
		}
	})

	t.Run("arm_armv6l_unchanged", func(t *testing.T) {
		// armv6l in filename: Go=armv6, Node classifier also extracts armv6. No translation.
		pd := storage.PackageData{
			Assets: []storage.Asset{
				{Filename: "tool-armv6l-linux.tar.gz", OS: "linux", Arch: "armv6", Format: ".tar.gz"},
			},
		}
		lc, _ := storage.ExportLegacy("tool", pd)
		if len(lc.Releases) != 1 {
			t.Fatalf("releases = %d, want 1", len(lc.Releases))
		}
		if lc.Releases[0].Arch != "armv6" {
			t.Errorf("arch = %q, want armv6 (no translation for armv6l)", lc.Releases[0].Arch)
		}
	})
}

// TestExportLegacyMixed verifies correct counting when multiple drop categories
// appear together in a single export call.
func TestExportLegacyMixed(t *testing.T) {
	pd := storage.PackageData{
		Assets: []storage.Asset{
			// kept: baseline linux build
			{Filename: "tool-linux-amd64.tar.gz", OS: "linux", Arch: "x86_64", Format: ".tar.gz"},
			// dropped: variant build
			{Filename: "tool-linux-amd64-rocm.tar.gz", OS: "linux", Arch: "x86_64", Format: ".tar.gz", Variants: []string{"rocm"}},
			// dropped: universal2
			{Filename: "tool-darwin-universal.tar.gz", OS: "darwin", Arch: "universal2", Format: ".tar.gz"},
			// dropped: illumos
			{Filename: "tool-illumos-amd64.tar.gz", OS: "illumos", Arch: "x86_64", Format: ".tar.gz"},
			// dropped: android
			{Filename: "tool-android-arm64.tar.gz", OS: "android", Arch: "aarch64", Format: ".tar.gz"},
			// dropped: .AppImage format
			{Filename: "tool.AppImage", OS: "linux", Format: ".AppImage"},
		},
	}
	lc, stats := storage.ExportLegacy("tool", pd)

	if stats.Variants != 1 {
		t.Errorf("Variants = %d, want 1", stats.Variants)
	}
	if stats.Universal != 1 {
		t.Errorf("Universal = %d, want 1", stats.Universal)
	}
	if stats.SunOS != 1 {
		t.Errorf("SunOS = %d, want 1", stats.SunOS)
	}
	if stats.Android != 1 {
		t.Errorf("Android = %d, want 1", stats.Android)
	}
	if stats.Formats != 1 {
		t.Errorf("Formats = %d, want 1", stats.Formats)
	}
	if len(lc.Releases) != 1 {
		t.Errorf("releases = %d, want 1 (linux only)", len(lc.Releases))
	}
}
