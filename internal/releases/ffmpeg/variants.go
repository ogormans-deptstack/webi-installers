// Package ffmpeg provides variant tagging for ffmpeg-static releases.
//
// ffmpeg-static publishes bare executables for Windows. The .gz files
// are gzip-compressed bare executables (not archives). Production
// classifies these as ext "exe".
package ffmpeg

import (
	"github.com/webinstall/webi-installers/internal/storage"
)

// TagVariants fixes Windows asset extensions for ffmpeg.
//
// Windows .gz files contain a gzipped bare executable, not a tar archive.
// Production treats these as ext "exe". Bare Windows files (no extension)
// are also executables.
func TagVariants(assets []storage.Asset) {
	for i := range assets {
		if assets[i].OS != "windows" {
			continue
		}
		switch assets[i].Format {
		case ".gz":
			assets[i].Format = ".exe"
		case "":
			assets[i].Format = ".exe"
		}
	}
}
