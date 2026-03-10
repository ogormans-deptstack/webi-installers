package node

import "github.com/webinstall/webi-installers/internal/storage"

// TagVariants tags node-specific build variants.
//
// .msi is a Windows installer. .exe is the bare node.exe binary — valid
// and installable by Go, but not present in the legacy Node.js cache
// (the nodedist classifier doesn't construct that filename).
func TagVariants(assets []storage.Asset) {
	for i := range assets {
		switch assets[i].Format {
		case ".msi":
			assets[i].Variants = append(assets[i].Variants, "installer")
		case ".exe":
			assets[i].Variants = append(assets[i].Variants, "bare-exe")
		}
	}
}
