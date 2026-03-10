// Package pwsh provides variant tagging for PowerShell releases.
//
// PowerShell publishes .NET framework-dependent builds (-fxdependent)
// that are smaller but require a .NET runtime to be installed.
package pwsh

import (
	"strings"

	"github.com/webinstall/webi-installers/internal/storage"
)

// TagVariants tags pwsh-specific build variants.
func TagVariants(assets []storage.Asset) {
	for i := range assets {
		lower := strings.ToLower(assets[i].Filename)
		if strings.Contains(lower, "-fxdependentwindesktop") {
			assets[i].Variants = append(assets[i].Variants, "fxdependentWinDesktop")
		} else if strings.Contains(lower, "-fxdependent") {
			assets[i].Variants = append(assets[i].Variants, "fxdependent")
		}
	}
}
