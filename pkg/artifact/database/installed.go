// Package database provides a simple JSON-backed store for installed artifacts.
package database

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/model"
)

// InstalledManager defines the interface for managing installed packages.
type InstalledManager interface {
	LoadDatabase(dbPath string) error
	SaveDatabase(dbPath string) error
	FindArtifact(name string) *model.InstalledArtifact
	IsArtifactInstalled(name string) bool
	AddArtifact(pkg *model.InstalledArtifact)
	RemoveArtifact(name string) bool
	GetInstalledArtifacts() []*model.InstalledArtifact
	FilteredArtifacts(nameFilter string) []*model.InstalledArtifact
	SetInstallationReason(name string, reason model.InstallationReason) error
}

// InstalledManagerImpl represents the database of installed packages.
type InstalledManagerImpl struct {
	FormatVersion string                     `json:"format_version"`
	LastUpdate    time.Time                  `json:"last_update"`
	Artifacts     []*model.InstalledArtifact `json:"artifacts"`
	rwMutex       sync.RWMutex
}

const (
	// InitialArtifactCapacity defines the initial slice capacity for installed artifacts.
	InitialArtifactCapacity = 100
)

// NewInstalledDatabase creates a new installed packages database.
func NewInstalledDatabase() *InstalledManagerImpl {
	return &InstalledManagerImpl{
		FormatVersion: "1",
		LastUpdate:    time.Now(),
		Artifacts:     make([]*model.InstalledArtifact, 0, InitialArtifactCapacity),
	}
}

// LoadDatabase loads the installed packages database from file.
func (installedDB *InstalledManagerImpl) LoadDatabase(dbPath string) error {
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
	defer func() { _ = file.Close() }()

	return installedDB.parseInstalledDatabaseFromReader(file)
}

// SetInstallationReason updates the installation reason for an artifact
func (installedDB *InstalledManagerImpl) SetInstallationReason(name string, reason model.InstallationReason) error {
	installedDB.rwMutex.Lock()
	defer installedDB.rwMutex.Unlock()

	for _, artifact := range installedDB.Artifacts {
		if artifact.Name == name {
			artifact.InstallationReason = reason
			installedDB.LastUpdate = time.Now()
			return nil
		}
	}

	return fmt.Errorf("artifact %s not found: %w", name, errors.ErrArtifactNotFound)
}

// SaveDatabase saves the installed packages database to file.
func (installedDB *InstalledManagerImpl) SaveDatabase(dbPath string) (err error) {
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
func (installedDB *InstalledManagerImpl) FindArtifact(name string) *model.InstalledArtifact {
	installedDB.rwMutex.RLock()
	defer installedDB.rwMutex.RUnlock()

	for _, pkg := range installedDB.Artifacts {
		if pkg.Name == name {
			return pkg
		}
	}
	return nil
}

// IsArtifactInstalled checks if a artifact is installed.
func (installedDB *InstalledManagerImpl) IsArtifactInstalled(name string) bool {
	installedDB.rwMutex.RLock()
	defer installedDB.rwMutex.RUnlock()

	for _, pkg := range installedDB.Artifacts {
		if pkg.Name == name {
			return true
		}
	}
	return false
}

// AddArtifact adds an installed artifact to the database.
func (installedDB *InstalledManagerImpl) AddArtifact(pkg *model.InstalledArtifact) {
	installedDB.rwMutex.Lock()
	defer installedDB.rwMutex.Unlock()

	for i, existing := range installedDB.Artifacts {
		if existing.Name == pkg.Name {
			installedDB.Artifacts[i] = pkg
			return
		}
	}

	if pkg.InstalledAt.IsZero() {
		pkg.InstalledAt = time.Now()
	}

	installedDB.Artifacts = append(installedDB.Artifacts, pkg)
	installedDB.LastUpdate = time.Now()
}

// RemoveArtifact removes an installed artifact from the database.
func (installedDB *InstalledManagerImpl) RemoveArtifact(name string) bool {
	installedDB.rwMutex.Lock()
	defer installedDB.rwMutex.Unlock()

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
func (installedDB *InstalledManagerImpl) GetInstalledArtifacts() []*model.InstalledArtifact {
	installedDB.rwMutex.RLock()
	defer installedDB.rwMutex.RUnlock()

	// Return a copy of the slice to prevent data races
	artifacts := make([]*model.InstalledArtifact, len(installedDB.Artifacts))
	copy(artifacts, installedDB.Artifacts)
	return artifacts
}

// FilteredArtifacts returns installed packages filtered by name (partial match, case-insensitive).
func (installedDB *InstalledManagerImpl) FilteredArtifacts(nameFilter string) []*model.InstalledArtifact {
	installedDB.rwMutex.RLock()
	defer installedDB.rwMutex.RUnlock()

	if nameFilter == "" {
		// Return all artifacts if no filter provided
		return installedDB.GetInstalledArtifacts()
	}

	// Filter artifacts by name
	var filtered []*model.InstalledArtifact
	for _, artifact := range installedDB.Artifacts {
		if strings.Contains(strings.ToLower(artifact.Name), strings.ToLower(nameFilter)) {
			filtered = append(filtered, artifact)
		}
	}

	return filtered
}

// parseInstalledDatabaseFromReader parses the database from an io.Reader.
func (installedDB *InstalledManagerImpl) parseInstalledDatabaseFromReader(reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read database: %w", err)
	}

	if err := json.Unmarshal(data, installedDB); err != nil {
		return fmt.Errorf("failed to parse database: %w", err)
	}

	return nil
}
