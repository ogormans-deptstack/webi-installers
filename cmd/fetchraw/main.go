// Command fetchraw fetches complete release histories from upstream APIs
// and stores them in rawcache. This is a development/bootstrap tool for
// populating the cache with permanent history.
//
// Usage:
//
//	go run ./cmd/fetchraw -cache ./_cache/raw
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
	"strings"
	"time"

	"github.com/webinstall/webi-installers/internal/rawcache"
	"github.com/webinstall/webi-installers/internal/releases/github"
	"github.com/webinstall/webi-installers/internal/releases/githubish"
	"github.com/webinstall/webi-installers/internal/releases/nodedist"
)

func main() {
	cacheDir := flag.String("cache", "_cache/raw", "root directory for raw cache")
	token := flag.String("token", os.Getenv("GITHUB_TOKEN"), "GitHub API token")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	client := &http.Client{Timeout: 30 * time.Second}
	var auth *githubish.Auth
	if *token != "" {
		auth = &githubish.Auth{Token: *token}
	}

	packages := []struct {
		name string
		fn   func(ctx context.Context) error
	}{
		{"node-official", func(ctx context.Context) error {
			return fetchNodeDist(ctx, client, *cacheDir, "node-official", "https://nodejs.org/download/release")
		}},
		{"node-unofficial", func(ctx context.Context) error {
			return fetchNodeDist(ctx, client, *cacheDir, "node-unofficial", "https://unofficial-builds.nodejs.org/download/release")
		}},
		{"hugo", func(ctx context.Context) error {
			return fetchGitHub(ctx, client, *cacheDir, "hugo", "gohugoio", "hugo", "", auth)
		}},
		{"caddy", func(ctx context.Context) error {
			return fetchGitHub(ctx, client, *cacheDir, "caddy", "caddyserver", "caddy", "", auth)
		}},
		{"monorel", func(ctx context.Context) error {
			return fetchGitHub(ctx, client, *cacheDir, "monorel", "therootcompany", "golib", "tools/monorel/", auth)
		}},
	}

	args := flag.Args()
	if len(args) > 0 {
		// Filter to only requested packages.
		nameSet := make(map[string]bool, len(args))
		for _, a := range args {
			nameSet[a] = true
		}
		var filtered []struct {
			name string
			fn   func(ctx context.Context) error
		}
		for _, p := range packages {
			if nameSet[p.name] {
				filtered = append(filtered, p)
			}
		}
		packages = filtered
	}

	for _, pkg := range packages {
		log.Printf("fetching %s...", pkg.name)
		if err := pkg.fn(ctx); err != nil {
			log.Printf("  ERROR: %s: %v", pkg.name, err)
			continue
		}
		log.Printf("  %s done", pkg.name)
	}
}

func fetchNodeDist(ctx context.Context, client *http.Client, cacheRoot, pkgName, baseURL string) error {
	d, err := rawcache.Open(filepath.Join(cacheRoot, pkgName))
	if err != nil {
		return err
	}

	r, err := d.BeginRefresh()
	if err != nil {
		return err
	}

	var count int
	var latest string
	for batch, err := range nodedist.Fetch(ctx, client, baseURL) {
		if err != nil {
			r.Abort()
			return fmt.Errorf("%s fetch: %w", pkgName, err)
		}
		for _, entry := range batch {
			tag := entry.Version
			data, err := json.Marshal(entry)
			if err != nil {
				r.Abort()
				return fmt.Errorf("%s marshal %s: %w", pkgName, tag, err)
			}
			if err := r.Put(tag, data); err != nil {
				r.Abort()
				return err
			}
			count++
			if latest == "" {
				latest = tag
			}
		}
	}

	if latest != "" {
		if err := r.SetLatest(latest); err != nil {
			r.Abort()
			return err
		}
	}

	if err := r.Commit(); err != nil {
		return err
	}

	log.Printf("  %s: %d releases, latest=%s", pkgName, count, latest)
	return nil
}

func fetchGitHub(ctx context.Context, client *http.Client, cacheRoot, pkgName, owner, repo, tagPrefix string, auth *githubish.Auth) error {
	d, err := rawcache.Open(filepath.Join(cacheRoot, pkgName))
	if err != nil {
		return err
	}

	r, err := d.BeginRefresh()
	if err != nil {
		return err
	}

	var count int
	var latest string
	for batch, err := range github.Fetch(ctx, client, owner, repo, auth) {
		if err != nil {
			r.Abort()
			return fmt.Errorf("github %s/%s: %w", owner, repo, err)
		}
		for _, rel := range batch {
			if rel.Draft {
				continue
			}

			tag := rel.TagName

			// Monorepo: skip releases that don't match the prefix,
			// strip the prefix from the tag for storage.
			if tagPrefix != "" {
				if !strings.HasPrefix(tag, tagPrefix) {
					continue
				}
				tag = strings.TrimPrefix(tag, tagPrefix)
			}

			data, err := json.Marshal(rel)
			if err != nil {
				r.Abort()
				return fmt.Errorf("marshal %s: %w", tag, err)
			}
			if err := r.Put(tag, data); err != nil {
				r.Abort()
				return err
			}
			count++
			// First non-prerelease is latest (GitHub returns newest first).
			if latest == "" && !rel.Prerelease {
				latest = tag
			}
		}
	}

	if latest != "" {
		if err := r.SetLatest(latest); err != nil {
			r.Abort()
			return err
		}
	}

	if err := r.Commit(); err != nil {
		return err
	}

	log.Printf("  %s: %d releases, latest=%s", pkgName, count, latest)
	return nil
}

