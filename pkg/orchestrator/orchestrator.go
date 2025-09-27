//go:generate mockgen -destination=./mocks/orchestrator.go . ArtifactResolver,ArtifactReverseResolver,ArtifactManager,Downloader

package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/cperrin88/gotya/pkg/download"
	"github.com/cperrin88/gotya/pkg/index"
	"github.com/cperrin88/gotya/pkg/model"
)

// ArtifactResolver is the subset of the index manager used by the orchestrator.
type ArtifactResolver interface {
	Resolve(ctx context.Context, req model.ResolveRequest) (model.ResolvedArtifacts, error)
}

type ArtifactReverseResolver interface {
	ReverseResolve(ctx context.Context, req model.ResolveRequest) (model.ResolvedArtifacts, error)
}

// ArtifactManager is the subset of the artifact manager used by the orchestrator.
type ArtifactManager interface {
	InstallArtifact(ctx context.Context, desc *model.IndexArtifactDescriptor, localPath string, reason model.InstallationReason) error
	UninstallArtifact(ctx context.Context, artifactName string, purge bool) error
}

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

// SyncAll downloads index files for the provided repositories into indexDir.
// The caller decides which repositories to pass (e.g., enabled-only). No TTL/autosync logic here.
func (o *Orchestrator) SyncAll(ctx context.Context, repos []*index.Repository, indexDir string, opts Options) error {
	if o.DL == nil {
		return fmt.Errorf("download manager is not configured")
	}
	items := make([]download.Item, 0, len(repos))
	for _, r := range repos {
		if r == nil || r.URL == nil {
			continue
		}
		items = append(items, download.Item{
			ID:       r.Name,
			URL:      r.URL,
			Filename: r.Name + ".json",
		})
	}
	if len(items) == 0 {
		return nil
	}
	_, err := o.DL.FetchAll(ctx, items, download.Options{Dir: indexDir, Concurrency: opts.Concurrency})
	return err
}

func emit(h Hooks, e Event) {
	if h.OnEvent != nil {
		h.OnEvent(e)
	}
}

// Install resolves and installs according to the plan (sequentially for now).
func (o *Orchestrator) Install(ctx context.Context, req model.ResolveRequest, opts InstallOptions) error {
	if o.Index == nil {
		return fmt.Errorf("index planner is not configured")
	}

	emit(o.Hooks, Event{Phase: "planning", Msg: req.Name})
	plan, err := o.Index.Resolve(ctx, req)
	if err != nil {
		return err
	}

	// Dry run: just emit steps and return
	if opts.DryRun {
		for _, step := range plan.Artifacts {
			emit(o.Hooks, Event{Phase: "planning", ID: step.ID, Msg: step.Name + "@" + step.Version})
		}
		emit(o.Hooks, Event{Phase: "done", Msg: "dry-run"})
		return nil
	}

	// Prefetch via Download Manager and capture paths (required for local-only installs)
	var fetched map[string]string
	if o.DL != nil && filepath.IsAbs(opts.CacheDir) {
		items := make([]download.Item, 0, len(plan.Artifacts))
		for _, s := range plan.Artifacts {
			if s.SourceURL == nil { // nothing to prefetch
				continue
			}
			items = append(items, download.Item{ID: s.ID, URL: s.SourceURL, Checksum: s.Checksum})
		}
		if len(items) > 0 {
			emit(o.Hooks, Event{Phase: "downloading", Msg: "prefetching artifacts"})
			var err error
			fetched, err = o.DL.FetchAll(ctx, items, download.Options{Dir: opts.CacheDir, Concurrency: opts.Concurrency})
			if err != nil {
				return err
			}
		}
	}

	if o.ArtifactManager == nil {
		return fmt.Errorf("artifact installer is not configured")
	}

	for _, step := range plan.Artifacts {
		emit(o.Hooks, Event{Phase: "installing", ID: step.ID, Msg: step.Name + "@" + step.Version})
		path := ""
		if fetched != nil {
			path = fetched[step.ID]
		}
		if path == "" {
			return fmt.Errorf("no local file available for step %s; downloads are required for install", step.ID)
		}
		desc := &model.IndexArtifactDescriptor{
			Name:     step.Name,
			Version:  step.Version,
			OS:       step.OS,
			Arch:     step.Arch,
			Checksum: step.Checksum,
			URL:      "",
		}
		if step.SourceURL != nil {
			desc.URL = step.SourceURL.String()
		}

		// Determine installation reason: first artifact is manual, rest are automatic (dependencies)
		reason := model.InstallationReasonAutomatic
		if step.Name == req.Name {
			reason = model.InstallationReasonManual
		}

		if err := o.ArtifactManager.InstallArtifact(ctx, desc, path, reason); err != nil {
			return err
		}
	}
	emit(o.Hooks, Event{Phase: "done"})
	return nil
}

// Uninstall resolves and uninstalls according to the reverse dependency plan (reverse order for dependencies).
func (o *Orchestrator) Uninstall(ctx context.Context, req model.ResolveRequest, opts UninstallOptions) error {
	if o.ReverseIndex == nil {
		return fmt.Errorf("reverse index resolver is not configured")
	}

	emit(o.Hooks, Event{Phase: "planning", Msg: req.Name})

	// If both NoCascade and Force are true, skip reverse dependency resolution
	var artifacts model.ResolvedArtifacts
	var err error
	if opts.NoCascade && opts.Force {
		// Create a minimal artifact list with just the target artifact
		artifacts = model.ResolvedArtifacts{
			Artifacts: []model.ResolvedArtifact{
				{
					ID:       req.Name + "@" + req.Version,
					Name:     req.Name,
					Version:  req.Version,
					OS:       req.OS,
					Arch:     req.Arch,
					Checksum: "",
				},
			},
		}
	} else {
		artifacts, err = o.ReverseIndex.ReverseResolve(ctx, req)
		if err != nil {
			return err
		}

		// Check NoCascade option
		if opts.NoCascade && len(artifacts.Artifacts) > 1 {
			return fmt.Errorf("artifact %s has %d reverse dependencies; use --force to uninstall anyway", req.Name, len(artifacts.Artifacts)-1)
		}
	}

	// Dry run: just emit steps and return
	if opts.DryRun {
		for _, step := range artifacts.Artifacts {
			emit(o.Hooks, Event{Phase: "planning", ID: step.ID, Msg: step.Name + "@" + step.Version})
		}
		emit(o.Hooks, Event{Phase: "done", Msg: "dry-run"})
		return nil
	}

	if o.ArtifactManager == nil {
		return fmt.Errorf("artifact uninstaller is not configured")
	}

	// Process artifacts in reverse order to handle dependencies properly
	// Reverse the slice to uninstall dependencies first
	for _, artifact := range slices.Backward(artifacts.Artifacts) {
		emit(o.Hooks, Event{Phase: "uninstalling", ID: artifact.ID, Msg: artifact.Name + "@" + artifact.Version})
		if err := o.ArtifactManager.UninstallArtifact(ctx, artifact.Name, false); err != nil {
			return err
		}
	}
	emit(o.Hooks, Event{Phase: "done"})
	return nil
}

// New constructs a default Orchestrator from existing managers. Helper for wiring.
// Hooks can be nil if no event handling is needed.
func New(idx ArtifactResolver, reverseIdx ArtifactReverseResolver, dl Downloader, am ArtifactManager, hooks Hooks) *Orchestrator {
	return &Orchestrator{
		Index:           idx,
		ReverseIndex:    reverseIdx,
		DL:              dl,
		ArtifactManager: am,
		Hooks:           hooks,
	}
}
