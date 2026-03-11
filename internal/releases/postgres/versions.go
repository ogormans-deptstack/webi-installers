package postgres

import (
	"strings"

	"github.com/webinstall/webi-installers/internal/storage"
)

// NormalizeVersions strips the REL_ prefix and converts underscores to dots.
// GitHub tags are "REL_17_0" → version becomes "17.0".
func NormalizeVersions(assets []storage.Asset) {
	for i := range assets {
		v := strings.TrimPrefix(assets[i].Version, "REL_")
		assets[i].Version = strings.ReplaceAll(v, "_", ".")
	}
}
