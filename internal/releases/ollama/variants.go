// Package ollama provides variant tagging for Ollama releases.
//
// Ollama publishes GPU accelerator builds: -rocm (AMD), -jetpack5
// and -jetpack6 (NVIDIA Jetson).
package ollama

import (
	"strings"

	"github.com/webinstall/webi-installers/internal/storage"
)

// Tagger implements storage.VariantTagger for Ollama.
var Tagger storage.VariantTagger = tagger{}

type tagger struct{}

func (tagger) TagVariants(assets []storage.Asset) {
	for i := range assets {
		lower := strings.ToLower(assets[i].Filename)
		for _, v := range []string{"rocm", "jetpack5", "jetpack6"} {
			if strings.Contains(lower, "-"+v) {
				assets[i].Variants = append(assets[i].Variants, v)
			}
		}
	}
}
