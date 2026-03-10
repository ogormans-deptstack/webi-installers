// Package yq provides variant tagging for yq releases.
//
// yq publishes a man-page-only tarball alongside binaries.
package yq

import (
	"strings"

	"github.com/webinstall/webi-installers/internal/storage"
)

// TagVariants tags yq-specific build variants.
func TagVariants(assets []storage.Asset) {
	for i := range assets {
		if strings.Contains(strings.ToLower(assets[i].Filename), "man_page_only") {
			assets[i].Variants = append(assets[i].Variants, "man-pages")
		}
	}
}
