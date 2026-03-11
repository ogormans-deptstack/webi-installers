package storage

// Legacy types for reading/writing the Node.js _cache/ JSON format.
//
// The Node.js server calls assets "releases" and uses "name" for the
// filename and "ext" for the format. These types preserve that wire
// format for backward compatibility during migration.
//
// Internal Go code uses [Asset] and [PackageData] directly.

// LegacyAsset matches the JSON shape the Node.js server writes and reads.
type LegacyAsset struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	LTS      bool   `json:"lts"`
	Channel  string `json:"channel"`
	Date     string `json:"date"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Libc     string `json:"libc"`
	Ext      string `json:"ext"`
	Download string `json:"download"`
}

// LegacyCache matches the top-level JSON shape in _cache/{pkg}.json.
type LegacyCache struct {
	Releases []LegacyAsset `json:"releases"`
	Download string        `json:"download"`
}

// LegacyDropStats reports how many assets were excluded during ExportLegacy.
type LegacyDropStats struct {
	Variants int // dropped: has build variant tags (e.g. rocm, installer, fxdependent)
	Formats  int // dropped: format not recognized by the Node.js server
}

// ToAsset converts a LegacyAsset to the internal Asset type.
func (la LegacyAsset) ToAsset() Asset {
	return Asset{
		Filename: la.Name,
		Version:  la.Version,
		LTS:      la.LTS,
		Channel:  la.Channel,
		Date:     la.Date,
		OS:       la.OS,
		Arch:     la.Arch,
		Libc:     la.Libc,
		Format:   la.Ext,
		Download: la.Download,
	}
}

// toLegacy converts an Asset to the LegacyAsset wire format.
// Callers must have already applied legacyFieldBackport before calling this.
func (a Asset) toLegacy() LegacyAsset {
	return LegacyAsset{
		Name:     a.Filename,
		Version:  a.Version,
		LTS:      a.LTS,
		Channel:  a.Channel,
		Date:     a.Date,
		OS:       a.OS,
		Arch:     a.Arch,
		Libc:     a.Libc,
		Ext:      a.Format,
		Download: a.Download,
	}
}

// legacyFieldBackport translates canonical classifier field values to the
// values the legacy Node.js resolver expects. This is called at export time
// only — the canonical values are preserved in Go-native storage (pgstore).
//
// Global rules (all packages):
//   - solaris/illumos → sunos  (Node.js only knows "sunos")
//
// Package-specific rules replicate per-package overrides in production's releases.js:
//   - ffmpeg: Windows .gz → .exe  (prod releases.js: rel.ext = 'exe')
func legacyFieldBackport(pkg string, a Asset) Asset {
	// Global OS normalization: Node.js uses "sunos" for both Solaris and Illumos.
	if a.OS == "solaris" || a.OS == "illumos" {
		a.OS = "sunos"
	}

	switch pkg {
	case "ffmpeg":
		if a.OS == "windows" {
			switch a.Format {
			case ".gz", "":
				a.Format = ".exe"
			}
		}
	}
	return a
}

// ImportLegacy converts a LegacyCache to PackageData.
func ImportLegacy(lc LegacyCache) PackageData {
	assets := make([]Asset, len(lc.Releases))
	for i, la := range lc.Releases {
		assets[i] = la.ToAsset()
	}
	return PackageData{Assets: assets}
}

// legacyFormats is the set of formats the Node.js server recognizes.
// Assets with formats not in this set are filtered out of legacy exports.
var legacyFormats = map[string]bool{
	".zip":     true,
	".tar.gz":  true,
	".tar.xz":  true,
	".tar.zst": true,
	".tar.bz2": true,
	".tar":     true,
	".xz":      true,
	".7z":      true,
	".pkg":     true,
	".msi":     true,
	".exe":     true,
	".exe.xz":  true,
	".dmg":     true,
	".app.zip": true,
	".gz":      true,
	"git":      true,
}

// ExportLegacy converts canonical PackageData to the LegacyCache wire format.
//
// The pkg name is used to apply per-package field translations before export
// (see legacyFieldBackport). Assets are excluded when:
//   - Variants is non-empty (Node.js has no variant logic)
//   - Format is non-empty and not in the Node.js recognized set
//
// Dropped counts are returned in LegacyDropStats for logging.
func ExportLegacy(pkg string, pd PackageData) (LegacyCache, LegacyDropStats) {
	var releases []LegacyAsset
	var stats LegacyDropStats

	for _, a := range pd.Assets {
		// Skip variant builds — Node.js doesn't have variant logic.
		if len(a.Variants) > 0 {
			stats.Variants++
			continue
		}
		// Apply per-package legacy field translations before format check.
		a = legacyFieldBackport(pkg, a)
		// Skip formats Node.js doesn't recognize.
		if a.Format != "" && !legacyFormats[a.Format] {
			stats.Formats++
			continue
		}
		releases = append(releases, a.toLegacy())
	}
	if releases == nil {
		releases = []LegacyAsset{}
	}
	return LegacyCache{Releases: releases}, stats
}
