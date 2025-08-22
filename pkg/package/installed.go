package pkg

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/cperrin88/gotya/pkg/errors"
)

// InstalledDatabase represents the database of installed packages.
type InstalledDatabase struct {
	FormatVersion string              `json:"format_version"`
	LastUpdate    time.Time           `json:"last_update"`
	Packages      []*InstalledPackage `json:"packages"`
}

const (
	// InitialPackageCapacity is the initial capacity for the installed packages slice.
	InitialPackageCapacity = 100
)

// NewInstalledDatabase creates a new installed packages database.
func NewInstalledDatabase() *InstalledDatabase {
	return &InstalledDatabase{
		FormatVersion: "1.0",
		LastUpdate:    time.Now(),
		Packages:      make([]*InstalledPackage, 0, InitialPackageCapacity),
	}
}

// LoadInstalledDatabase loads the installed packages database from file.
func LoadInstalledDatabase(dbPath string) (*InstalledDatabase, error) {
	// Clean and validate the database path
	cleanPath := filepath.Clean(dbPath)
	if !filepath.IsAbs(cleanPath) {
		return nil, errors.Wrapf(errors.ErrInvalidPath, "database path must be absolute: %s", dbPath)
	}

	// Check if file exists with cleaned path
	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		// Database doesn't exist, return new empty database
		return NewInstalledDatabase(), nil
	}

	// Open the file with the cleaned path
	file, err := os.Open(cleanPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open database")
	}
	defer file.Close()

	return ParseInstalledDatabaseFromReader(file)
}

// tempInstalledDatabase is used for JSON unmarshaling.
type tempInstalledDatabase struct {
	FormatVersion string             `json:"format_version"`
	LastUpdate    time.Time          `json:"last_update"`
	Packages      []InstalledPackage `json:"packages"`
}

// ParseInstalledDatabaseFromReader parses the database from an io.Reader.
func ParseInstalledDatabaseFromReader(reader io.Reader) (*InstalledDatabase, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read database")
	}

	// First unmarshal into a temporary struct
	var tempDB tempInstalledDatabase
	if err := json.Unmarshal(data, &tempDB); err != nil {
		return nil, errors.Wrapf(err, "failed to parse database")
	}

	// Convert to our actual database structure with pointers
	installedDB := &InstalledDatabase{
		FormatVersion: tempDB.FormatVersion,
		LastUpdate:    tempDB.LastUpdate,
		Packages:      make([]*InstalledPackage, 0, len(tempDB.Packages)),
	}

	// Convert each package to a pointer
	for i := range tempDB.Packages {
		pkg := tempDB.Packages[i] // Create a copy in the loop
		installedDB.Packages = append(installedDB.Packages, &pkg)
	}

	return installedDB, nil
}

// Save saves the installed packages database to file.
func (installedDB *InstalledDatabase) Save(dbPath string) (err error) {
	// Clean and validate the database path
	cleanPath := filepath.Clean(dbPath)
	if !filepath.IsAbs(cleanPath) {
		return errors.Wrapf(errors.ErrInvalidPath, "database path must be absolute: %s", dbPath)
	}

	// Get the directory of the database file
	dbDir := filepath.Dir(cleanPath)

	// Create a temporary file for atomic write
	tmpFile, err := os.CreateTemp(dbDir, "gotya-db-*.tmp")
	if err != nil {
		return errors.Wrapf(err, "failed to create temporary file in %s", dbDir)
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
		return errors.Wrap(err, "failed to marshal database to JSON")
	}

	// Write to temporary file
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return errors.Wrap(err, "failed to write to temporary file")
	}

	// Ensure the data is written to disk
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return errors.Wrap(err, "failed to sync temporary file to disk")
	}

	// Close the temporary file
	if err := tmpFile.Close(); err != nil {
		return errors.Wrap(err, "failed to close temporary file")
	}

	// Atomically rename the temporary file to the target file
	if err := os.Rename(tmpPath, cleanPath); err != nil {
		return errors.Wrapf(err, "failed to rename temporary file to %s", cleanPath)
	}

	return nil
}

// FindPackage finds an installed package by name.
func (installedDB *InstalledDatabase) FindPackage(name string) *InstalledPackage {
	for _, pkg := range installedDB.Packages {
		if pkg.Name == name {
			return pkg
		}
	}
	return nil
}

// IsPackageInstalled checks if a package is installed.
func (installedDB *InstalledDatabase) IsPackageInstalled(name string) bool {
	return installedDB.FindPackage(name) != nil
}

// AddPackage adds an installed package to the database.
func (installedDB *InstalledDatabase) AddPackage(pkg *InstalledPackage) {
	// Remove existing package with same name if it exists
	for i, existingPkg := range installedDB.Packages {
		if existingPkg.Name == pkg.Name {
			installedDB.Packages[i] = pkg
			installedDB.LastUpdate = time.Now()
			return
		}
	}

	// Add new package
	installedDB.Packages = append(installedDB.Packages, pkg)
	installedDB.LastUpdate = time.Now()
}

// RemovePackage removes an installed package from the database.
func (installedDB *InstalledDatabase) RemovePackage(name string) bool {
	for i, pkg := range installedDB.Packages {
		if pkg.Name == name {
			installedDB.Packages = append(installedDB.Packages[:i], installedDB.Packages[i+1:]...)
			installedDB.LastUpdate = time.Now()
			return true
		}
	}
	return false
}

// GetInstalledPackages returns all installed packages.
func (installedDB *InstalledDatabase) GetInstalledPackages() []*InstalledPackage {
	return installedDB.Packages
}
