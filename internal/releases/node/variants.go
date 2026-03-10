package node

import "github.com/webinstall/webi-installers/internal/storage"

// TagVariants tags node-specific build variants.
//
//   - .msi — Windows installer
//   - .pkg — macOS installer (pkgutil --expand-full)
//   - .exe — bare node.exe without npm, too minimal to be useful
func TagVariants(assets []storage.Asset) {
	for i := range assets {
		switch assets[i].Format {
		case ".msi", ".pkg":
			assets[i].Variants = append(assets[i].Variants, "installer")
		case ".exe":
			assets[i].Variants = append(assets[i].Variants, "bare-exe")
		}
	}
}
