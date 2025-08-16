package repository

import (
	"context"
	"time"
)

// Manager defines the interface for repository management operations
type Manager interface {
	AddRepository(name, url string) error
	RemoveRepository(name string) error
	EnableRepository(name string, enabled bool) error
	DisableRepository(name string) error
	ListRepositories() []Info
	GetRepository(name string) *Info
	GetRepositoryIndex(name string) (Index, error)
	SyncRepository(ctx context.Context, name string) error
	SyncRepositories(ctx context.Context) error
	IsCacheStale(name string, maxAge time.Duration) bool
	GetCacheAge(name string) (time.Duration, error)
	SyncIfStale(ctx context.Context, name string, maxAge time.Duration) error
}

// Info represents repository information
type Info struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Enabled  bool   `json:"enabled"`
	Priority int    `json:"priority"`
}

// Index represents the repository index interface
type Index interface {
	GetFormatVersion() string
	GetLastUpdate() string
	GetPackages() []Package
	FindPackage(name string) *Package
	AddPackage(pkg Package)
	RemovePackage(name string) bool
}

// Package represents a package in a repository
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
