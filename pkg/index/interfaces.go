//go:generate mockgen -destination=./mocks/index.go . Manager
package index

import (
	"time"

	"github.com/cperrin88/gotya/pkg/model"
)

type Index struct {
	FormatVersion string                           `json:"format_version"`
	LastUpdate    time.Time                        `json:"last_update"`
	Artifacts     []*model.IndexArtifactDescriptor `json:"packages"`
}

// Info represents index information.
type Info struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Enabled  bool   `json:"enabled"`
	Priority int    `json:"priority"`
}

// Manager defines the interface for managing and querying local indexes
type Manager interface {
	// Reload re-reads indexes from disk into memory
	Reload() error

	// FindArtifacts searches for packages by name across all repositories
	FindArtifacts(name string) (map[string][]*model.IndexArtifactDescriptor, error)

	// ResolveArtifact finds a specific package with the given name, version, OS and architecture
	ResolveArtifact(name, version, os, arch string) (*model.IndexArtifactDescriptor, error)

	// GetIndex retrieves an index by name
	GetIndex(name string) (*Index, error)
	ListRepositories() []*Repository
}
