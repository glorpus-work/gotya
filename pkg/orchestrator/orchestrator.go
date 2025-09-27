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

	// Load currently installed artifacts for compatibility checking (only if not dry run)
	var installedArtifacts []*model.InstalledArtifact
	if o.ArtifactManager != nil {
		var err error
		installedArtifacts, err = o.ArtifactManager.GetInstalledArtifacts()
		if err != nil {
			return fmt.Errorf("failed to load installed artifacts: %w", err)
		}
	}

	// Create a copy of the request with installed artifacts for resolution (only if we have artifacts)
	resolveReq := req
	if len(installedArtifacts) > 0 {
		resolveReq.InstalledArtifacts = installedArtifacts
	}

	plan, err := o.Index.Resolve(ctx, resolveReq)
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
		var actionMsg string
		switch step.Action {
		case model.ResolvedActionInstall:
			actionMsg = "installing"
		case model.ResolvedActionUpdate:
			actionMsg = "updating"
		case model.ResolvedActionSkip:
			emit(o.Hooks, Event{Phase: "skipping", ID: step.ID, Msg: step.Reason})
			continue
		default:
			actionMsg = "processing"
		}

		emit(o.Hooks, Event{Phase: actionMsg, ID: step.ID, Msg: step.Name + "@" + step.Version + " (" + step.Reason + ")"})
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

// Cleanup removes orphaned automatic artifacts that have no reverse dependencies.
// Returns the list of artifacts that were successfully cleaned up.
func (o *Orchestrator) Cleanup(ctx context.Context) ([]string, error) {
	if o.ArtifactManager == nil {
		return nil, fmt.Errorf("artifact manager is not configured")
	}

	// Get orphaned automatic artifacts
	orphaned, err := o.ArtifactManager.GetOrphanedAutomaticArtifacts()
	if err != nil {
		return nil, fmt.Errorf("failed to get orphaned artifacts: %w", err)
	}

	if len(orphaned) == 0 {
		return nil, nil // Nothing to clean up
	}

	var cleaned []string

	// Uninstall each orphaned artifact
	for _, artifactName := range orphaned {
		emit(o.Hooks, Event{Phase: "cleanup", ID: artifactName, Msg: fmt.Sprintf("removing orphaned automatic artifact %s", artifactName)})

		if err := o.ArtifactManager.UninstallArtifact(ctx, artifactName, true); err != nil {
			// Log error but continue with other artifacts
			emit(o.Hooks, Event{Phase: "error", ID: artifactName, Msg: fmt.Sprintf("failed to cleanup %s: %v", artifactName, err)})
			continue
		}

		cleaned = append(cleaned, artifactName)
	}

	if len(cleaned) > 0 {
		emit(o.Hooks, Event{Phase: "done", Msg: fmt.Sprintf("cleaned up %d orphaned artifacts", len(cleaned))})
	}

	return cleaned, nil
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
