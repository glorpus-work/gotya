package pkg

import (
	"encoding/json"
	"fmt"
	"io"
)

// ParseMetadata parses package metadata from JSON data.
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

// ParseMetadataFromReader parses metadata from an io.Reader.
func ParseMetadataFromReader(reader io.Reader) (*PackageMetadata, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}
	return ParseMetadata(data)
}

// ToJSON converts the metadata to JSON bytes.
func (m *PackageMetadata) ToJSON() ([]byte, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal package metadata to JSON: %w", err)
	}
	return data, nil
}

// Validate validates the package metadata.
func (m *PackageMetadata) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("package name is required")
	}
	if m.Version == "" {
		return fmt.Errorf("package version is required")
	}
	return nil
}
