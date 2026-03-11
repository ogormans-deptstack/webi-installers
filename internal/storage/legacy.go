package storage

import "strings"

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
	Android  int // dropped: android OS — classifier maps android filenames to linux
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
// The Node build-classifier re-parses each asset's download filename and drops
// any entry where the cache field doesn't match what it extracts from the name.
// These translations ensure the cache matches the classifier's extraction.
//
// Global arch translations (all packages):
//   - universal2/universal1 → x86_64: classifier maps "universal" in filename
//     to x86_64. The darwin WATERFALL falls back aarch64→x86_64, so arm64
//     users still receive these builds.
//   - x86_64_v2 → x86_64: classifier doesn't recognize micro-arch level suffixes.
//   - mips64r6/mips64r6el → mips64: MIPS Release 6 variants map to the base arch.
//   - ARM (filename-based): gnueabihf/armhf→armhf, armel→armel, armv5→armel,
//     armv7a→armv7a. Go normalizes these; Node classifier preserves the
//     original Debian/Rust naming. See legacyARMArchFromFilename.
//
// Note: solaris/illumos/sunos are kept as-is. The build-classifier (triplet.js)
// recognizes all three as distinct values, and the live cache uses them directly.
//
// Package-specific rules replicate per-package overrides in production's releases.js:
//   - ffmpeg: Windows .gz → .exe  (prod releases.js: rel.ext = 'exe')
func legacyFieldBackport(pkg string, a Asset) Asset {
	// Universal fat binaries: classifier maps "universal" in filename to x86_64.
	if a.Arch == "universal2" || a.Arch == "universal1" {
		a.Arch = "x86_64"
	}

	// x86_64 micro-arch levels: classifier doesn't know these suffixes.
	if a.Arch == "x86_64_v2" || a.Arch == "x86_64_v3" || a.Arch == "x86_64_v4" {
		a.Arch = "x86_64"
	}

	// MIPS Release 6 variants: map to the base mips64 arch.
	if a.Arch == "mips64r6" || a.Arch == "mips64r6el" {
		a.Arch = "mips64"
	}

	// ARM arch: the Node classifier re-parses filenames and expects the cache
	// arch to match what it extracts. Go normalizes (gnueabihf→armv6, armhf→armv7)
	// but the Node classifier preserves the original Debian/Rust naming.
	switch a.Arch {
	case "armv5", "armv6", "armv7":
		if leg := legacyARMArchFromFilename(a.Filename); leg != "" {
			a.Arch = leg
		}
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

// legacyARMArchFromFilename returns the arch string the Node build-classifier
// would extract from a filename for ARM-family builds. Returns "" when the
// Go canonical arch value already matches what the classifier would extract.
//
// The Node classifier's extraction rules differ from Go's normalization:
//   - gnueabihf (Rust triplet) / armhf (Debian) → "armhf" (not "armv6" or "armv7")
//   - armel (Debian soft-float ABI) → "armel" (not "armv6")
//   - armv5 → "armel" (Node tiered map: armv5 falls back to armel)
//   - armv7a → "armv7a" (not "armv7")
func legacyARMArchFromFilename(filename string) string {
	lower := strings.ToLower(filename)
	if strings.Contains(lower, "gnueabihf") || strings.Contains(lower, "armhf") {
		return "armhf"
	}
	if strings.Contains(lower, "armv7a") {
		return "armv7a"
	}
	if strings.Contains(lower, "armel") {
		return "armel"
	}
	if strings.Contains(lower, "armv5") {
		return "armel"
	}
	return ""
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
// The pkg name is used to apply per-package field translations (see legacyFieldBackport).
// Assets are excluded when:
//   - Variants is non-empty (Node.js has no variant logic)
//   - OS is android (classifier maps android filenames to linux)
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
		// Skip android — classifier maps android filenames to linux OS,
		// which mismatches cache entries tagged android.
		if a.OS == "android" {
			stats.Android++
			continue
		}
		// Apply per-package and global legacy field translations.
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
