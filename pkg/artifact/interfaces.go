package artifact

import (
	"context"

	"github.com/glorpus-work/gotya/pkg/model"
)

// Manager defines the interface for artifact management operations.
type Manager interface {
	// InstallArtifact installs (verifies/stages) an artifact strictly from a local file.
	// The descriptor must describe the artifact and localPath must point to the local archive file.
	InstallArtifact(ctx context.Context, desc *model.IndexArtifactDescriptor, localPath string, reason model.InstallationReason) error
	UninstallArtifact(ctx context.Context, artifactName string, purge bool) error
	// UpdateArtifact updates an installed artifact by replacing it with a new version.
	// Uses the simple approach: uninstall the old version, then install the new version.
	UpdateArtifact(ctx context.Context, newArtifactPath string, newDescriptor *model.IndexArtifactDescriptor) error
	// VerifyArtifact verifies an artifact in the cache without installing it.
	VerifyArtifact(ctx context.Context, artifact *model.IndexArtifactDescriptor) error
	// ReverseResolve returns the list of artifacts that depend on the given artifact recursively
	ReverseResolve(ctx context.Context, req model.ResolveRequest) (model.ResolvedArtifacts, error)
	// GetOrphanedAutomaticArtifacts returns all installed artifacts that are automatic and have no reverse dependencies
	GetOrphanedAutomaticArtifacts() ([]string, error)
	// GetInstalledArtifacts returns all installed artifacts
	GetInstalledArtifacts() ([]*model.InstalledArtifact, error)
	SetArtifactManuallyInstalled(artifactName string) error
}
