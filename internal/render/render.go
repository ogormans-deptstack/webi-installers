// Package render generates installer scripts by injecting release
// metadata into the package-install template.
//
// The template uses shell-style variable markers:
//
//	#WEBI_VERSION=        →  WEBI_VERSION='1.2.3'
//	#export WEBI_PKG_URL= →  export WEBI_PKG_URL='https://...'
//
// The package's install.sh is injected at the {{ installer }} marker.
package render

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Params holds all the values to inject into the installer template.
type Params struct {
	// Host is the base URL of the webi server (e.g. "https://webinstall.dev").
	Host string

	// Checksum is the webi.sh bootstrap script checksum (first 8 hex chars of SHA-1).
	Checksum string

	// Package name (e.g. "bat", "node").
	PkgName string

	// Tag is the version selector from the URL (e.g. "20", "stable", "").
	Tag string

	// OS, Arch, Libc are the detected platform strings.
	OS   string
	Arch string
	Libc string

	// Resolved release info.
	Version   string
	Major     string
	Minor     string
	Patch     string
	Build     string
	GitTag    string
	GitBranch string
	LTS       string // "true" or "false"
	Channel   string
	Ext       string // archive extension (e.g. "tar.gz", "zip")
	Formats   string // comma-separated format list

	// Download info.
	PkgURL  string // download URL
	PkgFile string // filename

	// Releases API URL for this request.
	ReleasesURL string

	// CSV line for WEBI_CSV.
	CSV string

	// Package catalog info.
	PkgStable  string
	PkgLatest  string
	PkgOSes    string // space-separated
	PkgArches  string // space-separated
	PkgLibcs   string // space-separated
	PkgFormats string // space-separated
}

// Bash renders a complete bash installer script by injecting params
// into the template and splicing in the package's install.sh.
func Bash(tplPath, installersDir, pkgName string, p Params) (string, error) {
	tpl, err := os.ReadFile(tplPath)
	if err != nil {
		return "", fmt.Errorf("render: read template: %w", err)
	}

	// Read the package's install.sh.
	installPath := filepath.Join(installersDir, pkgName, "install.sh")
	installSh, err := os.ReadFile(installPath)
	if err != nil {
		return "", fmt.Errorf("render: read %s/install.sh: %w", pkgName, err)
	}

	text := string(tpl)

	// Inject environment variables.
	vars := []struct {
		name  string
		value string
	}{
		{"WEBI_CHECKSUM", p.Checksum},
		{"WEBI_PKG", p.PkgName + "@" + p.Tag},
		{"WEBI_HOST", p.Host},
		{"WEBI_OS", p.OS},
		{"WEBI_ARCH", p.Arch},
		{"WEBI_LIBC", p.Libc},
		{"WEBI_TAG", p.Tag},
		{"WEBI_RELEASES", p.ReleasesURL},
		{"WEBI_CSV", p.CSV},
		{"WEBI_VERSION", p.Version},
		{"WEBI_MAJOR", p.Major},
		{"WEBI_MINOR", p.Minor},
		{"WEBI_PATCH", p.Patch},
		{"WEBI_BUILD", p.Build},
		{"WEBI_GIT_BRANCH", p.GitBranch},
		{"WEBI_GIT_TAG", p.GitTag},
		{"WEBI_LTS", p.LTS},
		{"WEBI_CHANNEL", p.Channel},
		{"WEBI_EXT", p.Ext},
		{"WEBI_FORMATS", p.Formats},
		{"WEBI_PKG_URL", p.PkgURL},
		{"WEBI_PKG_PATHNAME", p.PkgFile},
		{"WEBI_PKG_FILE", p.PkgFile},
		{"PKG_NAME", p.PkgName},
		{"PKG_STABLE", p.PkgStable},
		{"PKG_LATEST", p.PkgLatest},
		{"PKG_OSES", p.PkgOSes},
		{"PKG_ARCHES", p.PkgArches},
		{"PKG_LIBCS", p.PkgLibcs},
		{"PKG_FORMATS", p.PkgFormats},
	}

	for _, v := range vars {
		text = injectVar(text, v.name, v.value)
	}

	// Inject the installer script at the {{ installer }} marker.
	text = strings.Replace(text, "# {{ installer }}", string(installSh), 1)
	text = strings.Replace(text, "{{ installer }}", string(installSh), 1)

	return text, nil
}

// varPattern matches shell variable declarations in the template.
// Matches lines like:
//
//	#WEBI_VERSION=
//	#export WEBI_PKG_URL=
//	    #WEBI_OS=
var varPatterns = map[string]*regexp.Regexp{}

func getVarPattern(name string) *regexp.Regexp {
	if p, ok := varPatterns[name]; ok {
		return p
	}
	// Match: optional leading whitespace, optional #, optional export, the var name, =, rest of line
	p := regexp.MustCompile(`(?m)^([ \t]*)#?([ \t]*)(export[ \t]+)?[ \t]*(` + regexp.QuoteMeta(name) + `)=.*$`)
	varPatterns[name] = p
	return p
}

// injectVar replaces a template variable line with its value.
func injectVar(text, name, value string) string {
	p := getVarPattern(name)
	return p.ReplaceAllString(text, "${1}${3}"+name+"='"+sanitizeShellValue(value)+"'")
}

// sanitizeShellValue ensures a value is safe to embed in single quotes.
// Single quotes in shell can't be escaped inside single quotes, so we
// close-quote, add escaped quote, re-open quote: 'foo'\''bar'
func sanitizeShellValue(s string) string {
	return strings.ReplaceAll(s, "'", `'\''`)
}
