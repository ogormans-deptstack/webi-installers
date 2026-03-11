// Package installerconf reads per-package releases.conf files.
//
// The format is simple key=value, one per line. Blank lines and lines
// starting with # are ignored. Keys and values are trimmed of whitespace.
// Multi-value keys are whitespace-delimited.
//
// The source type is inferred from the primary key:
//
// GitHub releases (covers ~70% of packages):
//
//	github_repo = sharkdp/bat
//
// With version prefix stripping (jq tags are "jq-1.7.1"):
//
//	github_repo = jqlang/jq
//	version_prefixes = jq-
//
// With filename exclusions and variant documentation:
//
//	github_repo = gohugoio/hugo
//	exclude = _extended_ Linux-64bit
//	variants = extended extended_withdeploy
//
// Monorepo with tag prefix:
//
//	github_repo = therootcompany/golib
//	tag_prefix = tools/monorel/
//
// Git tag sources (vim plugins, etc.):
//
//	git_url = https://github.com/tpope/vim-commentary.git
//
// Gitea releases:
//
//	gitea_repo = root/pathman
//	base_url = https://git.rootprojects.org
//
// HashiCorp releases:
//
//	hashicorp_product = terraform
//
// Other sources (one-off scrapers):
//
//	source = nodedist
//	url = https://nodejs.org/download/release
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

	// AliasOf names another package that this one mirrors.
	// When set, the package has no releases of its own — it shares
	// the cache output of the named target (e.g. dashd → dashcore).
	AliasOf string

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

	// Infer source from primary key, falling back to explicit "source".
	switch {
	case raw["github_repo"] != "":
		c.Source = "github"
		c.Owner, c.Repo, _ = strings.Cut(raw["github_repo"], "/")
	case raw["github_source"] != "":
		c.Source = "githubsource"
		c.Owner, c.Repo, _ = strings.Cut(raw["github_source"], "/")
	case raw["git_url"] != "":
		c.Source = "gittag"
		c.BaseURL = raw["git_url"]
	case raw["gitea_repo"] != "":
		c.Source = "gitea"
		c.Owner, c.Repo, _ = strings.Cut(raw["gitea_repo"], "/")
		c.BaseURL = raw["base_url"]
	case raw["hashicorp_product"] != "":
		c.Source = "hashicorp"
		c.Repo = raw["hashicorp_product"]
	default:
		// One-off dist sources (nodedist, zigdist, etc.).
		c.Source = raw["source"]
		c.BaseURL = raw["url"]
	}

	c.TagPrefix = raw["tag_prefix"]

	if v := raw["version_prefixes"]; v != "" {
		c.VersionPrefixes = strings.Fields(v)
	} else if v := raw["version_prefix"]; v != "" {
		c.VersionPrefixes = strings.Fields(v)
	}

	// Accept both "exclude" and "asset_exclude" (back-compat).
	if v := raw["exclude"]; v != "" {
		c.Exclude = strings.Fields(v)
	} else if v := raw["asset_exclude"]; v != "" {
		c.Exclude = strings.Fields(v)
	}

	c.AssetFilter = raw["asset_filter"]
	c.AliasOf = raw["alias_of"]

	if v := raw["variants"]; v != "" {
		c.Variants = strings.Fields(v)
	}

	// Collect unrecognized keys.
	known := map[string]bool{
		"source":             true,
		"github_repo":        true,
		"github_source":      true,
		"git_url":            true,
		"gitea_repo":         true,
		"hashicorp_product":  true,
		"base_url":           true,
		"url":                true,
		"tag_prefix":         true,
		"version_prefix":     true,
		"version_prefixes":   true,
		"exclude":            true,
		"asset_exclude":      true,
		"asset_filter":       true,
		"variants":           true,
		"alias_of":           true,
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
