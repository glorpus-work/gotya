package pkg

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// InstalledDatabase represents the database of installed packages
type InstalledDatabase struct {
	FormatVersion string             `json:"format_version"`
	LastUpdate    time.Time          `json:"last_update"`
	Packages      []InstalledPackage `json:"packages"`
}

// NewInstalledDatabase creates a new installed packages database
func NewInstalledDatabase() *InstalledDatabase {
	return &InstalledDatabase{
		FormatVersion: "1.0",
		LastUpdate:    time.Now(),
		Packages:      make([]InstalledPackage, 0),
	}
}

// LoadInstalledDatabase loads the installed packages database from file
func LoadInstalledDatabase(dbPath string) (*InstalledDatabase, error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		// Database doesn't exist, return new empty database
		return NewInstalledDatabase(), nil
	}

	file, err := os.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer file.Close()

	return ParseInstalledDatabaseFromReader(file)
}

// ParseInstalledDatabaseFromReader parses the database from an io.Reader
func ParseInstalledDatabaseFromReader(reader io.Reader) (*InstalledDatabase, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read database: %w", err)
	}

	var db InstalledDatabase
	if err := json.Unmarshal(data, &db); err != nil {
		return nil, fmt.Errorf("failed to parse database: %w", err)
	}

	return &db, nil
}

// Save saves the installed packages database to file
func (db *InstalledDatabase) Save(dbPath string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Create temporary file first
	tempPath := dbPath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp database file: %w", err)
	}

	// Update timestamp and write database
	db.LastUpdate = time.Now()
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		file.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to serialize database: %w", err)
	}

	if _, err := file.Write(data); err != nil {
		file.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to write database: %w", err)
	}

	file.Close()

	// Atomically replace the database file
	if err := os.Rename(tempPath, dbPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to replace database file: %w", err)
	}

	return nil
}

// FindPackage finds an installed package by name
func (db *InstalledDatabase) FindPackage(name string) *InstalledPackage {
	for i := range db.Packages {
		if db.Packages[i].Name == name {
			return &db.Packages[i]
		}
	}
	return nil
}

// IsPackageInstalled checks if a package is installed
func (db *InstalledDatabase) IsPackageInstalled(name string) bool {
	return db.FindPackage(name) != nil
}

// AddPackage adds an installed package to the database
func (db *InstalledDatabase) AddPackage(pkg InstalledPackage) {
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

// RemovePackage removes an installed package from the database
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

// GetInstalledPackages returns all installed packages
func (db *InstalledDatabase) GetInstalledPackages() []InstalledPackage {
	return db.Packages
}
