package node

import "github.com/webinstall/webi-installers/internal/storage"

// TagVariants tags node-specific build variants.
//
//   - .msi — Windows package format (msiexec /a to extract)
//   - .pkg — macOS package format (pkgutil --expand-full to extract)
//   - .exe — bare node.exe without npm, too minimal to be useful
func TagVariants(assets []storage.Asset) {
	for i := range assets {
		switch assets[i].Format {
		case ".msi":
			assets[i].Variants = append(assets[i].Variants, "msi")
		case ".pkg":
			assets[i].Variants = append(assets[i].Variants, "pkg")
		case ".exe":
			assets[i].Variants = append(assets[i].Variants, "bare-exe")
		}
	}
}
