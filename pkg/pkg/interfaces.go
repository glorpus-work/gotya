package pkg

import (
	"context"
	"time"
)

type Manager interface {
	InstallPackage(ctx context.Context, pkgName, version string, force bool) error
}

// InstalledManager defines the interface for managing installed packages.
type InstalledManager interface {
	LoadDatabase(dbPath string) error
	SaveDatabase(dbPath string) error
	FindPackage(name string) *InstalledPackage
	IsPackageInstalled(name string) bool
	AddPackage(pkg *InstalledPackage)
	RemovePackage(name string) bool
	GetInstalledPackages() []InstalledPackage
}

// MetadataExtractor defines the interface for extracting pkg metadata.
type MetadataExtractor interface {
	ExtractMetadata(packagePath string) (*PackageMetadata, error)
	ValidatePackage(packagePath string) error
}

// InstalledPackage represents an installed pkg with its files.
type InstalledPackage struct {
	Name          string    `json:"name"`
	Version       string    `json:"version"`
	Description   string    `json:"description"`
	InstalledAt   time.Time `json:"installed_at"`
	InstalledFrom string    `json:"installed_from"` // URL or index where it was installed from
	Files         []string  `json:"files"`          // List of files installed by this pkg
	Checksum      string    `json:"checksum"`       // Checksum of the original pkg
}

// PackageMetadata represents metadata about a pkg.
type PackageMetadata struct {
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
