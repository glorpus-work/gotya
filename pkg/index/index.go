package index

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/model"
)

const (
	// InitialArtifactCapacity is the initial capacity for the packages slice.
	InitialArtifactCapacity = 100
)

// NewIndex creates a new index with the current timestamp.
func NewIndex(formatVersion string) *Index {
	return &Index{
		FormatVersion: formatVersion,
		LastUpdate:    time.Now(),
		Artifacts:     make([]*model.IndexArtifactDescriptor, 0, InitialArtifactCapacity),
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

// GetArtifacts returns all packages.
func (idx *Index) GetArtifacts() []*model.IndexArtifactDescriptor {
	return idx.Artifacts
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
	if err != nil {
		return nil, errors.Wrapf(err, "Cannot open index file %s for parsing", filePath)
	}
	defer file.Close()
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

func (idx *Index) FindArtifacts(name string) []*model.IndexArtifactDescriptor {
	packages := make([]*model.IndexArtifactDescriptor, 0, 5)
	for _, pkg := range idx.Artifacts {
		if pkg.Name == name {
			packages = append(packages, pkg)
		}
	}

	return packages
}
