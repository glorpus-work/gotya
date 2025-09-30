package artifact

import (
	"encoding/json"
	"io"
	"os"

	"github.com/cperrin88/gotya/pkg/model"
	"github.com/cperrin88/gotya/pkg/platform"
	"github.com/hashicorp/go-version"
)

// Metadata represents the metadata of an artifact, including name, version, OS, architecture,
// maintainer, description, dependencies, hooks, and file hashes.
type Metadata struct {
	Name         string             `json:"name"`
	Version      string             `json:"version"`
	OS           string             `json:"os"`
	Arch         string             `json:"arch"`
	Maintainer   string             `json:"maintainer,omitempty"`
	Description  string             `json:"description"`
	Dependencies []model.Dependency `json:"dependencies,omitempty"`
	Hashes       map[string]string  `json:"files,omitempty"`
	Hooks        map[string]string  `json:"hooks,omitempty"`
}

// GetVersion returns the parsed version of this artifact.
func (m *Metadata) GetVersion() *version.Version {
	v, err := version.NewVersion(m.Version)
	if err != nil {
		return nil
	}
	return v
}

// GetOS returns the operating system this artifact targets, or AnyOS if not specified.
func (m *Metadata) GetOS() string {
	if m.OS == "" {
		return platform.AnyOS
	}
	return m.OS
}

// GetArch returns the architecture this artifact targets, or AnyArch if not specified.
func (m *Metadata) GetArch() string {
	if m.Arch == "" {
		return platform.AnyArch
	}
	return m.Arch
}

// ParseMetadataFromPath parses metadata from a file path.
func ParseMetadataFromPath(path string) (*Metadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	return ParseMetadataFromStream(file)
}

// ParseMetadataFromStream parses metadata from an io.Reader stream.
func ParseMetadataFromStream(stream io.Reader) (*Metadata, error) {
	var metadata Metadata
	decoder := json.NewDecoder(stream)
	if err := decoder.Decode(&metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}
