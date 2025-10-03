// Package orchestrator provides high-level operations for managing artifacts,
// including installation, uninstallation, updates, and cleanup of orphaned artifacts.
// It coordinates between index resolution, downloading, and artifact management.
package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/glorpus-work/gotya/pkg/download"
	"github.com/glorpus-work/gotya/pkg/errors"
	"github.com/glorpus-work/gotya/pkg/index"
	"github.com/glorpus-work/gotya/pkg/model"
)

const phaseUpdating = "updating"

// SyncAll downloads index files for the provided repositories into indexDir.
// The caller decides which repositories to pass (e.g., enabled-only). No TTL/autosync logic here.
func (o *Orchestrator) SyncAll(ctx context.Context, repos []*index.Repository, indexDir string, opts Options) error {
	if o.DL == nil {
		return fmt.Errorf("download manager is not configured: %w", errors.ErrValidation)
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
		// If the index file doesn't exist (e.g., mocked downloader didn't actually create it), skip transformation
		indexPath := filepath.Join(indexDir, repo.Name+".json")
		if _, statErr := os.Stat(indexPath); statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return fmt.Errorf("failed to access index %s: %w", repo.Name, statErr)
		}
		if err := o.transformIndexURLs(repo, indexDir); err != nil {
			return fmt.Errorf("failed to transform URLs in index %s: %w", repo.Name, err)
		}
	}

	return nil
}

// Cleanup removes orphaned automatic artifacts that have no reverse dependencies.
// Returns the list of artifacts that were successfully cleaned up.
func (o *Orchestrator) Cleanup(ctx context.Context) ([]string, error) {
	if o.ArtifactManager == nil {
		return nil, fmt.Errorf("artifact manager is not configured: %w", errors.ErrValidation)
	}

	// Get orphaned automatic artifacts
	orphaned, err := o.ArtifactManager.GetOrphanedAutomaticArtifacts()
	if err != nil {
		return nil, fmt.Errorf("failed to get orphaned artifacts: %w", err)
	}

	if len(orphaned) == 0 {
		return nil, nil
	}

	var cleaned []string
	for _, artifactName := range orphaned {
		emit(o.Hooks, Event{Phase: "cleanup", ID: artifactName, Msg: fmt.Sprintf("removing orphaned automatic artifact %s", artifactName)})
		if err := o.ArtifactManager.UninstallArtifact(ctx, artifactName, true); err != nil {
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
		return fmt.Errorf("artifact manager is not configured: %w", errors.ErrValidation)
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

	// Filter packages for update
	packagesToUpdate, err := o.filterPackagesForUpdate(installed, opts)
	if err != nil || packagesToUpdate == nil {
		return err
	}

	if o.Index == nil {
		return fmt.Errorf("index resolver is not configured: %w", errors.ErrNoRepositories)
	}

	// Resolve the update plan
	updateRequests := buildUpdateRequests(installed, packagesToUpdate)
	emit(o.Hooks, Event{Phase: "planning", Msg: fmt.Sprintf("resolving updates for %d packages", len(updateRequests))})
	plan, err := o.Index.Resolve(ctx, updateRequests)
	if err != nil {
		return fmt.Errorf("failed to resolve update plan: %w", err)
	}

	// Handle dry run
	if opts.DryRun {
		o.handleDryRunUpdate(plan)
		return nil
	}

	// Check if updates are needed
	if !checkForUpdates(plan) {
		emit(o.Hooks, Event{Phase: "done", Msg: "all packages are already at the latest compatible versions"})
		return nil
	}

	// Execute updates and report results
	return o.executeUpdateWithResults(ctx, plan, opts)
}

// filterPackagesForUpdate filters installed artifacts to determine which packages should be updated.
func (o *Orchestrator) filterPackagesForUpdate(installed []*model.InstalledArtifact, opts UpdateOptions) ([]*model.InstalledArtifact, error) {
	// Filter to specific packages if requested
	var packagesToUpdate []*model.InstalledArtifact
	if len(opts.Packages) > 0 {
		packageMap := make(map[string]*model.InstalledArtifact)
		for _, pkg := range installed {
			packageMap[pkg.Name] = pkg
		}
		for _, name := range opts.Packages {
			if pkg, ok := packageMap[name]; ok {
				packagesToUpdate = append(packagesToUpdate, pkg)
			} else {
				return nil, fmt.Errorf("package %s is not installed: %w", name, errors.ErrArtifactNotFound)
			}
		}
	} else {
		packagesToUpdate = installed
	}
	if len(packagesToUpdate) == 0 {
		emit(o.Hooks, Event{Phase: "done", Msg: "no packages to update"})
		return nil, nil
	}
	return packagesToUpdate, nil
}

// handleDryRunUpdate processes dry run for update operations.
func (o *Orchestrator) handleDryRunUpdate(plan model.ResolvedArtifacts) {
	for _, step := range plan.Artifacts {
		emit(o.Hooks, Event{Phase: phaseUpdating, ID: step.GetID(), Msg: step.Name + "@" + step.Version})
	}
	emit(o.Hooks, Event{Phase: "done", Msg: "update dry-run completed"})
}

// checkForUpdates determines if there are actual updates to perform.
func checkForUpdates(plan model.ResolvedArtifacts) bool {
	hasUpdates := false
	for _, step := range plan.Artifacts {
		if step.Action == model.ResolvedActionInstall || step.Action == model.ResolvedActionUpdate {
			hasUpdates = true
			break
		}
	}
	return hasUpdates
}

// executeUpdateWithResults handles the update execution and result reporting.
func (o *Orchestrator) executeUpdateWithResults(ctx context.Context, plan model.ResolvedArtifacts, opts UpdateOptions) error {
	// Prefetch and execute
	fetched, err := o.prefetchPlanArtifacts(ctx, plan, download.Options{Dir: opts.CacheDir, Concurrency: opts.Concurrency})
	if err != nil {
		return fmt.Errorf("failed to prefetch updates: %w", err)
	}
	updatedCount, newlyInstalledCount, err := o.executeUpdatePlan(ctx, plan, fetched)
	if err != nil {
		return err
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

// transformIndexURLs converts relative URLs in the downloaded index to absolute URLs
// based on the repository server URL.
func (o *Orchestrator) transformIndexURLs(repo *index.Repository, indexDir string) error {
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
func (o *Orchestrator) Install(ctx context.Context, requests []*model.ResolveRequest, opts InstallOptions) error {
	if o.Index == nil {
		return fmt.Errorf("index planner is not configured: %w", errors.ErrValidation)
	}

	emit(o.Hooks, Event{Phase: "planning", Msg: fmt.Sprintf("installing %d packages", len(requests))})
	allRequests, err := o.buildInstallRequests(requests)
	if err != nil {
		return err
	}

	plan, err := o.Index.Resolve(ctx, allRequests)
	if err != nil {
		return err
	}

	// Dry run: just emit steps and return
	if opts.DryRun {
		for _, step := range plan.Artifacts {
			emit(o.Hooks, Event{Phase: "planning", ID: step.GetID(), Msg: step.Name + "@" + step.Version})
		}
		emit(o.Hooks, Event{Phase: "done", Msg: "dry-run"})
		return nil
	}

	// Prefetch via Download Manager and capture paths (required for local-only installs)
	fetched, err := o.prefetchPlanArtifacts(ctx, plan, download.Options{Dir: opts.CacheDir, Concurrency: opts.Concurrency})
	if err != nil {
		return err
	}

	if o.ArtifactManager == nil {
		return fmt.Errorf("artifact installer is not configured: %w", errors.ErrValidation)
	}

	if err := o.executeInstallPlan(ctx, plan, requests, fetched); err != nil {
		return err
	}
	emit(o.Hooks, Event{Phase: "done"})
	return nil
}

// buildInstallRequests loads installed artifacts and combines them with incoming requests
// adding keep preferences for installed packages not explicitly requested.
func (o *Orchestrator) buildInstallRequests(requests []*model.ResolveRequest) ([]*model.ResolveRequest, error) {
	// Load currently installed artifacts for compatibility checking
	var installedArtifacts []*model.InstalledArtifact
	if o.ArtifactManager != nil {
		var err error
		installedArtifacts, err = o.ArtifactManager.GetInstalledArtifacts()
		if err != nil {
			return nil, fmt.Errorf("failed to load installed artifacts: %w", err)
		}
	}

	installedMap := make(map[string]*model.ResolveRequest)
	for _, req := range requests {
		installedMap[req.Name] = req
	}

	allRequests := make([]*model.ResolveRequest, len(requests))
	copy(allRequests, requests)

	for _, installed := range installedArtifacts {
		if installedMap[installed.Name] == nil {
			allRequests = append(allRequests, &model.ResolveRequest{
				Name:              installed.Name,
				VersionConstraint: "",
				OS:                installed.OS,
				Arch:              installed.Arch,
				OldVersion:        installed.Version,
				KeepVersion:       true,
			})
		} else {
			installedMap[installed.Name].OldVersion = installed.Version
		}
	}
	return allRequests, nil
}

// prefetchPlanArtifacts downloads artifacts for a plan when a downloader is configured.
func (o *Orchestrator) prefetchPlanArtifacts(ctx context.Context, plan model.ResolvedArtifacts, dlOpts download.Options) (map[string]string, error) {
	if o.DL == nil || !filepath.IsAbs(dlOpts.Dir) {
		return map[string]string{}, nil
	}
	items := make([]download.Item, 0, len(plan.Artifacts))
	for _, s := range plan.Artifacts {
		if s.SourceURL == nil {
			continue
		}
		items = append(items, download.Item{ID: s.GetID(), URL: s.SourceURL, Checksum: s.Checksum})
	}
	if len(items) == 0 {
		return map[string]string{}, nil
	}
	emit(o.Hooks, Event{Phase: "downloading", Msg: "prefetching artifacts"})
	fetched, err := o.DL.FetchAll(ctx, items, dlOpts)
	if err != nil {
		return nil, err
	}
	return fetched, nil
}

// executeInstallPlan installs/updates artifacts as instructed by the plan.
func (o *Orchestrator) executeInstallPlan(ctx context.Context, plan model.ResolvedArtifacts, requests []*model.ResolveRequest, fetched map[string]string) error {
	onlyUpdateReasonRequest := make([]*model.ResolveRequest, 0, len(requests))
	onlyUpdateReasonRequest = append(onlyUpdateReasonRequest, requests...)

	for _, step := range plan.Artifacts {
		reqIdx := slices.IndexFunc(onlyUpdateReasonRequest, func(req *model.ResolveRequest) bool {
			return req.Name == step.Name
		})
		if reqIdx != -1 {
			onlyUpdateReasonRequest = append(onlyUpdateReasonRequest[:reqIdx], onlyUpdateReasonRequest[reqIdx+1:]...)
		}

		var actionMsg string
		switch step.Action {
		case model.ResolvedActionInstall:
			actionMsg = "installing"
		case model.ResolvedActionUpdate:
			actionMsg = phaseUpdating
		}
		emit(o.Hooks, Event{Phase: actionMsg, ID: step.GetID(), Msg: step.Name + "@" + step.Version + " (" + step.Reason + ")"})

		path := ""
		if fetched != nil {
			path = fetched[step.GetID()]
		}
		if path == "" {
			return fmt.Errorf("no local file available for step %s; downloads are required for install: %w", step.GetID(), errors.ErrDownloadFailed)
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
		// Determine installation reason: requested packages are manual, others automatic
		reason := model.InstallationReasonAutomatic
		for _, req := range requests {
			if step.Name == req.Name {
				reason = model.InstallationReasonManual
				break
			}
		}
		switch step.Action {
		case model.ResolvedActionInstall:
			if err := o.ArtifactManager.InstallArtifact(ctx, desc, path, reason); err != nil {
				return err
			}
		case model.ResolvedActionUpdate:
			if err := o.ArtifactManager.UpdateArtifact(ctx, path, desc); err != nil {
				return err
			}
		}
	}

	for _, req := range onlyUpdateReasonRequest {
		if err := o.ArtifactManager.SetArtifactManuallyInstalled(req.Name); err != nil {
			return err
		}
	}

	return nil
}

// Uninstall resolves and uninstalls according to the reverse dependency plan (reverse order for dependencies).
func (o *Orchestrator) Uninstall(ctx context.Context, req model.ResolveRequest, opts UninstallOptions) error {
	emit(o.Hooks, Event{Phase: "planning", Msg: req.Name})

	// If both NoCascade and Force are true, skip reverse dependency resolution
	var artifacts model.ResolvedArtifacts
	var err error
	if opts.NoCascade && opts.Force {
		// Create a minimal artifact list with just the target artifact
		artifacts = model.ResolvedArtifacts{
			Artifacts: []model.ResolvedArtifact{
				{
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
			return fmt.Errorf("artifact %s has %d reverse dependencies; use --force to uninstall anyway: %w", req.Name, len(artifacts.Artifacts)-1, errors.ErrValidation)
		}
	}

	// Dry run: just emit steps and return
	if opts.DryRun {
		for _, step := range artifacts.Artifacts {
			emit(o.Hooks, Event{Phase: "planning", ID: step.GetID(), Msg: step.Name + "@" + step.Version})
		}
		emit(o.Hooks, Event{Phase: "done", Msg: "dry-run"})
		return nil
	}

	if o.ArtifactManager == nil {
		return fmt.Errorf("artifact uninstaller is not configured: %w", errors.ErrValidation)
	}

	// Process artifacts in reverse order to handle dependencies properly
	for _, artifact := range slices.Backward(artifacts.Artifacts) {
		emit(o.Hooks, Event{Phase: "uninstalling", ID: artifact.GetID(), Msg: artifact.Name + "@" + artifact.Version})
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

// buildUpdateRequests composes resolver requests for an update flow.
func buildUpdateRequests(installed, packagesToUpdate []*model.InstalledArtifact) []*model.ResolveRequest {
	reqs := make([]*model.ResolveRequest, 0, len(installed))
	requested := make(map[string]struct{}, len(packagesToUpdate))
	for _, pkg := range packagesToUpdate {
		reqs = append(reqs, &model.ResolveRequest{
			Name:              pkg.Name,
			VersionConstraint: ">= " + pkg.Version,
			OS:                pkg.OS,
			Arch:              pkg.Arch,
			OldVersion:        pkg.Version,
			KeepVersion:       false,
		})
		requested[pkg.Name] = struct{}{}
	}
	for _, inst := range installed {
		if _, already := requested[inst.Name]; already {
			continue
		}
		reqs = append(reqs, &model.ResolveRequest{
			Name:              inst.Name,
			VersionConstraint: ">= 0.0.0",
			OS:                inst.OS,
			Arch:              inst.Arch,
			OldVersion:        inst.Version,
			KeepVersion:       false,
		})
	}
	return reqs
}

// executeUpdatePlan runs the resolved update and install steps during update flow.
func (o *Orchestrator) executeUpdatePlan(ctx context.Context, plan model.ResolvedArtifacts, fetched map[string]string) (updatedCount, newlyInstalledCount int, err error) {
	for _, step := range plan.Artifacts {
		path := ""
		if fetched != nil {
			path = fetched[step.GetID()]
		}
		if path == "" {
			return 0, 0, fmt.Errorf("no local file available for update step %s: %w", step.GetID(), errors.ErrDownloadFailed)
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
		switch step.Action {
		case model.ResolvedActionUpdate:
			emit(o.Hooks, Event{Phase: "updating", ID: step.GetID(), Msg: step.Name + "@" + step.Version})
			if err := o.ArtifactManager.UpdateArtifact(ctx, path, desc); err != nil {
				return 0, 0, fmt.Errorf("failed to update %s: %w", step.Name, err)
			}
			updatedCount++
		case model.ResolvedActionInstall:
			emit(o.Hooks, Event{Phase: "installing", ID: step.GetID(), Msg: step.Name + "@" + step.Version})
			if err := o.ArtifactManager.InstallArtifact(ctx, desc, path, model.InstallationReasonAutomatic); err != nil {
				return 0, 0, fmt.Errorf("failed to install dependency %s: %w", step.Name, err)
			}
			newlyInstalledCount++
		}
	}
	return updatedCount, newlyInstalledCount, nil
}
