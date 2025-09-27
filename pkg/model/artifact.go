package model

import (
	"net/url"

	"github.com/cperrin88/gotya/pkg/platform"
	"github.com/hashicorp/go-version"
)

// Dependency represents a dependency with a name and an optional version constraint.
type Dependency struct {
	Name              string `json:"name"`
	VersionConstraint string `json:"version,omitempty"`
}

// InstallationReason tracks why an artifact was installed
type InstallationReason string

const (
	// InstallationReasonManual indicates the artifact was installed explicitly by the user
	InstallationReasonManual InstallationReason = "manual"
	// InstallationReasonAutomatic indicates the artifact was installed automatically as a dependency
	InstallationReasonAutomatic InstallationReason = "automatic"
)

// IndexArtifactDescriptor represents the metadata and properties of an indexed artifact in a repository or package.
type IndexArtifactDescriptor struct {
	Name         string       `json:"name"`
	Version      string       `json:"version"`
	Description  string       `json:"description"`
	URL          string       `json:"url"`
	Checksum     string       `json:"checksum"`
	Size         int64        `json:"size"`
	OS           string       `json:"os,omitempty"`
	Arch         string       `json:"arch,omitempty"`
	Dependencies []Dependency `json:"dependencies,omitempty"`
}

func (a *IndexArtifactDescriptor) MatchOs(os string) bool {
	return a.OS == "" || a.OS == os || a.OS == platform.AnyOS
}

func (a *IndexArtifactDescriptor) MatchArch(arch string) bool {
	return a.Arch == "" || a.Arch == arch || a.Arch == platform.AnyArch
}

func (a *IndexArtifactDescriptor) MatchVersion(versionConstraint string) bool {
	constraint, err := version.NewConstraint(versionConstraint)
	if err != nil {
		return false
	}
	v := a.GetVersion()
	if v == nil {
		return false
	}
	return constraint.Check(v)
}

func (a *IndexArtifactDescriptor) GetVersion() *version.Version {
	v, err := version.NewVersion(a.Version)
	if err != nil {
		return nil
	}
	return v
}

func (a *IndexArtifactDescriptor) GetOS() string {
	if a.OS == "" {
		return platform.AnyOS
	}
	return a.OS
}

func (a *IndexArtifactDescriptor) GetArch() string {
	if a.Arch == "" {
		return platform.AnyArch
	}
	return a.Arch
}

func (a *IndexArtifactDescriptor) GetURL() *url.URL {
	parse, err := url.Parse(a.URL)
	if err != nil {
		return nil
	}
	return parse
}
