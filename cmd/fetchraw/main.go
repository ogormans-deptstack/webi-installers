// Command fetchraw fetches release histories from upstream APIs and
// merges them into rawcache. Safe to run repeatedly — unchanged releases
// are skipped, new/changed ones are recorded in the audit log.
//
// Reads releases.conf files from package directories to discover what
// to fetch. Adding a new package is just creating a conf file.
//
// Usage:
//
//	go run ./cmd/fetchraw -cache ./_cache/raw
//	go run ./cmd/fetchraw -cache ./_cache/raw hugo caddy
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/webinstall/webi-installers/internal/installerconf"
	"github.com/webinstall/webi-installers/internal/lexver"
	"github.com/webinstall/webi-installers/internal/rawcache"
	"github.com/webinstall/webi-installers/internal/releases/github"
	"github.com/webinstall/webi-installers/internal/releases/githubish"
	"github.com/webinstall/webi-installers/internal/releases/nodedist"
)

func main() {
	cacheDir := flag.String("cache", "_cache/raw", "root directory for raw cache")
	confDir := flag.String("conf", ".", "root directory containing {pkg}/releases.conf files")
	token := flag.String("token", os.Getenv("GITHUB_TOKEN"), "GitHub API token")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	client := &http.Client{Timeout: 30 * time.Second}
	var auth *githubish.Auth
	if *token != "" {
		auth = &githubish.Auth{Token: *token}
	}

	// Discover packages from releases.conf files.
	packages, err := discover(*confDir)
	if err != nil {
		log.Fatalf("discover: %v", err)
	}

	// Filter to requested packages if args given.
	args := flag.Args()
	if len(args) > 0 {
		nameSet := make(map[string]bool, len(args))
		for _, a := range args {
			nameSet[a] = true
		}
		var filtered []pkgConf
		for _, p := range packages {
			if nameSet[p.name] {
				filtered = append(filtered, p)
			}
		}
		packages = filtered
	}

	log.Printf("found %d packages", len(packages))

	for _, pkg := range packages {
		log.Printf("fetching %s...", pkg.name)
		var err error
		switch pkg.conf.Source() {
		case "github":
			err = fetchGitHub(ctx, client, *cacheDir, pkg.name, pkg.conf, auth)
		case "nodedist":
			err = fetchNodeDist(ctx, client, *cacheDir, pkg.name, pkg.conf)
		default:
			log.Printf("  %s: unknown source %q, skipping", pkg.name, pkg.conf.Source())
			continue
		}
		if err != nil {
			log.Printf("  ERROR: %s: %v", pkg.name, err)
		}
	}
}

type pkgConf struct {
	name string
	conf *installerconf.Conf
}

// discover finds all {dir}/*/releases.conf files and returns them sorted.
func discover(dir string) ([]pkgConf, error) {
	pattern := filepath.Join(dir, "*", "releases.conf")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	var packages []pkgConf
	for _, path := range matches {
		name := filepath.Base(filepath.Dir(path))
		// Skip infrastructure dirs (_example, _webi, _common, etc.)
		if strings.HasPrefix(name, "_") {
			continue
		}
		conf, err := installerconf.Read(path)
		if err != nil {
			log.Printf("warning: %s: %v", path, err)
			continue
		}
		packages = append(packages, pkgConf{name: name, conf: conf})
	}

	sort.Slice(packages, func(i, j int) bool {
		return packages[i].name < packages[j].name
	})

	return packages, nil
}

func fetchNodeDist(ctx context.Context, client *http.Client, cacheRoot, pkgName string, conf *installerconf.Conf) error {
	baseURL := conf.Get("url")
	if baseURL == "" {
		return fmt.Errorf("missing url in releases.conf")
	}

	d, err := rawcache.Open(filepath.Join(cacheRoot, pkgName))
	if err != nil {
		return err
	}

	var added, changed, skipped int
	var latest string
	for batch, err := range nodedist.Fetch(ctx, client, baseURL) {
		if err != nil {
			return fmt.Errorf("%s fetch: %w", pkgName, err)
		}
		for _, entry := range batch {
			tag := entry.Version
			data, err := json.Marshal(entry)
			if err != nil {
				return fmt.Errorf("%s marshal %s: %w", pkgName, tag, err)
			}

			action, err := d.Merge(tag, data)
			if err != nil {
				return err
			}
			switch action {
			case "added":
				added++
			case "changed":
				changed++
			default:
				skipped++
			}

			if latest == "" {
				latest = tag
			}
		}
	}

	if err := updateLatest(d, latest); err != nil {
		return err
	}

	log.Printf("  %s: +%d ~%d =%d latest=%s", pkgName, added, changed, skipped, d.Latest())
	return nil
}

func fetchGitHub(ctx context.Context, client *http.Client, cacheRoot, pkgName string, conf *installerconf.Conf, auth *githubish.Auth) error {
	owner := conf.Get("owner")
	repo := conf.Get("repo")
	tagPrefix := conf.Get("tag_prefix")

	if owner == "" || repo == "" {
		return fmt.Errorf("missing owner or repo in releases.conf")
	}

	d, err := rawcache.Open(filepath.Join(cacheRoot, pkgName))
	if err != nil {
		return err
	}

	var added, changed, skipped int
	var latest string
	for batch, err := range github.Fetch(ctx, client, owner, repo, auth) {
		if err != nil {
			return fmt.Errorf("github %s/%s: %w", owner, repo, err)
		}
		for _, rel := range batch {
			if rel.Draft {
				continue
			}

			tag := rel.TagName

			if tagPrefix != "" {
				if !strings.HasPrefix(tag, tagPrefix) {
					continue
				}
				tag = strings.TrimPrefix(tag, tagPrefix)
			}

			data, err := json.Marshal(rel)
			if err != nil {
				return fmt.Errorf("marshal %s: %w", tag, err)
			}

			action, err := d.Merge(tag, data)
			if err != nil {
				return err
			}
			switch action {
			case "added":
				added++
			case "changed":
				changed++
			default:
				skipped++
			}

			if latest == "" && !rel.Prerelease {
				latest = tag
			}
		}
	}

	if err := updateLatest(d, latest); err != nil {
		return err
	}

	log.Printf("  %s: +%d ~%d =%d latest=%s", pkgName, added, changed, skipped, d.Latest())
	return nil
}

func updateLatest(d *rawcache.Dir, candidate string) error {
	if candidate == "" {
		return nil
	}
	current := d.Latest()
	if current == "" || lexver.Compare(lexver.Parse(candidate), lexver.Parse(current)) > 0 {
		return d.SetLatest(candidate)
	}
	return nil
}
