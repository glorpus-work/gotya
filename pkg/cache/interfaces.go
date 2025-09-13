package cache

import "time"

// Manager defines the interface for cache management operations.
type Manager interface {
	Clean(options CleanOptions) (*CleanResult, error)
	GetInfo() (*Info, error)
	GetDirectory() string
	SetDirectory(dir string) error
}

// CleanOptions specifies what to clean from the cache.
type CleanOptions struct {
	All       bool
	Indexes   bool
	Artifacts bool
}

// CleanResult contains information about what was cleaned.
type CleanResult struct {
	TotalFreed    int64
	IndexFreed    int64
	ArtifactFreed int64
}

// Info represents cache information.
type Info struct {
	Directory     string
	TotalSize     int64
	IndexSize     int64
	IndexFiles    int
	ArtifactSize  int64
	ArtifactFiles int
	LastCleaned   time.Time
}
