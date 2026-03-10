// Package xcaddy provides variant tagging for xcaddy releases.
//
// xcaddy publishes .deb packages alongside the standard archives.
package xcaddy

import "github.com/webinstall/webi-installers/internal/storage"

// Tagger implements storage.VariantTagger for xcaddy.
var Tagger storage.VariantTagger = tagger{}

type tagger struct{}

func (tagger) TagVariants(assets []storage.Asset) {
	for i := range assets {
		if assets[i].Format == ".deb" {
			assets[i].Variants = append(assets[i].Variants, "deb")
		}
	}
}
