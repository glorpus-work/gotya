package repository

import (
	"context"
	"time"
)

// RepositoryManager defines the core interface for repository management operations.
// It composes smaller, more focused interfaces to provide all repository functionality.
type RepositoryManager interface {
	RepositoryLister
	RepositorySynchronizer
	RepositoryQuerier
}

// RepositoryLister defines operations for listing and managing repository metadata.
type RepositoryLister interface {
	AddRepository(name, url string) error
	RemoveRepository(name string) error
	EnableRepository(name string, enabled bool) error
	DisableRepository(name string) error
	ListRepositories() []Info
	GetRepository(name string) *Info
}

// RepositorySynchronizer defines operations for synchronizing repository data.
type RepositorySynchronizer interface {
	SyncRepository(ctx context.Context, name string) error
	SyncRepositories(ctx context.Context) error
	IsCacheStale(name string, maxAge time.Duration) bool
	GetCacheAge(name string) (time.Duration, error)
	SyncIfStale(ctx context.Context, name string, maxAge time.Duration) error
}

// RepositoryQuerier defines operations for querying repository data.
type RepositoryQuerier interface {
	GetRepositoryIndex(name string) (*IndexImpl, error)
	FindPackage(name string) (*Package, error)
}

// Info represents repository information.
type Info struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Enabled  bool   `json:"enabled"`
	Priority int    `json:"priority"`
}

// Index represents the repository index interface.
type Index interface {
	GetFormatVersion() string
	GetLastUpdate() string
	GetPackages() []Package
	FindPackage(name string) *Package
	AddPackage(pkg *Package)
	RemovePackage(name string) bool
}

// Package represents a package in a repository.
type Package struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description"`
	URL          string            `json:"url"`
	Checksum     string            `json:"checksum"`
	Size         int64             `json:"size"`
	OS           string            `json:"os,omitempty"`
	Arch         string            `json:"arch,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}
