// Package fish provides variant tagging for fish shell releases.
//
// Fish publishes .pkg macOS installers alongside the standard archives.
package fish

import "github.com/webinstall/webi-installers/internal/storage"

// Tagger implements storage.VariantTagger for fish.
var Tagger storage.VariantTagger = tagger{}

type tagger struct{}

func (tagger) TagVariants(assets []storage.Asset) {
	for i := range assets {
		if assets[i].Format == ".pkg" {
			assets[i].Variants = append(assets[i].Variants, "installer")
		}
	}
}
