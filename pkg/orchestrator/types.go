//go:generate mockgen -destination=./mocks/orchestrator.go . ArtifactResolver,ArtifactReverseResolver,ArtifactManager,Downloader

package orchestrator

import (
	"context"

	"github.com/cperrin88/gotya/pkg/download"
	"github.com/cperrin88/gotya/pkg/model"
)

// ArtifactResolver is the subset of the index manager used by the orchestrator.
type ArtifactResolver interface {
	Resolve(ctx context.Context, req model.ResolveRequest) (model.ResolvedArtifacts, error)
}

// ArtifactReverseResolver provides reverse dependency resolution.
type ArtifactReverseResolver interface {
	ReverseResolve(ctx context.Context, req model.ResolveRequest) (model.ResolvedArtifacts, error)
}

// ArtifactManager is the subset of the artifact manager used by the orchestrator.
type ArtifactManager interface {
	InstallArtifact(ctx context.Context, desc *model.IndexArtifactDescriptor, localPath string, reason model.InstallationReason) error
	UninstallArtifact(ctx context.Context, artifactName string, purge bool) error
	GetOrphanedAutomaticArtifacts() ([]string, error)
}

// Downloader handles artifact downloading.
type Downloader interface {
	FetchAll(ctx context.Context, items []download.Item, opts download.Options) (map[string]string, error)
}

// Orchestrator ties Index, Download and ArtifactManager managers together for installs.
type Orchestrator struct {
	Index           ArtifactResolver
	ReverseIndex    ArtifactReverseResolver
	DL              Downloader
	ArtifactManager ArtifactManager
	Hooks           Hooks // Hooks for progress and event notifications
}

// Event represents a simple progress notification.
type Event struct {
	Phase string // resolving|planning|downloading|installing|done|error
	ID    string // step ID
	Msg   string
}

// Hooks carries callbacks for progress events.
type Hooks struct {
	OnEvent func(Event)
}

// InstallOptions control orchestrator install execution.
type InstallOptions struct {
	CacheDir    string
	Concurrency int
	DryRun      bool
}

// UninstallOptions control orchestrator uninstall execution.
type UninstallOptions struct {
	DryRun    bool
	NoCascade bool // Only uninstall if no reverse dependencies, unless Force is true
	Force     bool // Force uninstall even with reverse dependencies
}

// Options control orchestrator execution.
type Options struct {
	CacheDir    string
	Concurrency int
	DryRun      bool
}
