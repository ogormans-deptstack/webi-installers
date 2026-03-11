// Package fsstore implements [storage.Store] on the local filesystem,
// compatible with the Node.js _cache/ directory format.
//
// Directory layout:
//
//	{root}/
//	  YYYY-MM/
//	    {package}.json          # asset list
//	    {package}.updated.txt   # unix timestamp (seconds.millis)
//
// Write transactions build the new JSON in memory, then atomically
// rename into place so readers never see a partial file.
package fsstore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/webinstall/webi-installers/internal/storage"
)

// Store is a filesystem-backed asset store.
type Store struct {
	root string
}

// Root returns the store's root directory path.
func (s *Store) Root() string {
	return s.root
}

// New creates a Store rooted at the given directory.
// The directory is created if it doesn't exist.
func New(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("fsstore: create root: %w", err)
	}
	return &Store{root: root}, nil
}

// monthDir returns the YYYY-MM subdirectory for the current month.
func monthDir(now time.Time) string {
	return now.Format("2006-01")
}

// ListPackages returns the names of all packages in the current month's cache.
func (s *Store) ListPackages(_ context.Context) ([]string, error) {
	dir := filepath.Join(s.root, monthDir(time.Now()))
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fsstore: list packages: %w", err)
	}
	var pkgs []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			pkgs = append(pkgs, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return pkgs, nil
}

// Load reads a package's cached assets from disk.
// Returns nil (not an error) if the package is not cached.
func (s *Store) Load(_ context.Context, pkg string) (*storage.PackageData, error) {
	dir := filepath.Join(s.root, monthDir(time.Now()))
	jsonPath := filepath.Join(dir, pkg+".json")

	data, err := os.ReadFile(jsonPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fsstore: read %s: %w", pkg, err)
	}

	// Decode via legacy format (Node.js compat: "releases", "name", "ext").
	var lc storage.LegacyCache
	if err := json.Unmarshal(data, &lc); err != nil {
		return nil, fmt.Errorf("fsstore: decode %s: %w", pkg, err)
	}
	pd := storage.ImportLegacy(lc)

	// Read the timestamp file.
	tsPath := filepath.Join(dir, pkg+".updated.txt")
	if tsData, err := os.ReadFile(tsPath); err == nil {
		pd.UpdatedAt = parseTimestamp(strings.TrimSpace(string(tsData)))
	}

	return &pd, nil
}

// BeginRefresh starts a write transaction for a package.
func (s *Store) BeginRefresh(_ context.Context, pkg string) (storage.RefreshTx, error) {
	return &refreshTx{
		store: s,
		pkg:   pkg,
	}, nil
}

type refreshTx struct {
	store  *Store
	pkg    string
	assets []storage.Asset
}

func (tx *refreshTx) Put(assets []storage.Asset) error {
	tx.assets = append(tx.assets, assets...)
	return nil
}

func (tx *refreshTx) Commit(_ context.Context) error {
	now := time.Now()
	dir := filepath.Join(tx.store.root, monthDir(now))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("fsstore: mkdir: %w", err)
	}

	// Encode via legacy format (Node.js compat: "releases", "name", "ext").
	lc := storage.ExportLegacy(storage.PackageData{Assets: tx.assets})

	data, err := json.MarshalIndent(lc, "", "  ")
	if err != nil {
		return fmt.Errorf("fsstore: encode %s: %w", tx.pkg, err)
	}

	// Write JSON atomically via temp file + rename.
	jsonPath := filepath.Join(dir, tx.pkg+".json")
	if err := atomicWrite(jsonPath, data); err != nil {
		return err
	}

	// Write timestamp file.
	tsPath := filepath.Join(dir, tx.pkg+".updated.txt")
	ts := fmt.Sprintf("%.3f", float64(now.UnixMilli())/1000.0)
	if err := atomicWrite(tsPath, []byte(ts)); err != nil {
		return err
	}

	tx.assets = nil
	return nil
}

func (tx *refreshTx) Rollback() error {
	tx.assets = nil
	return nil
}

// atomicWrite writes data to path via a temp file + rename.
func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("fsstore: write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("fsstore: rename: %w", err)
	}
	return nil
}

// LinkAlias creates symlinks so that alias resolves to the same cache
// files as target: alias.json → target.json, alias.updated.txt → target.updated.txt.
func (s *Store) LinkAlias(alias, target string) error {
	dir := filepath.Join(s.root, monthDir(time.Now()))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("fsstore: mkdir: %w", err)
	}

	for _, ext := range []string{".json", ".updated.txt"} {
		link := filepath.Join(dir, alias+ext)
		dest := target + ext // relative symlink within same dir

		// Remove existing link/file so we can recreate it.
		os.Remove(link)
		if err := os.Symlink(dest, link); err != nil {
			return fmt.Errorf("fsstore: symlink %s → %s: %w", alias+ext, dest, err)
		}
	}
	return nil
}

// parseTimestamp parses the "seconds.millis" format from .updated.txt files.
func parseTimestamp(s string) time.Time {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f == 0 {
		return time.Time{}
	}
	sec := int64(f)
	nsec := int64((f - float64(sec)) * 1e9)
	return time.Unix(sec, nsec)
}
