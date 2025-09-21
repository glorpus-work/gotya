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
