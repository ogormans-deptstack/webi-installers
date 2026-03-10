// Command comparecache compares Go-generated cache output against the
// Node.js LIVE_cache. It identifies categorical differences in asset
// selection — which filenames appear in one cache but not the other.
//
// The comparison is done at the filename level (not OS/arch/ext fields)
// because the Node.js cache leaves those empty (normalize.js fills them
// at serve time), while the Go pipeline classifies at write time.
//
// Usage:
//
//	go run ./cmd/comparecache -live ./LIVE_cache -go ./_cache
//	go run ./cmd/comparecache -live ./LIVE_cache -go ./_cache bat jq
//	go run ./cmd/comparecache -live ./LIVE_cache -go ./_cache -summary
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type cacheEntry struct {
	Releases []struct {
		Name      string `json:"name"`
		Filename  string `json:"_filename"` // Node.js uses _filename for some sources
		Version   string `json:"version"`
		Download  string `json:"download"`
		Channel   string `json:"channel"`
		OS        string `json:"os"`
		Arch      string `json:"arch"`
		Ext       string `json:"ext"`
	} `json:"releases"`
}

type packageDiff struct {
	Name         string
	LiveCount    int
	GoCount      int
	OnlyInLive   []string // filenames only in Node.js cache
	OnlyInGo     []string // filenames only in Go cache
	VersionsLive []string // unique versions in live
	VersionsGo   []string // unique versions in go
	GoMissing    bool     // true if Go didn't produce output for this package
	LiveMissing  bool     // true if no live cache for this package
	Categories   []string // categorical difference labels
}

func main() {
	liveDir := flag.String("live", "./LIVE_cache", "path to Node.js LIVE_cache directory")
	goDir := flag.String("go", "./_cache", "path to Go cache directory")
	summary := flag.Bool("summary", false, "only print summary, not per-package details")
	latest := flag.Bool("latest", false, "only compare latest version in each cache")
	flag.Parse()
	filterPkgs := flag.Args()

	// Find the most recent month directory in each cache.
	liveMonth := findLatestMonth(*liveDir)
	goMonth := findLatestMonth(*goDir)
	if liveMonth == "" {
		log.Fatalf("no month directories found in %s", *liveDir)
	}

	livePath := filepath.Join(*liveDir, liveMonth)
	goPath := ""
	if goMonth != "" {
		goPath = filepath.Join(*goDir, goMonth)
	}

	// Discover all packages across both caches.
	allPkgs := discoverPackages(livePath, goPath)
	if len(filterPkgs) > 0 {
		nameSet := make(map[string]bool, len(filterPkgs))
		for _, n := range filterPkgs {
			nameSet[n] = true
		}
		var filtered []string
		for _, p := range allPkgs {
			if nameSet[p] {
				filtered = append(filtered, p)
			}
		}
		allPkgs = filtered
	}

	var diffs []packageDiff
	for _, pkg := range allPkgs {
		d := compare(livePath, goPath, pkg, *latest)
		categorize(&d)
		diffs = append(diffs, d)
	}

	if *summary {
		printSummary(diffs)
	} else {
		printDetails(diffs)
	}
}

func findLatestMonth(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var months []string
	for _, e := range entries {
		if e.IsDir() && len(e.Name()) == 7 && e.Name()[4] == '-' {
			months = append(months, e.Name())
		}
	}
	if len(months) == 0 {
		return ""
	}
	sort.Strings(months)
	return months[len(months)-1]
}

func discoverPackages(livePath, goPath string) []string {
	seen := make(map[string]bool)
	for _, dir := range []string{livePath, goPath} {
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if strings.HasSuffix(name, ".json") && !strings.HasSuffix(name, ".updated.txt") {
				pkg := strings.TrimSuffix(name, ".json")
				seen[pkg] = true
			}
		}
	}
	var pkgs []string
	for p := range seen {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	return pkgs
}

func loadCache(dir, pkg string) *cacheEntry {
	if dir == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(dir, pkg+".json"))
	if err != nil {
		return nil
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil
	}
	return &entry
}

// effectiveName returns the best available filename for a release entry.
// Node.js sometimes uses _filename (a path) instead of name.
func effectiveName(name, filename, download string) string {
	if name != "" {
		return name
	}
	if filename != "" {
		// _filename may be a path like "stable/macos/flutter_macos_3.41.4.zip"
		if i := strings.LastIndex(filename, "/"); i >= 0 {
			return filename[i+1:]
		}
		return filename
	}
	// Last resort: basename of download URL.
	if download != "" {
		if i := strings.LastIndex(download, "/"); i >= 0 {
			return download[i+1:]
		}
	}
	return ""
}

func compare(livePath, goPath, pkg string, latestOnly bool) packageDiff {
	live := loadCache(livePath, pkg)
	goCache := loadCache(goPath, pkg)

	d := packageDiff{Name: pkg}

	if live == nil {
		d.LiveMissing = true
	}
	if goCache == nil {
		d.GoMissing = true
	}
	if d.LiveMissing && d.GoMissing {
		return d
	}

	// Collect filenames by version.
	type versionFiles struct {
		version string
		files   map[string]bool
	}

	extractVersionFiles := func(ce *cacheEntry) (map[string]map[string]bool, []string) {
		vf := make(map[string]map[string]bool)
		for _, r := range ce.Releases {
			if vf[r.Version] == nil {
				vf[r.Version] = make(map[string]bool)
			}
			vf[r.Version][effectiveName(r.Name, r.Filename, r.Download)] = true
		}
		var versions []string
		for v := range vf {
			versions = append(versions, v)
		}
		sort.Strings(versions)
		return vf, versions
	}

	var liveFiles, goFiles map[string]bool

	if live != nil {
		vf, versions := extractVersionFiles(live)
		d.VersionsLive = versions
		d.LiveCount = len(live.Releases)

		if latestOnly && len(versions) > 0 {
			liveFiles = vf[versions[len(versions)-1]]
		} else {
			liveFiles = make(map[string]bool)
			for _, r := range live.Releases {
				liveFiles[effectiveName(r.Name, r.Filename, r.Download)] = true
			}
		}
	} else {
		liveFiles = make(map[string]bool)
	}

	if goCache != nil {
		vf, versions := extractVersionFiles(goCache)
		d.VersionsGo = versions
		d.GoCount = len(goCache.Releases)

		if latestOnly && len(versions) > 0 {
			goFiles = vf[versions[len(versions)-1]]
		} else {
			goFiles = make(map[string]bool)
			for _, r := range goCache.Releases {
				goFiles[effectiveName(r.Name, r.Filename, r.Download)] = true
			}
		}
	} else {
		goFiles = make(map[string]bool)
	}

	for f := range liveFiles {
		if !goFiles[f] {
			d.OnlyInLive = append(d.OnlyInLive, f)
		}
	}
	for f := range goFiles {
		if !liveFiles[f] {
			d.OnlyInGo = append(d.OnlyInGo, f)
		}
	}
	sort.Strings(d.OnlyInLive)
	sort.Strings(d.OnlyInGo)

	return d
}

func categorize(d *packageDiff) {
	if d.GoMissing {
		d.Categories = append(d.Categories, "go-missing")
		return
	}
	if d.LiveMissing {
		d.Categories = append(d.Categories, "live-missing")
		return
	}

	if len(d.OnlyInLive) == 0 && len(d.OnlyInGo) == 0 {
		d.Categories = append(d.Categories, "match")
		return
	}

	// Check if differences are only version depth (Go has more history).
	liveVersionSet := make(map[string]bool, len(d.VersionsLive))
	for _, v := range d.VersionsLive {
		liveVersionSet[v] = true
	}
	goVersionSet := make(map[string]bool, len(d.VersionsGo))
	for _, v := range d.VersionsGo {
		goVersionSet[v] = true
	}

	goExtraVersions := 0
	for _, v := range d.VersionsGo {
		if !liveVersionSet[v] {
			goExtraVersions++
		}
	}
	liveExtraVersions := 0
	for _, v := range d.VersionsLive {
		if !goVersionSet[v] {
			liveExtraVersions++
		}
	}

	if goExtraVersions > 0 {
		d.Categories = append(d.Categories, fmt.Sprintf("go-extra-versions(%d)", goExtraVersions))
	}
	if liveExtraVersions > 0 {
		d.Categories = append(d.Categories, fmt.Sprintf("live-extra-versions(%d)", liveExtraVersions))
	}

	// Check for meta-asset filtering differences.
	metaOnlyInLive := 0
	nonMetaOnlyInLive := 0
	for _, f := range d.OnlyInLive {
		if isMetaFile(f) {
			metaOnlyInLive++
		} else {
			nonMetaOnlyInLive++
		}
	}
	metaOnlyInGo := 0
	nonMetaOnlyInGo := 0
	for _, f := range d.OnlyInGo {
		if isMetaFile(f) {
			metaOnlyInGo++
		} else {
			nonMetaOnlyInGo++
		}
	}

	if metaOnlyInLive > 0 {
		d.Categories = append(d.Categories, fmt.Sprintf("live-has-meta(%d)", metaOnlyInLive))
	}
	if metaOnlyInGo > 0 {
		d.Categories = append(d.Categories, fmt.Sprintf("go-has-meta(%d)", metaOnlyInGo))
	}

	// Check for source tarball differences.
	srcOnlyInGo := 0
	for _, f := range d.OnlyInGo {
		if strings.HasSuffix(f, ".tar.gz") || strings.HasSuffix(f, ".zip") {
			if strings.HasPrefix(f, "v") || strings.HasPrefix(f, "refs/") {
				srcOnlyInGo++
			}
		}
	}
	if srcOnlyInGo > 0 {
		d.Categories = append(d.Categories, fmt.Sprintf("go-has-source-tarballs(%d)", srcOnlyInGo))
	}

	if nonMetaOnlyInLive > 0 {
		d.Categories = append(d.Categories, fmt.Sprintf("live-extra-assets(%d)", nonMetaOnlyInLive))
	}
	if nonMetaOnlyInGo > 0 {
		d.Categories = append(d.Categories, fmt.Sprintf("go-extra-assets(%d)", nonMetaOnlyInGo))
	}
}

func isMetaFile(name string) bool {
	lower := strings.ToLower(name)
	for _, suffix := range []string{
		".sha256", ".sha256sum", ".sha512", ".sha512sum",
		".md5", ".md5sum", ".sig", ".asc", ".pem",
		"checksums.txt", "sha256sums", "sha512sums",
		".sbom", ".spdx", ".json.sig", ".sigstore",
		".d.ts", ".pub",
	} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	for _, contains := range []string{
		"checksums", "sha256sum", "sha512sum",
	} {
		if strings.Contains(lower, contains) {
			return true
		}
	}
	return false
}

func printSummary(diffs []packageDiff) {
	// Count by category.
	categoryCounts := make(map[string]int)
	for _, d := range diffs {
		for _, c := range d.Categories {
			// Strip the count suffix for grouping.
			base := c
			if idx := strings.Index(c, "("); idx != -1 {
				base = c[:idx]
			}
			categoryCounts[base]++
		}
	}

	fmt.Println("=== COMPARISON SUMMARY ===")
	fmt.Printf("Total packages: %d\n\n", len(diffs))

	var cats []string
	for c := range categoryCounts {
		cats = append(cats, c)
	}
	sort.Strings(cats)
	for _, c := range cats {
		fmt.Printf("  %-30s %d\n", c, categoryCounts[c])
	}

	fmt.Println("\n=== PER-PACKAGE CATEGORIES ===")
	for _, d := range diffs {
		fmt.Printf("%-25s %s\n", d.Name, strings.Join(d.Categories, ", "))
	}
}

func printDetails(diffs []packageDiff) {
	for _, d := range diffs {
		fmt.Printf("=== %s ===\n", d.Name)
		fmt.Printf("  Categories: %s\n", strings.Join(d.Categories, ", "))
		fmt.Printf("  Live: %d assets, %d versions  |  Go: %d assets, %d versions\n",
			d.LiveCount, len(d.VersionsLive), d.GoCount, len(d.VersionsGo))

		if len(d.OnlyInLive) > 0 {
			fmt.Printf("  Only in LIVE (%d):\n", len(d.OnlyInLive))
			for _, f := range d.OnlyInLive {
				if len(d.OnlyInLive) > 20 {
					fmt.Printf("    - %s\n", f)
					if f == d.OnlyInLive[19] {
						fmt.Printf("    ... and %d more\n", len(d.OnlyInLive)-20)
						break
					}
				} else {
					fmt.Printf("    - %s\n", f)
				}
			}
		}

		if len(d.OnlyInGo) > 0 {
			fmt.Printf("  Only in Go (%d):\n", len(d.OnlyInGo))
			for _, f := range d.OnlyInGo {
				if len(d.OnlyInGo) > 20 {
					fmt.Printf("    - %s\n", f)
					if f == d.OnlyInGo[19] {
						fmt.Printf("    ... and %d more\n", len(d.OnlyInGo)-20)
						break
					}
				} else {
					fmt.Printf("    - %s\n", f)
				}
			}
		}

		fmt.Println()
	}
}
