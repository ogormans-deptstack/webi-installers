package node

import "github.com/webinstall/webi-installers/internal/storage"

// TagVariants tags node-specific build variants.
// Only .msi files are installers; .exe is the bare binary.
func TagVariants(assets []storage.Asset) {
	for i := range assets {
		if assets[i].Format == ".msi" {
			assets[i].Variants = append(assets[i].Variants, "installer")
		}
	}
}
