package node

import "github.com/webinstall/webi-installers/internal/storage"

// TagVariants tags node-specific build variants.
//
// The bare .exe is just node.exe without npm — too minimal to be useful.
// .msi and .pkg are standard package formats and need no special tagging.
func TagVariants(assets []storage.Asset) {
	for i := range assets {
		if assets[i].Format == ".exe" {
			assets[i].Variants = append(assets[i].Variants, "bare-exe")
		}
	}
}
