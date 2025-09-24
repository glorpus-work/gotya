package artifact

import (
	"context"
	"time"

	"github.com/cperrin88/gotya/pkg/model"
)

type Manager interface {
	// InstallArtifact installs (verifies/stages) an artifact strictly from a local file.
	// The descriptor must describe the artifact and localPath must point to the local archive file.
	InstallArtifact(ctx context.Context, desc *model.IndexArtifactDescriptor, localPath string) error
}

// InstalledManager defines the interface for managing installed packages.
type InstalledManager interface {
	LoadDatabase(dbPath string) error
	SaveDatabase(dbPath string) error
	FindArtifact(name string) *InstalledArtifact
	IsArtifactInstalled(name string) bool
	AddArtifact(pkg *InstalledArtifact)
	RemoveArtifact(name string) bool
	GetInstalledArtifacts() []*InstalledArtifact
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
