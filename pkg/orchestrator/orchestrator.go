//go:generate mockgen -destination=./mocks/orchestrator.go . IndexPlanner,ArtifactInstaller,Downloader

package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/cperrin88/gotya/pkg/download"
	"github.com/cperrin88/gotya/pkg/index"
	"github.com/cperrin88/gotya/pkg/model"
)

// IndexPlanner is the subset of the index manager used by the orchestrator.
type IndexPlanner interface {
	Plan(ctx context.Context, req index.InstallRequest) (index.InstallPlan, error)
}

// ArtifactInstaller is the subset of the artifact manager used by the orchestrator.
type ArtifactInstaller interface {
	InstallArtifact(ctx context.Context, desc *model.IndexArtifactDescriptor, localPath string) error
}

type Downloader interface {
	FetchAll(ctx context.Context, items []download.Item, opts download.Options) (map[string]string, error)
}

// Orchestrator ties Index, Download and Artifact managers together for installs.
type Orchestrator struct {
	Index    IndexPlanner
	DL       Downloader
	Artifact ArtifactInstaller
	Hooks    Hooks // Hooks for progress and event notifications
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
func (o *Orchestrator) Install(ctx context.Context, req index.InstallRequest, opts Options) error {
	if o.Index == nil {
		return fmt.Errorf("index planner is not configured")
	}

	emit(o.Hooks, Event{Phase: "planning", Msg: req.Name})
	plan, err := o.Index.Plan(ctx, req)
	if err != nil {
		return err
	}

	// Dry run: just emit steps and return
	if opts.DryRun {
		for _, step := range plan.Steps {
			emit(o.Hooks, Event{Phase: "planning", ID: step.ID, Msg: step.Name + "@" + step.Version})
		}
		emit(o.Hooks, Event{Phase: "done", Msg: "dry-run"})
		return nil
	}

	// Prefetch via Download Manager and capture paths (required for local-only installs)
	var fetched map[string]string
	if o.DL != nil && filepath.IsAbs(opts.CacheDir) {
		items := make([]download.Item, 0, len(plan.Steps))
		for _, s := range plan.Steps {
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

	if o.Artifact == nil {
		return fmt.Errorf("artifact installer is not configured")
	}

	for _, step := range plan.Steps {
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
		if err := o.Artifact.InstallArtifact(ctx, desc, path); err != nil {
			return err
		}
	}
	emit(o.Hooks, Event{Phase: "done"})
	return nil
}

// New constructs a default Orchestrator from existing managers. Helper for wiring.
// Hooks can be nil if no event handling is needed.
func New(idx IndexPlanner, dl Downloader, am ArtifactInstaller, hooks Hooks) *Orchestrator {
	return &Orchestrator{
		Index:    idx,
		DL:       dl,
		Artifact: am,
		Hooks:    hooks,
	}
}
