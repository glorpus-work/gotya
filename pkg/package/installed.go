package pkg

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/cperrin88/gotya/pkg/fsutil"
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
		return nil, fmt.Errorf("database path must be absolute: %s", dbPath)
	}

	// Check if file exists with cleaned path
	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		// Database doesn't exist, return new empty database
		return NewInstalledDatabase(), nil
	}

	// Open the file with the cleaned path
	file, err := os.Open(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
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
		return nil, fmt.Errorf("failed to read database: %w", err)
	}

	// First unmarshal into a temporary struct
	var tempDB tempInstalledDatabase
	if err := json.Unmarshal(data, &tempDB); err != nil {
		return nil, fmt.Errorf("failed to parse database: %w", err)
	}

	// Convert to our actual database structure with pointers
	db := &InstalledDatabase{
		FormatVersion: tempDB.FormatVersion,
		LastUpdate:    tempDB.LastUpdate,
		Packages:      make([]*InstalledPackage, 0, len(tempDB.Packages)),
	}

	// Convert each package to a pointer
	for i := range tempDB.Packages {
		pkg := tempDB.Packages[i] // Create a copy in the loop
		db.Packages = append(db.Packages, &pkg)
	}

	return db, nil
}

// Save saves the installed packages database to file.
func (db *InstalledDatabase) Save(dbPath string) (err error) {
	// Clean and validate the database path
	cleanPath := filepath.Clean(dbPath)
	if !filepath.IsAbs(cleanPath) {
		return fmt.Errorf("database path must be absolute: %s", dbPath)
	}

	// Create a temporary struct for JSON marshaling
	tempDB := struct {
		FormatVersion string             `json:"format_version"`
		LastUpdate    time.Time          `json:"last_update"`
		Packages      []InstalledPackage `json:"packages"`
	}{
		FormatVersion: db.FormatVersion,
		LastUpdate:    db.LastUpdate,
		Packages:      make([]InstalledPackage, 0, len(db.Packages)),
	}

	// Convert pointer slice to value slice
	for _, pkg := range db.Packages {
		tempDB.Packages = append(tempDB.Packages, *pkg)
	}

	// Ensure directory exists with secure permissions
	if err := fsutil.EnsureDir(filepath.Dir(cleanPath)); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Create a temporary file in the same directory
	tempPath := cleanPath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp database file: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tempPath)
		}
	}()

	// Write JSON with indentation for better readability
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(tempDB); err != nil {
		return fmt.Errorf("failed to encode database: %w", err)
	}

	// Ensure all data is written to disk
	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync database file: %w", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close database file: %w", err)
	}

	// Rename the temporary file to the final location
	if err := os.Rename(tempPath, cleanPath); err != nil {
		return fmt.Errorf("failed to replace database file: %w", err)
	}

	return nil
}

// FindPackage finds an installed package by name.
func (db *InstalledDatabase) FindPackage(name string) *InstalledPackage {
	for _, pkg := range db.Packages {
		if pkg.Name == name {
			return pkg
		}
	}
	return nil
}

// IsPackageInstalled checks if a package is installed.
func (db *InstalledDatabase) IsPackageInstalled(name string) bool {
	return db.FindPackage(name) != nil
}

// AddPackage adds an installed package to the database.
func (db *InstalledDatabase) AddPackage(pkg *InstalledPackage) {
	// Remove existing package with same name if it exists
	for i, existingPkg := range db.Packages {
		if existingPkg.Name == pkg.Name {
			db.Packages[i] = pkg
			db.LastUpdate = time.Now()
			return
		}
	}

	// Add new package
	db.Packages = append(db.Packages, pkg)
	db.LastUpdate = time.Now()
}

// RemovePackage removes an installed package from the database.
func (db *InstalledDatabase) RemovePackage(name string) bool {
	for i, pkg := range db.Packages {
		if pkg.Name == name {
			db.Packages = append(db.Packages[:i], db.Packages[i+1:]...)
			db.LastUpdate = time.Now()
			return true
		}
	}
	return false
}

// GetInstalledPackages returns all installed packages.
func (db *InstalledDatabase) GetInstalledPackages() []*InstalledPackage {
	return db.Packages
}
