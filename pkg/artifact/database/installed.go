package database

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/cperrin88/gotya/pkg/errors"
)

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

// InstalledDatabase represents the database of installed packages.
type InstalledDatabase struct {
	FormatVersion string               `json:"format_version"`
	LastUpdate    time.Time            `json:"last_update"`
	Artifacts     []*InstalledArtifact `json:"artifacts"`
}

const (
	// InitialArtifactCapacity is the initial capacity for the installed packages slice.
	InitialArtifactCapacity = 100
)

// NewInstalledDatabase creates a new installed packages database.
func NewInstalledDatabase() *InstalledDatabase {
	return &InstalledDatabase{
		FormatVersion: "1",
		LastUpdate:    time.Now(),
		Artifacts:     make([]*InstalledArtifact, 0, InitialArtifactCapacity),
	}
}

// LoadDatabase loads the installed packages database from file.
func (installedDB *InstalledDatabase) LoadDatabase(dbPath string) error {
	// Clean and validate the database path
	cleanPath := filepath.Clean(dbPath)
	if !filepath.IsAbs(cleanPath) {
		return fmt.Errorf("database path must be absolute: %s: %w", dbPath, errors.ErrInvalidPath)
	}

	// Check if file exists with cleaned path
	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		// Database doesn't exist, return new empty database
		return nil
	}

	// Open the file with the cleaned path
	file, err := os.Open(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to open database file: %w", err)
	}
	defer file.Close()

	return installedDB.parseInstalledDatabaseFromReader(file)
}

// parseInstalledDatabaseFromReader parses the database from an io.Reader.
func (installedDB *InstalledDatabase) parseInstalledDatabaseFromReader(reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read database: %w", err)
	}

	// First unmarshal into a temporary struct
	var tempDB InstalledDatabase
	if err := json.Unmarshal(data, &tempDB); err != nil {
		return fmt.Errorf("failed to parse database: %w", err)
	}

	// Convert each artifact to a pointer
	for i := range tempDB.Artifacts {
		pkg := tempDB.Artifacts[i] // Create a copy in the loop
		installedDB.Artifacts = append(installedDB.Artifacts, pkg)
	}

	return nil
}

// SaveDatabase saves the installed packages database to file.
func (installedDB *InstalledDatabase) SaveDatabase(dbPath string) (err error) {
	// Clean and validate the database path
	cleanPath := filepath.Clean(dbPath)
	if !filepath.IsAbs(cleanPath) {
		return fmt.Errorf("database path must be absolute: %s: %w", dbPath, errors.ErrInvalidPath)
	}

	// Get the directory of the database file
	dbDir := filepath.Dir(cleanPath)

	// Create a temporary file for atomic write
	tmpFile, err := os.CreateTemp(dbDir, "gotya-db-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary file in %s: %w", dbDir, err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if err != nil {
			// Clean up the temporary file if there was an error
			_ = os.Remove(tmpPath)
		}
	}()

	// Convert to JSON
	data, err := json.MarshalIndent(installedDB, "", "  ")
	if err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to marshal database to JSON: %w", err)
	}

	// Write to temporary file
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write to temporary file: %w", err)
	}

	// Ensure the data is written to disk
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to sync temporary file to disk: %w", err)
	}

	// Close the temporary file
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Atomically rename the temporary file to the target file
	if err := os.Rename(tmpPath, cleanPath); err != nil {
		return fmt.Errorf("failed to rename temporary file to %s: %w", cleanPath, err)
	}

	return nil
}

// FindArtifact finds an installed artifact by name.
func (installedDB *InstalledDatabase) FindArtifact(name string) *InstalledArtifact {
	for _, pkg := range installedDB.Artifacts {
		if pkg.Name == name {
			return pkg
		}
	}
	return nil
}

// IsArtifactInstalled checks if a artifact is installed.
func (installedDB *InstalledDatabase) IsArtifactInstalled(name string) bool {
	return installedDB.FindArtifact(name) != nil
}

// AddArtifact adds an installed artifact to the database.
func (installedDB *InstalledDatabase) AddArtifact(pkg *InstalledArtifact) {
	// Remove existing artifact with same name if it exists
	for i, existingPkg := range installedDB.Artifacts {
		if existingPkg.Name == pkg.Name {
			installedDB.Artifacts[i] = pkg
			installedDB.LastUpdate = time.Now()
			return
		}
	}

	// Add new artifact
	installedDB.Artifacts = append(installedDB.Artifacts, pkg)
	installedDB.LastUpdate = time.Now()
}

// RemoveArtifact removes an installed artifact from the database.
func (installedDB *InstalledDatabase) RemoveArtifact(name string) bool {
	for i, pkg := range installedDB.Artifacts {
		if pkg.Name == name {
			installedDB.Artifacts = append(installedDB.Artifacts[:i], installedDB.Artifacts[i+1:]...)
			installedDB.LastUpdate = time.Now()
			return true
		}
	}
	return false
}

// GetInstalledArtifacts returns all installed packages.
func (installedDB *InstalledDatabase) GetInstalledArtifacts() []*InstalledArtifact {
	return installedDB.Artifacts
}
