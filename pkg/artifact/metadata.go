package artifact

import (
	"encoding/json"
	"io"
	"os"

	"github.com/cperrin88/gotya/pkg/platform"
	"github.com/hashicorp/go-version"
)

// in the artifact meta directory.
type Metadata struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	OS           string            `json:"os"`
	Arch         string            `json:"arch"`
	Maintainer   string            `json:"maintainer,omitempty"`
	Description  string            `json:"description"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Hashes       map[string]string `json:"files,omitempty"`
	Hooks        map[string]string `json:"hooks,omitempty"`
}

func (m *Metadata) GetVersion() *version.Version {
	v, err := version.NewVersion(m.Version)
	if err != nil {
		return nil
	}
	return v
}

func (m *Metadata) GetOS() string {
	if m.OS == "" {
		return platform.AnyOS
	}
	return m.OS
}

func (m *Metadata) GetArch() string {
	if m.Arch == "" {
		return platform.AnyArch
	}
	return m.Arch
}

func ParseMetadataFromPath(path string) (*Metadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return ParseMetadataFromStream(file)
}

func ParseMetadataFromStream(stream io.Reader) (*Metadata, error) {
	var metadata Metadata
	decoder := json.NewDecoder(stream)
	if err := decoder.Decode(&metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}
