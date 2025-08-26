package index

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cperrin88/gotya/pkg/errors"
)

const (
	// InitialPackageCapacity is the initial capacity for the packages slice.
	InitialPackageCapacity = 100
)

// NewIndex creates a new index with the current timestamp.
func NewIndex(formatVersion string) *Index {
	return &Index{
		FormatVersion: formatVersion,
		LastUpdate:    time.Now(),
		Packages:      make([]*Package, 0, InitialPackageCapacity),
	}
}

// GetFormatVersion returns the format version.
func (idx *Index) GetFormatVersion() string {
	return idx.FormatVersion
}

// GetLastUpdate returns the last update timestamp as string.
func (idx *Index) GetLastUpdate() string {
	return idx.LastUpdate.Format(time.RFC3339)
}

// GetPackages returns all packages.
func (idx *Index) GetPackages() []*Package {
	return idx.Packages
}

// ParseIndex parses an index from JSON data.
func ParseIndex(data []byte) (*Index, error) {
	var index Index
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, errors.Wrap(err, "failed to parse index")
	}

	// Validate format version
	if index.FormatVersion == "" {
		return nil, fmt.Errorf("missing format version in index")
	}

	return &index, nil
}

// ParseIndexFromReader parses an index from an io.Reader.
func ParseIndexFromReader(reader io.Reader) (*Index, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read index data")
	}

	return ParseIndex(data)
}

func ParseIndexFromFile(filePath string) (*Index, error) {
	file, err := os.Open(filePath)
	defer file.Close()
	if err != nil {
		return nil, errors.Wrapf(err, "Cannot open index file %s for parsing", filePath)
	}
	return ParseIndexFromReader(file)
}

// ToJSON converts the index to JSON bytes.
func (idx *Index) ToJSON() ([]byte, error) {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal index to JSON")
	}
	return data, nil
}

func (idx *Index) FindPackages(name string) []*Package {
	packages := make([]*Package, 0, 5)
	for _, pkg := range idx.Packages {
		if pkg.Name == name {
			packages = append(packages, pkg)
		}
	}

	return packages
}

// AddPackage adds a pkg to the index.
func (idx *Index) AddPackage(pkg *Package) {
	// Remove existing pkg with same name if it exists
	for i := range idx.Packages {
		if idx.Packages[i].Name == pkg.Name {
			idx.Packages[i] = pkg
			idx.LastUpdate = time.Now()
			return
		}
	}

	// Add new pkg
	idx.Packages = append(idx.Packages, pkg)
	idx.LastUpdate = time.Now()
}

// RemovePackage removes a pkg from the index.
func (idx *Index) RemovePackage(name string) bool {
	for i := range idx.Packages {
		if idx.Packages[i].Name == name {
			idx.Packages = append(idx.Packages[:i], idx.Packages[i+1:]...)
			idx.LastUpdate = time.Now()
			return true
		}
	}
	return false
}
