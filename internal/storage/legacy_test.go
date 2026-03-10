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
	lc2 := storage.ExportLegacy(pd)
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
