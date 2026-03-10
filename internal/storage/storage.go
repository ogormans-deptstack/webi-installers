// Package storage defines the interface for reading and writing
// classified release assets.
//
// webid reads assets through [Store]. webicached writes them through
// [RefreshTx], obtained from [Store.BeginRefresh].
//
// The two implementations are fsstore (filesystem JSON, compatible with
// the Node.js _cache/ format) and pgstore (PostgreSQL, future).
package storage

import (
	"context"
	"time"
)

// Asset is a single downloadable file — one entry in a release.
// A release like "bat v0.26.1" has many assets (one per platform/format).
//
// No JSON tags — serialization goes through [LegacyAsset] for Node.js
// compat, or through a future v2 format.
type Asset struct {
	Filename string
	Version  string
	LTS      bool
	Channel  string
	Date     string
	OS       string
	Arch     string
	Libc     string
	Format   string
	Download string
	Extra    string   // extra version info for sorting (e.g. build metadata)
	Variants []string // build qualifiers: "installer", "rocm", "jetpack5", "fxdependent", etc.
}

// PackageData is the full set of assets for a package, plus metadata.
type PackageData struct {
	Assets    []Asset
	UpdatedAt time.Time
}

// Store is the read/write interface for release asset storage.
type Store interface {
	// Load returns all assets for a package, or nil if the package
	// is not cached. The returned data may be stale — check UpdatedAt.
	Load(ctx context.Context, pkg string) (*PackageData, error)

	// BeginRefresh starts a write transaction for a package.
	// Write assets via [RefreshTx.Put], then call Commit to atomically
	// replace the stored data. Call Rollback to discard.
	BeginRefresh(ctx context.Context, pkg string) (RefreshTx, error)
}

// VariantTagger applies package-specific variant tags to classified assets.
// Implementations live in per-installer packages (e.g. internal/releases/bun).
type VariantTagger interface {
	TagVariants(assets []Asset)
}

// RefreshTx is a write transaction for replacing a package's assets.
type RefreshTx interface {
	// Put stages assets to be written. May be called multiple times
	// to append assets incrementally.
	Put(assets []Asset) error

	// Commit atomically replaces the package's stored assets with
	// everything staged via Put.
	Commit(ctx context.Context) error

	// Rollback discards all staged data.
	Rollback() error
}
