// Package installerconf reads per-package releases.conf files.
//
// The format is simple key=value, one per line. Blank lines and lines
// starting with # are ignored. Keys and values are trimmed of whitespace.
// Multi-value keys are whitespace-delimited.
//
// Minimal example (covers ~60% of packages):
//
//	source = github
//	owner = sharkdp
//	repo = bat
//
// With version prefix stripping (jq tags are "jq-1.7.1"):
//
//	source = github
//	owner = jqlang
//	repo = jq
//	version_prefixes = jq-
//
// With filename exclusions and variant documentation:
//
//	source = github
//	owner = gohugoio
//	repo = hugo
//	exclude = _extended_ Linux-64bit
//	variants = extended extended_withdeploy
//
// Monorepo with tag prefix:
//
//	source = github
//	owner = therootcompany
//	repo = golib
//	tag_prefix = tools/monorel/
//
// Non-GitHub sources:
//
//	source = nodedist
//	url = https://nodejs.org/download/release
//
//	source = gitea
//	base_url = https://gitea.com
//	owner = xorm
//	repo = xorm
//
// Complex packages that need custom logic beyond what the classifier
// auto-detects (e.g. ollama's universal binaries, ffmpeg's non-standard
// naming) should put that logic in Go code, not in the config.
// The variants key documents known build variants for human readers;
// actual variant detection logic lives in Go.
package installerconf

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Conf holds the parsed per-package release configuration.
type Conf struct {
	// Source is the fetch source type: "github", "gitea", "gitlab",
	// "gittag", "nodedist", etc.
	Source string

	// Owner is the repository owner (org or user).
	Owner string

	// Repo is the repository name.
	Repo string

	// BaseURL is a custom base URL for non-GitHub sources
	// (e.g. a Gitea instance or nodedist index URL).
	BaseURL string

	// TagPrefix filters releases in monorepos. Only tags starting with
	// this prefix are included, and the prefix is stripped from the
	// version string. Example: "tools/monorel/"
	TagPrefix string

	// VersionPrefixes are stripped from version/tag strings.
	// Whitespace-delimited. Each release tag is checked against these
	// in order; the first match is stripped. Projects may change tag
	// conventions across versions (e.g. "jq-1.7.1" older, "1.8.0" later).
	VersionPrefixes []string

	// Exclude lists filename substrings to filter out.
	// Whitespace-delimited. Assets whose name contains any of these
	// are skipped entirely (not stored).
	Exclude []string

	// AssetFilter is a substring that asset filenames must contain.
	// Used when multiple packages share a GitHub release (e.g.
	// kubectx/kubens) to select only the relevant assets.
	AssetFilter string

	// Variants documents known build variant names for this package.
	// Whitespace-delimited. This is a human-readable cue — actual
	// variant detection logic lives in Go code per-package.
	Variants []string

	// Extra holds any unrecognized keys for forward compatibility.
	Extra map[string]string
}

// Read parses a releases.conf file.
func Read(path string) (*Conf, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("installerconf: %w", err)
	}
	defer f.Close()

	raw := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		raw[strings.TrimSpace(key)] = strings.TrimSpace(val)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("installerconf: read %s: %w", path, err)
	}

	c := &Conf{}
	c.Source = raw["source"]
	c.Owner = raw["owner"]
	c.Repo = raw["repo"]
	c.TagPrefix = raw["tag_prefix"]

	if v := raw["version_prefixes"]; v != "" {
		c.VersionPrefixes = strings.Fields(v)
	} else if v := raw["version_prefix"]; v != "" {
		c.VersionPrefixes = strings.Fields(v)
	}

	if v := raw["base_url"]; v != "" {
		c.BaseURL = v
	} else {
		c.BaseURL = raw["url"]
	}

	// Accept both "exclude" and "asset_exclude" (back-compat).
	if v := raw["exclude"]; v != "" {
		c.Exclude = strings.Fields(v)
	} else if v := raw["asset_exclude"]; v != "" {
		c.Exclude = strings.Fields(v)
	}

	c.AssetFilter = raw["asset_filter"]

	if v := raw["variants"]; v != "" {
		c.Variants = strings.Fields(v)
	}

	// Collect unrecognized keys.
	known := map[string]bool{
		"source": true, "owner": true, "repo": true,
		"base_url": true, "url": true,
		"tag_prefix": true, "version_prefix": true, "version_prefixes": true,
		"exclude": true, "asset_exclude": true, "asset_filter": true,
		"variants": true,
	}
	for k, v := range raw {
		if !known[k] {
			if c.Extra == nil {
				c.Extra = make(map[string]string)
			}
			c.Extra[k] = v
		}
	}

	return c, nil
}
