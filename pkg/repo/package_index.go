package repo

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Index represents the repository index structure
type Index struct {
	FormatVersion string    `json:"format_version"`
	LastUpdate    time.Time `json:"last_update"`
	Packages      []Package `json:"packages"`
}

// Package represents a package in the repository
type Package struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description"`
	URL          string            `json:"url"`
	Checksum     string            `json:"checksum"`
	Size         int64             `json:"size"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ParseIndex parses an index from JSON data
func ParseIndex(data []byte) (*Index, error) {
	var index Index
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
func ParseIndexFromReader(reader io.Reader) (*Index, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read index data: %w", err)
	}

	return ParseIndex(data)
}

// ToJSON converts the index to JSON bytes
func (idx *Index) ToJSON() ([]byte, error) {
	return json.MarshalIndent(idx, "", "  ")
}

// FindPackage finds a package by name
func (idx *Index) FindPackage(name string) *Package {
	for i := range idx.Packages {
		if idx.Packages[i].Name == name {
			return &idx.Packages[i]
		}
	}
	return nil
}

// AddPackage adds a package to the index
func (idx *Index) AddPackage(pkg Package) {
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
func (idx *Index) RemovePackage(name string) bool {
	for i, pkg := range idx.Packages {
		if pkg.Name == name {
			idx.Packages = append(idx.Packages[:i], idx.Packages[i+1:]...)
			idx.LastUpdate = time.Now()
			return true
		}
	}
	return false
}

// NewIndex creates a new index with the current timestamp
func NewIndex(formatVersion string) *Index {
	return &Index{
		FormatVersion: formatVersion,
		LastUpdate:    time.Now(),
		Packages:      make([]Package, 0),
	}
}
