// Package git provides variant tagging for Git for Windows releases.
//
// Git for Windows publishes GUI installer .exe files (Git-*-bit.exe),
// self-extracting PortableGit archives, and .pdb debug symbol packages
// alongside the MinGit .zip that webi installs.
package git

import (
	"strings"

	"github.com/webinstall/webi-installers/internal/storage"
)

// Tagger implements storage.VariantTagger for Git.
var Tagger storage.VariantTagger = tagger{}

type tagger struct{}

func (tagger) TagVariants(assets []storage.Asset) {
	for i := range assets {
		lower := strings.ToLower(assets[i].Filename)
		if assets[i].Format == ".exe" {
			assets[i].Variants = append(assets[i].Variants, "installer")
		}
		if strings.Contains(lower, "portablegit") {
			assets[i].Variants = append(assets[i].Variants, "installer")
		}
		if strings.Contains(lower, "-pdb") {
			assets[i].Variants = append(assets[i].Variants, "pdb")
		}
	}
}
