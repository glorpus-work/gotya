package pkg

import (
	"encoding/json"
	"io"

	"github.com/cperrin88/gotya/pkg/errors"
)

// ParseMetadata parses pkg metadata from JSON data.
func ParseMetadata(data []byte) (*PackageMetadata, error) {
	var metadata PackageMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, errors.Wrapf(err, "failed to parse metadata")
	}

	// Validate required fields
	if err := metadata.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid metadata")
	}

	return &metadata, nil
}

// ParseMetadataFromReader parses metadata from an io.Reader.
func ParseMetadataFromReader(reader io.Reader) (*PackageMetadata, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read metadata")
	}
	return ParseMetadata(data)
}

// ToJSON converts the metadata to JSON bytes.
func (m *PackageMetadata) ToJSON() ([]byte, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal pkg metadata to JSON")
	}
	return data, nil
}

// Validate validates the pkg metadata.
func (m *PackageMetadata) Validate() error {
	if m.Name == "" {
		return errors.Wrap(errors.ErrValidationFailed, "pkg name is required")
	}
	if m.Version == "" {
		return errors.Wrap(errors.ErrValidationFailed, "pkg version is required")
	}
	return nil
}
