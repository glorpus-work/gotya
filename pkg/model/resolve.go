package model

import (
	"net/url"
	"time"
)

// ResolveRequest describes what the user asked to resolve.
type ResolveRequest struct {
	Name               string               `json:"name"`
	Version            string               `json:"version"` // semver constraint (e.g., ">= 0.0.0" for latest)
	OS                 string               `json:"os"`      // target os
	Arch               string               `json:"arch"`    // target arch
	InstalledArtifacts []*InstalledArtifact `json:"-"`       // currently installed artifacts for compatibility checking
}

// ResolvedArtifact represents a concrete installation action.
type ResolvedArtifact struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Version   string         `json:"version"`
	OS        string         `json:"os"`
	Arch      string         `json:"arch"`
	SourceURL *url.URL       `json:"source_url"`
	Checksum  string         `json:"checksum"`
	Action    ResolvedAction `json:"action"` // install, update, or skip
	Reason    string         `json:"reason"` // explanation for the chosen action
}

// ResolvedAction represents the type of action to take for an artifact.
type ResolvedAction string

const (
	// ResolvedActionInstall indicates the artifact should be newly installed.
	ResolvedActionInstall ResolvedAction = "install"
	// ResolvedActionUpdate indicates an existing artifact should be updated.
	ResolvedActionUpdate ResolvedAction = "update"
	// ResolvedActionSkip indicates the artifact is already at the correct version.
	ResolvedActionSkip ResolvedAction = "skip"
)

// ResolvedArtifacts is an ordered list of steps, topologically sorted if dependencies are present.
type ResolvedArtifacts struct {
	Artifacts []ResolvedArtifact `json:"artifacts"`
}

// InstalledFile represents a file installed by an artifact with its hash.
type InstalledFile struct {
	Path string `json:"path"` // Relative path from its base directory
	Hash string `json:"hash"` // SHA256 hash of the file contents
}

// ArtifactStatus represents the status of an installed artifact.
type ArtifactStatus string

const (
	// StatusInstalled indicates the artifact is fully installed.
	StatusInstalled ArtifactStatus = "installed"
	// StatusMissing indicates the artifact is not installed but referenced as a dependency.
	StatusMissing ArtifactStatus = "missing"
)

// InstalledArtifact represents an installed artifact with its files.
type InstalledArtifact struct {
	Name                string             `json:"name"`
	Version             string             `json:"version"`
	Description         string             `json:"description"`
	InstalledAt         time.Time          `json:"installed_at"`
	InstalledFrom       string             `json:"installed_from"`       // URL or index where it was installed from
	ArtifactMetaDir     string             `json:"artifact_meta_dir"`    // Base directory for meta files
	ArtifactDataDir     string             `json:"artifact_data_dir"`    // Base directory for data files
	MetaFiles           []InstalledFile    `json:"meta_files"`           // List of meta files with their hashes
	DataFiles           []InstalledFile    `json:"data_files"`           // List of data files with their hashes
	ReverseDependencies []string           `json:"reverse_dependencies"` // List of artifact names that depend on this artifact
	Status              ArtifactStatus     `json:"status"`               // Status of the artifact
	Checksum            string             `json:"checksum"`
	InstallationReason  InstallationReason `json:"installation_reason"` // Why this artifact was installed
}
