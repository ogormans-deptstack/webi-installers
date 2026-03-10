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

// ToLegacy converts an Asset to the LegacyAsset wire format.
func (a Asset) ToLegacy() LegacyAsset {
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

// ImportLegacy converts a LegacyCache to PackageData.
func ImportLegacy(lc LegacyCache) PackageData {
	assets := make([]Asset, len(lc.Releases))
	for i, la := range lc.Releases {
		assets[i] = la.ToAsset()
	}
	return PackageData{Assets: assets}
}

// ExportLegacy converts PackageData to the LegacyCache wire format.
func ExportLegacy(pd PackageData) LegacyCache {
	releases := make([]LegacyAsset, len(pd.Assets))
	for i, a := range pd.Assets {
		releases[i] = a.ToLegacy()
	}
	return LegacyCache{Releases: releases}
}
