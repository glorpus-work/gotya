package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

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

	// Download all indexes
	_, err := o.DL.FetchAll(ctx, items, download.Options{Dir: indexDir, Concurrency: opts.Concurrency})
	if err != nil {
		return err
	}

	// Transform relative URLs in downloaded indexes to absolute URLs
	for _, repo := range repos {
		if repo == nil || repo.URL == nil {
			continue
		}
		if err := o.transformIndexURLs(ctx, repo, indexDir); err != nil {
			return fmt.Errorf("failed to transform URLs in index %s: %w", repo.Name, err)
		}
	}

	return nil
}

// transformIndexURLs converts relative URLs in the downloaded index to absolute URLs
// based on the repository server URL.
func (o *Orchestrator) transformIndexURLs(ctx context.Context, repo *index.Repository, indexDir string) error {
	indexPath := filepath.Join(indexDir, repo.Name+".json")

	// Parse the index
	idx, err := index.ParseIndexFromFile(indexPath)
	if err != nil {
		return fmt.Errorf("failed to parse index: %w", err)
	}

	// Transform relative URLs to absolute URLs
	modified := false
	for _, artifact := range idx.Artifacts {
		if artifact.URL != "" && !strings.HasPrefix(artifact.URL, "http") {
			// This is a relative URL, convert it to absolute
			repoURL := repo.URL.String()
			// Remove the index.json part from the repository URL
			baseURL := strings.TrimSuffix(repoURL, "/index.json")
			if !strings.HasSuffix(baseURL, "/") {
				baseURL += "/"
			}
			artifact.URL = baseURL + artifact.URL
			modified = true
		}
	}

	// Write back the modified index if URLs were transformed
	if modified {
		if err := index.WriteIndexToFile(idx, indexPath); err != nil {
			return fmt.Errorf("failed to write transformed index: %w", err)
		}
	}

	return nil
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

	// Load currently installed artifacts for compatibility checking
	var installedArtifacts []*model.InstalledArtifact
	if o.ArtifactManager != nil {
		var err error
		installedArtifacts, err = o.ArtifactManager.GetInstalledArtifacts()
		if err != nil {
			return fmt.Errorf("failed to load installed artifacts: %w", err)
		}
	}

	// Build resolve requests: main request + keep preferences for installed artifacts
	requests := []model.ResolveRequest{
		{
			Name:              req.Name,
			VersionConstraint: req.VersionConstraint,
			OS:                req.OS,
			Arch:              req.Arch,
		},
	}

	// Add keep preferences for all installed artifacts
	for _, installed := range installedArtifacts {
		requests = append(requests, model.ResolveRequest{
			Name:              installed.Name,
			VersionConstraint: "", // No hard constraint, just preference
			OS:                req.OS,
			Arch:              req.Arch,
			OldVersion:        installed.Version,
			KeepVersion:       true, // Prefer to keep current version
		})
	}

	plan, err := o.Index.Resolve(ctx, requests)
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

		// Determine installation reason: main requested package is manual, others are automatic
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
					ID:       req.Name + "@" + req.VersionConstraint,
					Name:     req.Name,
					Version:  req.VersionConstraint,
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

// Update resolves and updates packages to their latest compatible versions.
func (o *Orchestrator) Update(ctx context.Context, opts UpdateOptions) error {
	if o.ArtifactManager == nil {
		return fmt.Errorf("artifact manager is not configured")
	}

	emit(o.Hooks, Event{Phase: "planning", Msg: "analyzing installed packages"})

	// Get all installed artifacts
	installed, err := o.ArtifactManager.GetInstalledArtifacts()
	if err != nil {
		return fmt.Errorf("failed to get installed artifacts: %w", err)
	}

	if len(installed) == 0 {
		emit(o.Hooks, Event{Phase: "done", Msg: "no packages installed to update"})
		return nil
	}

	// Filter to specific packages if requested
	var packagesToUpdate []*model.InstalledArtifact
	if len(opts.Packages) > 0 {
		packageMap := make(map[string]*model.InstalledArtifact)
		for _, pkg := range installed {
			packageMap[pkg.Name] = pkg
		}

		for _, pkgName := range opts.Packages {
			if pkg, exists := packageMap[pkgName]; exists {
				packagesToUpdate = append(packagesToUpdate, pkg)
			} else {
				return fmt.Errorf("package %s is not installed", pkgName)
			}
		}
	} else {
		packagesToUpdate = installed
	}

	if len(packagesToUpdate) == 0 {
		emit(o.Hooks, Event{Phase: "done", Msg: "no packages to update"})
		return nil
	}

	// Index resolver is required for updates
	if o.Index == nil {
		return fmt.Errorf("index resolver is not configured")
	}

	// Build update requests for each package
	requests := make([]model.ResolveRequest, 0, len(packagesToUpdate))
	for _, pkg := range packagesToUpdate {
		// Create update request: get latest version compatible with current or higher
		requests = append(requests, model.ResolveRequest{
			Name:              pkg.Name,
			VersionConstraint: ">= " + pkg.Version, // Update to current version or higher
			OS:                "linux",             // TODO: Get from system or config
			Arch:              "amd64",             // TODO: Get from system or config
			OldVersion:        pkg.Version,         // Current version for reference
			KeepVersion:       false,               // Always update to latest compatible
		})
	}

	emit(o.Hooks, Event{Phase: "planning", Msg: fmt.Sprintf("resolving updates for %d packages", len(requests))})

	// Resolve the update plan
	plan, err := o.Index.Resolve(ctx, requests)
	if err != nil {
		return fmt.Errorf("failed to resolve update plan: %w", err)
	}

	// Dry run: just emit steps and return
	if opts.DryRun {
		for _, step := range plan.Artifacts {
			var actionMsg string
			switch step.Action {
			case model.ResolvedActionInstall:
				actionMsg = "updating"
			case model.ResolvedActionUpdate:
				actionMsg = "updating"
			case model.ResolvedActionSkip:
				actionMsg = "skipping"
			default:
				actionMsg = "processing"
			}
			emit(o.Hooks, Event{Phase: actionMsg, ID: step.ID, Msg: step.Name + "@" + step.Version})
		}
		emit(o.Hooks, Event{Phase: "done", Msg: "update dry-run completed"})
		return nil
	}

	// Check if there are any actual updates to perform
	hasUpdates := false
	for _, step := range plan.Artifacts {
		if step.Action == model.ResolvedActionInstall || step.Action == model.ResolvedActionUpdate {
			hasUpdates = true
			break
		}
	}

	if !hasUpdates {
		emit(o.Hooks, Event{Phase: "done", Msg: "all packages are already at the latest compatible versions"})
		return nil
	}

	// Prefetch artifacts if download manager is available
	var fetched map[string]string
	if o.DL != nil {
		items := make([]download.Item, 0, len(plan.Artifacts))
		for _, step := range plan.Artifacts {
			if step.SourceURL == nil {
				continue
			}
			items = append(items, download.Item{
				ID:       step.ID,
				URL:      step.SourceURL,
				Checksum: step.Checksum,
			})
		}

		if len(items) > 0 {
			emit(o.Hooks, Event{Phase: "downloading", Msg: "prefetching updates"})
			fetched, err = o.DL.FetchAll(ctx, items, download.Options{
				Dir:         opts.CacheDir,
				Concurrency: opts.Concurrency,
			})
			if err != nil {
				return fmt.Errorf("failed to prefetch updates: %w", err)
			}
		}
	}

	// Execute the updates
	updatedCount := 0
	newlyInstalledCount := 0
	for _, step := range plan.Artifacts {
		if step.Action == model.ResolvedActionSkip {
			emit(o.Hooks, Event{Phase: "skipping", ID: step.ID, Msg: step.Reason})
			continue
		}

		if step.Action != model.ResolvedActionInstall && step.Action != model.ResolvedActionUpdate {
			continue
		}

		path := ""
		if fetched != nil {
			path = fetched[step.ID]
		}
		if path == "" {
			return fmt.Errorf("no local file available for update step %s", step.ID)
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

		// Handle different actions appropriately
		if step.Action == model.ResolvedActionUpdate {
			// Use UpdateArtifact for existing packages being updated
			actionMsg := "updating"
			emit(o.Hooks, Event{Phase: actionMsg, ID: step.ID, Msg: step.Name + "@" + step.Version})

			// UpdateArtifact method signature changed - it takes newArtifactPath and newDescriptor directly
			if err := o.ArtifactManager.UpdateArtifact(ctx, path, desc); err != nil {
				return fmt.Errorf("failed to update %s: %w", step.Name, err)
			}
			updatedCount++
		} else if step.Action == model.ResolvedActionInstall {
			// Use InstallArtifact for new packages (dependencies)
			actionMsg := "installing"
			emit(o.Hooks, Event{Phase: actionMsg, ID: step.ID, Msg: step.Name + "@" + step.Version})

			// Install as automatic since it's a new dependency
			if err := o.ArtifactManager.InstallArtifact(ctx, desc, path, model.InstallationReasonAutomatic); err != nil {
				return fmt.Errorf("failed to install dependency %s: %w", step.Name, err)
			}
			newlyInstalledCount++
		}
	}

	if updatedCount > 0 || newlyInstalledCount > 0 {
		msg := fmt.Sprintf("successfully updated %d packages", updatedCount)
		if newlyInstalledCount > 0 {
			msg += fmt.Sprintf(" and installed %d new dependencies", newlyInstalledCount)
		}
		emit(o.Hooks, Event{Phase: "done", Msg: msg})
	} else {
		emit(o.Hooks, Event{Phase: "done", Msg: "no updates were performed"})
	}

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
