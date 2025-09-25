package model

import "net/url"

// ResolveRequest describes what the user asked to resolve.
type ResolveRequest struct {
	Name    string `json:"name"`
	Version string `json:"version"` // semver constraint (e.g., ">= 0.0.0" for latest)
	OS      string `json:"os"`      // target os
	Arch    string `json:"arch"`    // target arch
}

// ResolvedArtifact represents a concrete installation action.
type ResolvedArtifact struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Version   string   `json:"version"`
	OS        string   `json:"os"`
	Arch      string   `json:"arch"`
	SourceURL *url.URL `json:"source_url"`
	Checksum  string   `json:"checksum"`
}

// ResolvedArtifacts is an ordered list of steps, topologically sorted if dependencies are present.
type ResolvedArtifacts struct {
	Artifacts []ResolvedArtifact `json:"artifacts"`
}
