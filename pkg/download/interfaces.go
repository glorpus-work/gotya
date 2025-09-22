package download

import (
	"context"
	"net/url"
)

// Manager defines the interface for downloading remote artifacts (indexes, packages, etc.).
// It is designed to replace ad-hoc HTTP downloading with a higher-level, testable API
// that supports batching, de-duplication and integrity verification.
type Manager interface {
	// FetchAll downloads all items, respecting Options (e.g., concurrency and cache dir).
	// It returns a map from Item.ID to absolute local file path.
	FetchAll(ctx context.Context, items []Item, opts Options) (map[string]string, error)

	// Fetch downloads a single item to a deterministic location (within opts.Dir).
	// It returns the absolute local file path.
	Fetch(ctx context.Context, item Item, opts Options) (string, error)
}

// Item represents one remote resource to download.
type Item struct {
	ID       string   // stable identifier (e.g., artifact id). Must be unique within a batch.
	URL      *url.URL // source URL to download
	Checksum string   // optional hex-encoded SHA-256 checksum; if provided, will be verified
	Filename string   // optional preferred filename; if empty, a name will be derived
}

// Options control the behavior of the download manager.
type Options struct {
	Dir         string // destination directory (cache). Must be absolute.
	Concurrency int    // number of parallel downloads; if <=0, a sane default is used
}
