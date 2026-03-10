package node

import "github.com/webinstall/webi-installers/internal/storage"

// Tagger implements storage.VariantTagger for Node.js.
// Only .msi files are installers; .exe is the bare binary.
var Tagger storage.VariantTagger = tagger{}

type tagger struct{}

func (tagger) TagVariants(assets []storage.Asset) {
	for i := range assets {
		if assets[i].Format == ".msi" {
			assets[i].Variants = append(assets[i].Variants, "installer")
		}
	}
}
