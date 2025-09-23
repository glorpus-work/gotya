// Package artifact provides functionality for managing software artifacts in the gotya package manager.
// It handles the installation, verification, and management of package artifacts, including
// their metadata, file structure, and dependencies. The package provides interfaces and
// implementations for working with artifacts across different platforms and architectures.
package artifact

const (
	artifactSuffix  = "gotya"
	artifactMetaDir = "meta"
	artifactDataDir = "data"
	metadataFile    = "artifact.json"
)
