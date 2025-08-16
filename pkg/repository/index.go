package repository

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// IndexImpl implements the Index interface
type IndexImpl struct {
	FormatVersion string    `json:"format_version"`
	LastUpdate    time.Time `json:"last_update"`
	Packages      []Package `json:"packages"`
}

// NewIndex creates a new index with the current timestamp
func NewIndex(formatVersion string) *IndexImpl {
	return &IndexImpl{
		FormatVersion: formatVersion,
		LastUpdate:    time.Now(),
		Packages:      make([]Package, 0),
	}
}

// GetFormatVersion returns the format version
func (idx *IndexImpl) GetFormatVersion() string {
	return idx.FormatVersion
}

// GetLastUpdate returns the last update timestamp as string
func (idx *IndexImpl) GetLastUpdate() string {
	return idx.LastUpdate.Format(time.RFC3339)
}

// GetPackages returns all packages
func (idx *IndexImpl) GetPackages() []Package {
	return idx.Packages
}

// ParseIndex parses an index from JSON data
func ParseIndex(data []byte) (*IndexImpl, error) {
	var index IndexImpl
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse index: %w", err)
	}

	// Validate format version
	if index.FormatVersion == "" {
		return nil, fmt.Errorf("missing format version in index")
	}

	return &index, nil
}

// ParseIndexFromReader parses an index from an io.Reader
func ParseIndexFromReader(reader io.Reader) (*IndexImpl, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read index data: %w", err)
	}

	return ParseIndex(data)
}

// ToJSON converts the index to JSON bytes
func (idx *IndexImpl) ToJSON() ([]byte, error) {
	return json.MarshalIndent(idx, "", "  ")
}

// FindPackage finds a package by name
func (idx *IndexImpl) FindPackage(name string) *Package {
	for i := range idx.Packages {
		if idx.Packages[i].Name == name {
			return &idx.Packages[i]
		}
	}
	return nil
}

// AddPackage adds a package to the index
func (idx *IndexImpl) AddPackage(pkg Package) {
	// Remove existing package with same name if it exists
	for i, existingPkg := range idx.Packages {
		if existingPkg.Name == pkg.Name {
			idx.Packages[i] = pkg
			idx.LastUpdate = time.Now()
			return
		}
	}

	// Add new package
	idx.Packages = append(idx.Packages, pkg)
	idx.LastUpdate = time.Now()
}

// RemovePackage removes a package from the index
func (idx *IndexImpl) RemovePackage(name string) bool {
	for i, pkg := range idx.Packages {
		if pkg.Name == name {
			idx.Packages = append(idx.Packages[:i], idx.Packages[i+1:]...)
			idx.LastUpdate = time.Now()
			return true
		}
	}
	return false
}
