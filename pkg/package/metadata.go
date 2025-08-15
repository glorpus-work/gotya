package pkg

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// PackageMetadata represents the metadata stored in a package tar.xz file
type PackageMetadata struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description"`
	Author       string            `json:"author,omitempty"`
	Homepage     string            `json:"homepage,omitempty"`
	License      string            `json:"license,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Conflicts    []string          `json:"conflicts,omitempty"`
	Provides     []string          `json:"provides,omitempty"`
	Architecture string            `json:"architecture,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}

// ParseMetadata parses package metadata from JSON data
func ParseMetadata(data []byte) (*PackageMetadata, error) {
	var metadata PackageMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Validate required fields
	if metadata.Name == "" {
		return nil, fmt.Errorf("package name is required")
	}
	if metadata.Version == "" {
		return nil, fmt.Errorf("package version is required")
	}

	return &metadata, nil
}

// ParseMetadataFromReader parses metadata from an io.Reader
func ParseMetadataFromReader(reader io.Reader) (*PackageMetadata, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}
	return ParseMetadata(data)
}

// ToJSON converts the metadata to JSON bytes
func (m *PackageMetadata) ToJSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// Validate validates the package metadata
func (m *PackageMetadata) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("package name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("package version is required")
	}
	return nil
}
