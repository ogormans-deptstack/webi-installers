// Command fetchraw fetches release histories from upstream APIs and
// merges them into rawcache. Safe to run repeatedly — unchanged releases
// are skipped, new/changed ones are recorded in the audit log.
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
	"strings"
	"time"

	"github.com/webinstall/webi-installers/internal/lexver"
	"github.com/webinstall/webi-installers/internal/rawcache"
	"github.com/webinstall/webi-installers/internal/releases/github"
	"github.com/webinstall/webi-installers/internal/releases/githubish"
	"github.com/webinstall/webi-installers/internal/releases/nodedist"
)

type pkg struct {
	name string
	fn   func(ctx context.Context) error
}

func main() {
	cacheDir := flag.String("cache", "_cache/raw", "root directory for raw cache")
	token := flag.String("token", os.Getenv("GITHUB_TOKEN"), "GitHub API token")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	client := &http.Client{Timeout: 30 * time.Second}
	var auth *githubish.Auth
	if *token != "" {
		auth = &githubish.Auth{Token: *token}
	}

	gh := func(name, owner, repo string) pkg {
		return pkg{name, func(ctx context.Context) error {
			return fetchGitHub(ctx, client, *cacheDir, name, owner, repo, "", auth)
		}}
	}
	ghMono := func(name, owner, repo, prefix string) pkg {
		return pkg{name, func(ctx context.Context) error {
			return fetchGitHub(ctx, client, *cacheDir, name, owner, repo, prefix, auth)
		}}
	}
	nodeDist := func(name, baseURL string) pkg {
		return pkg{name, func(ctx context.Context) error {
			return fetchNodeDist(ctx, client, *cacheDir, name, baseURL)
		}}
	}

	packages := []pkg{
		// Node.js
		nodeDist("node-official", "https://nodejs.org/download/release"),
		nodeDist("node-unofficial", "https://unofficial-builds.nodejs.org/download/release"),

		// GitHub packages (alphabetical)
		gh("arc", "mholt", "archiver"),
		gh("atomicparsley", "wez", "atomicparsley"),
		gh("bat", "sharkdp", "bat"),
		gh("bun", "oven-sh", "bun"),
		gh("caddy", "caddyserver", "caddy"),
		gh("cilium", "cilium", "cilium-cli"),
		gh("cmake", "Kitware", "CMake"),
		gh("comrak", "kivikakk", "comrak"),
		gh("crabz", "sstadick", "crabz"),
		gh("curlie", "rs", "curlie"),
		gh("dashcore", "dashpay", "dash"),
		gh("dashmsg", "dashhive", "dashmsg"),
		gh("delta", "dandavison", "delta"),
		gh("deno", "denoland", "deno"),
		gh("dotenv", "therootcompany", "dotenv"),
		gh("dotenv-linter", "dotenv-linter", "dotenv-linter"),
		gh("fd", "sharkdp", "fd"),
		gh("ffmpeg", "eugeneware", "ffmpeg-static"),
		gh("ffuf", "ffuf", "ffuf"),
		gh("fish", "fish-shell", "fish-shell"),
		gh("fzf", "junegunn", "fzf"),
		gh("gh", "cli", "cli"),
		gh("git", "git-for-windows", "git"),
		gh("gitdeploy", "therootcompany", "gitdeploy"),
		gh("gitea", "go-gitea", "gitea"),
		gh("goreleaser", "goreleaser", "goreleaser"),
		gh("gprox", "creedasaurus", "gprox"),
		gh("grype", "anchore", "grype"),
		gh("hexyl", "sharkdp", "hexyl"),
		gh("hugo", "gohugoio", "hugo"),
		gh("jq", "stedolan", "jq"),
		gh("k9s", "derailed", "k9s"),
		gh("keypairs", "therootcompany", "keypairs"),
		gh("kind", "kubernetes-sigs", "kind"),
		gh("koji", "cococonscious", "koji"),
		gh("kubectx", "ahmetb", "kubectx"),
		gh("lf", "gokcehan", "lf"),
		gh("lsd", "lsd-rs", "lsd"),
		gh("mutagen", "mutagen-io", "mutagen"),
		gh("ollama", "jmorganca", "ollama"),
		gh("ots", "emdneto", "otsgo"),
		gh("pandoc", "jgm", "pandoc"),
		gh("pg", "bnnanet", "postgresql-releases"),
		gh("pwsh", "powershell", "powershell"),
		gh("rclone", "rclone", "rclone"),
		gh("ripgrep", "BurntSushi", "ripgrep"),
		gh("runzip", "therootcompany", "runzip"),
		gh("sass", "sass", "dart-sass"),
		gh("sclient", "therootcompany", "sclient"),
		gh("sd", "chmln", "sd"),
		gh("serviceman", "bnnanet", "serviceman"),
		gh("shellcheck", "koalaman", "shellcheck"),
		gh("shfmt", "mvdan", "sh"),
		gh("sqlc", "sqlc-dev", "sqlc"),
		gh("sqlpkg", "nalgeon", "sqlpkg-cli"),
		gh("sttr", "abhimanyu003", "sttr"),
		gh("syncthing", "syncthing", "syncthing"),
		gh("terramate", "terramate-io", "terramate"),
		gh("tinygo", "tinygo-org", "tinygo"),
		gh("trip", "fujiapple852", "trippy"),
		gh("uuidv7", "coolaj86", "uuidv7"),
		gh("watchexec", "watchexec", "watchexec"),
		gh("xcaddy", "caddyserver", "xcaddy"),
		gh("xsv", "BurntSushi", "xsv"),
		gh("xz", "therootcompany", "xz-static"),
		gh("yq", "mikefarah", "yq"),
		gh("zoxide", "ajeetdsouza", "zoxide"),

		// Monorepo
		ghMono("monorel", "therootcompany", "golib", "tools/monorel/"),
	}

	args := flag.Args()
	if len(args) > 0 {
		nameSet := make(map[string]bool, len(args))
		for _, a := range args {
			nameSet[a] = true
		}
		var filtered []pkg
		for _, p := range packages {
			if nameSet[p.name] {
				filtered = append(filtered, p)
			}
		}
		packages = filtered
	}

	for _, p := range packages {
		log.Printf("fetching %s...", p.name)
		if err := p.fn(ctx); err != nil {
			log.Printf("  ERROR: %s: %v", p.name, err)
			continue
		}
	}
}

func fetchNodeDist(ctx context.Context, client *http.Client, cacheRoot, pkgName, baseURL string) error {
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

func fetchGitHub(ctx context.Context, client *http.Client, cacheRoot, pkgName, owner, repo, tagPrefix string, auth *githubish.Auth) error {
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
