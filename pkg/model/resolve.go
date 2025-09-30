package model

import (
	"net/url"
	"time"
)

// ResolveRequest describes what the user asked to resolve.
type ResolveRequest struct {
	Name              string
	VersionConstraint string // semver constraint (e.g., ">= 0.0.0" for latest)
	OS                string // target os
	Arch              string // target arch
	OldVersion        string // current installed version (optional)
	KeepVersion       bool   // prefer to keep OldVersion if possible
}

// ResolvedArtifact represents a concrete installation action.
type ResolvedArtifact struct {
	Name      string
	Version   string
	OS        string
	Arch      string
	SourceURL *url.URL
	Checksum  string
	Action    ResolvedAction
	Reason    string
}

// ResolvedAction represents the type of action to take for an artifact.
type ResolvedAction string

// ResolvedArtifacts is an ordered list of steps, topologically sorted if dependencies are present.
type ResolvedArtifacts struct {
	Artifacts []ResolvedArtifact
}

// InstalledFile represents a file installed by an artifact with its hash.
type InstalledFile struct {
	Path string // Relative path from its base directory
	Hash string // SHA256 hash of the file contents
}

// ArtifactStatus represents the status of an installed artifact.
type ArtifactStatus string

// InstalledArtifact represents an installed artifact with its files and installation metadata.
type InstalledArtifact struct {
	Name                string
	Version             string
	Description         string
	OS                  string // target operating system
	Arch                string // target architecture
	InstalledAt         time.Time
	InstalledFrom       string // URL or index where it was installed from
	ArtifactMetaDir     string // Base directory for meta files
	ArtifactDataDir     string // Base directory for data files
	MetaFiles           []InstalledFile
	DataFiles           []InstalledFile
	ReverseDependencies []string       // List of artifact names that depend on this artifact
	Status              ArtifactStatus // Status of the artifact
	Checksum            string
	InstallationReason  InstallationReason // Why this artifact was installed
}

const (
	// ResolvedActionInstall indicates the artifact should be newly installed.
	ResolvedActionInstall ResolvedAction = "install"
	// ResolvedActionUpdate indicates an existing artifact should be updated.
	ResolvedActionUpdate ResolvedAction = "update"
	// ResolvedActionSkip indicates the artifact is already at the correct version.
	ResolvedActionSkip ResolvedAction = "skip"

	// StatusInstalled indicates the artifact is fully installed.
	StatusInstalled ArtifactStatus = "installed"
	// StatusMissing indicates the artifact is not installed but referenced as a dependency.
	StatusMissing ArtifactStatus = "missing"
)

// GetID returns the unique identifier for this artifact (name@version)
func (ra *ResolvedArtifact) GetID() string {
	return ra.Name + "@" + ra.Version
}
