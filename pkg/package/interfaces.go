package pkg

import "time"

// InstalledManager defines the interface for managing installed packages
type InstalledManager interface {
	LoadDatabase(dbPath string) error
	SaveDatabase(dbPath string) error
	FindPackage(name string) *InstalledPackage
	IsPackageInstalled(name string) bool
	AddPackage(pkg InstalledPackage)
	RemovePackage(name string) bool
	GetInstalledPackages() []InstalledPackage
}

// MetadataExtractor defines the interface for extracting package metadata
type MetadataExtractor interface {
	ExtractMetadata(packagePath string) (*PackageMetadata, error)
	ValidatePackage(packagePath string) error
}

// InstalledPackage represents an installed package with its files
type InstalledPackage struct {
	Name          string    `json:"name"`
	Version       string    `json:"version"`
	Description   string    `json:"description"`
	InstalledAt   time.Time `json:"installed_at"`
	InstalledFrom string    `json:"installed_from"` // URL or repository where it was installed from
	Files         []string  `json:"files"`          // List of files installed by this package
	Checksum      string    `json:"checksum"`       // Checksum of the original package
}

// PackageMetadata represents metadata about a package
type PackageMetadata struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description"`
	Author       string            `json:"author,omitempty"`
	License      string            `json:"license,omitempty"`
	Homepage     string            `json:"homepage,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Keywords     []string          `json:"keywords,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}
