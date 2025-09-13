package artifact

import (
	"context"
	"time"
)

type Manager interface {
	InstallArtifact(ctx context.Context, pkgName, version string, force bool) error
}

// InstalledManager defines the interface for managing installed packages.
type InstalledManager interface {
	LoadDatabase(dbPath string) error
	SaveDatabase(dbPath string) error
	FindArtifact(name string) *InstalledArtifact
	IsArtifactInstalled(name string) bool
	AddArtifact(pkg *InstalledArtifact)
	RemoveArtifact(name string) bool
	GetInstalledArtifacts() []InstalledArtifact
}

// MetadataExtractor defines the interface for extracting artifact metadata.
type MetadataExtractor interface {
	ExtractMetadata(packagePath string) (*ArtifactMetadata, error)
	ValidateArtifact(packagePath string) error
}

// InstalledArtifact represents an installed artifact with its files.
type InstalledArtifact struct {
	Name          string    `json:"name"`
	Version       string    `json:"version"`
	Description   string    `json:"description"`
	InstalledAt   time.Time `json:"installed_at"`
	InstalledFrom string    `json:"installed_from"` // URL or index where it was installed from
	Files         []string  `json:"files"`          // List of files installed by this artifact
	Checksum      string    `json:"checksum"`       // Checksum of the original artifact
}

// ArtifactMetadata represents metadata about a artifact.
type ArtifactMetadata struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description"`
	Author       string            `json:"author,omitempty"`
	Homepage     string            `json:"homepage,omitempty"`
	License      string            `json:"license,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Conflicts    []string          `json:"conflicts,omitempty"`
	Provides     []string          `json:"provides,omitempty"`
	Architecture string            `json:"architecture,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}
