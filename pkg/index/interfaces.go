//go:generate mockgen -destination=./mocks/index.go . Manager
package index

import (
	"context"
	"time"

	"github.com/cperrin88/gotya/pkg/repository"
)

type Index struct {
	FormatVersion string      `json:"format_version"`
	LastUpdate    time.Time   `json:"last_update"`
	Artifacts     []*Artifact `json:"packages"`
}

// Info represents index information.
type Info struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Enabled  bool   `json:"enabled"`
	Priority int    `json:"priority"`
}

// Manager defines the interface for managing package indexes
type Manager interface {
	// Sync updates the index for a specific repository
	Sync(ctx context.Context, name string) error

	// SyncAll updates all repository indexes
	SyncAll(ctx context.Context) error

	// IsCacheStale checks if the cache for a repository is stale
	IsCacheStale(name string) bool

	// GetCacheAge returns the age of the cache for a repository
	GetCacheAge(name string) (time.Duration, error)

	// FindArtifacts searches for packages by name across all repositories
	FindArtifacts(name string) (map[string][]*Artifact, error)

	// ResolveArtifact finds a specific package with the given name, version, OS and architecture
	ResolveArtifact(name, version, os, arch string) (*Artifact, error)

	// GetIndex retrieves an index by name
	GetIndex(name string) (*Index, error)
	ListRepositories() []*repository.Repository
}
