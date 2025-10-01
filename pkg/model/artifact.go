// Package model provides data structures and types for representing artifacts,
// dependencies, and related metadata in the gotya package manager.
package model

// Package model provides data structures and types for representing artifacts,
// dependencies, and related metadata in the gotya package manager.

import (
	"net/url"

	"github.com/glorpus-work/gotya/pkg/platform"
	"github.com/hashicorp/go-version"
)

// Dependency represents a dependency with a name and an optional version constraint.
type Dependency struct {
	Name              string `json:"name"`
	VersionConstraint string `json:"version_constraint,omitempty"`
}

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

// InstallationReason tracks why an artifact was installed
type InstallationReason string

const (
	// InstallationReasonManual indicates the artifact was installed explicitly by the user
	InstallationReasonManual InstallationReason = "manual"
	// InstallationReasonAutomatic indicates the artifact was installed automatically as a dependency
	InstallationReasonAutomatic InstallationReason = "automatic"
)

// MatchOs checks if this artifact matches the given operating system.
func (a *IndexArtifactDescriptor) MatchOs(os string) bool {
	return a.OS == "" || a.OS == os || a.OS == platform.AnyOS
}

// MatchArch checks if this artifact matches the given architecture.
func (a *IndexArtifactDescriptor) MatchArch(arch string) bool {
	return a.Arch == "" || a.Arch == arch || a.Arch == platform.AnyArch
}

// MatchVersion checks if this artifact's version satisfies the given version constraint.
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

// GetVersion returns the parsed version of this artifact.
func (a *IndexArtifactDescriptor) GetVersion() *version.Version {
	v, err := version.NewVersion(a.Version)
	if err != nil {
		return nil
	}
	return v
}

// GetOS returns the operating system this artifact targets, or AnyOS if not specified.
func (a *IndexArtifactDescriptor) GetOS() string {
	if a.OS == "" {
		return platform.AnyOS
	}
	return a.OS
}

// GetArch returns the architecture this artifact targets, or AnyArch if not specified.
func (a *IndexArtifactDescriptor) GetArch() string {
	if a.Arch == "" {
		return platform.AnyArch
	}
	return a.Arch
}

// GetURL returns the parsed URL of this artifact.
func (a *IndexArtifactDescriptor) GetURL() *url.URL {
	parse, err := url.Parse(a.URL)
	if err != nil {
		return nil
	}
	return parse
}
