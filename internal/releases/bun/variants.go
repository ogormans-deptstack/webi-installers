// Package bun provides variant tagging for Bun releases.
//
// Bun publishes -profile (debug) builds and uses a non-standard arch
// convention: the default x86_64 build targets x86_64_v3 (AVX2+),
// while -baseline targets plain x86_64.
package bun

import (
	"strings"

	"github.com/webinstall/webi-installers/internal/storage"
)

// TagVariants tags bun-specific build variants and remaps arch fields.
func TagVariants(assets []storage.Asset) {
	for i := range assets {
		lower := strings.ToLower(assets[i].Filename)
		if strings.Contains(lower, "-profile") {
			assets[i].Variants = append(assets[i].Variants, "profile")
		}
		// Non-baseline x86_64 is actually x86_64_v3; baseline is plain amd64.
		if assets[i].Arch == "amd64" {
			if !strings.Contains(lower, "-baseline") {
				assets[i].Arch = "amd64v3"
			}
		}
	}
}
