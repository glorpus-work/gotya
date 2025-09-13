package artifact

import (
	"encoding/json"
	"io"

	"github.com/cperrin88/gotya/pkg/errors"
)

// ParseMetadata parses artifact metadata from JSON data.
func ParseMetadata(data []byte) (*ArtifactMetadata, error) {
	var metadata ArtifactMetadata
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
func ParseMetadataFromReader(reader io.Reader) (*ArtifactMetadata, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read metadata")
	}
	return ParseMetadata(data)
}

// ToJSON converts the metadata to JSON bytes.
func (m *ArtifactMetadata) ToJSON() ([]byte, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal artifact metadata to JSON")
	}
	return data, nil
}

// Validate validates the artifact metadata.
func (m *ArtifactMetadata) Validate() error {
	if m.Name == "" {
		return errors.Wrap(errors.ErrValidationFailed, "artifact name is required")
	}
	if m.Version == "" {
		return errors.Wrap(errors.ErrValidationFailed, "artifact version is required")
	}
	return nil
}
