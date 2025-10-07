package artifact

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/glorpus-work/gotya/pkg/archive"
	"github.com/glorpus-work/gotya/pkg/artifact/database"
	"github.com/glorpus-work/gotya/pkg/errors"
	"github.com/glorpus-work/gotya/pkg/fsutil"
	"github.com/glorpus-work/gotya/pkg/model"
)

// ManagerImpl is the default implementation of the Manager interface for artifact operations.
// It handles installation, uninstallation, updates, and verification of artifacts.
type ManagerImpl struct {
	os                     string
	arch                   string
	artifactCacheDir       string
	artifactDataInstallDir string
	artifactMetaInstallDir string
	verifier               *Verifier
	archiveExtractor       ArchiveExtractor
	hookExecutor           HookExecutor
	installDB              database.InstalledManager
}

// NewManager creates a new artifact manager instance with the specified configuration.
// It initializes the manager with OS/arch info, cache directories, install directories, and database path.
func NewManager(operatingSystem, arch, artifactCacheDir, artifactInstallDir, artifactMetaInstallDir, installedDBPath string) *ManagerImpl {
	return &ManagerImpl{
		os:                     operatingSystem,
		arch:                   arch,
		artifactCacheDir:       artifactCacheDir,
		artifactDataInstallDir: artifactInstallDir,
		artifactMetaInstallDir: artifactMetaInstallDir,
		verifier:               NewVerifier(),
		archiveExtractor:       archive.NewManager(),
		hookExecutor:           NewHookExecutor(),
		installDB:              database.NewInstalledMangerWithPath(installedDBPath),
	}
}

// SetArtifactManuallyInstalled marks an artifact as manually installed.
func (m *ManagerImpl) SetArtifactManuallyInstalled(artifactName string) error {
	if err := m.loadInstalledDB(); err != nil {
		return errors.Wrapf(err, "failure to change artifact install reason for %s", artifactName)
	}
	artifact := m.installDB.FindArtifact(artifactName)
	if artifact == nil {
		return errors.Wrapf(errors.ErrArtifactNotFound, "failure to change artifact install reason for %s", artifactName)
	}
	artifact.InstallationReason = model.InstallationReasonManual
	return m.installDB.SaveDatabase()
}

// InstallArtifact installs an artifact from a local file path.
func (m *ManagerImpl) InstallArtifact(ctx context.Context, desc *model.IndexArtifactDescriptor, localPath string, reason model.InstallationReason) error {
	// Input validation
	if desc == nil {
		return errors.Wrap(errors.ErrValidation, "artifact descriptor cannot be nil")
	}
	if err := desc.Verify(); err != nil {
		return errors.Wrap(err, "invalid artifact descriptor")
	}
	if localPath == "" {
		return errors.Wrap(errors.ErrValidation, "local path cannot be empty")
	}

	var installed bool
	var err error
	defer func() {
		if err != nil && installed {
			// If we installed files but then failed, clean them up
			m.installRollback(desc.Name)
		}
	}()

	extractDir, err := os.MkdirTemp("", fmt.Sprintf("gotya-extract-%s-%s", desc.Name, desc.Version))
	defer func() { _ = os.RemoveAll(extractDir) }()

	err = m.extractAndVerify(ctx, desc, localPath, extractDir)
	if err != nil {
		return err
	}

	// Load or create the installed database
	err = m.loadInstalledDB()
	if err != nil {
		return err
	}

	done, artifact, err := m.handleExistingArtifact(desc.Name, reason)
	if err != nil {
		return err
	}
	if done {
		return nil
	}
	var existingReverseDeps []string
	if artifact != nil {
		existingReverseDeps = artifact.ReverseDependencies
		reason = artifact.InstallationReason
	}

	err = m.excutePreInstallHook(desc, extractDir)
	if err != nil {
		return err
	}

	// Perform the actual installation (includes hook execution)
	err = m.performInstallation(extractDir, desc, reason, existingReverseDeps)
	if err != nil {
		return err
	}
	installed = true

	err = m.executePostInstallHook(desc)
	if err != nil {
		return err
	}

	return nil
}

// UninstallArtifact removes an installed artifact from the system.
func (m *ManagerImpl) UninstallArtifact(ctx context.Context, artifactName string, purge bool) error {
	// Input validation
	if artifactName == "" {
		return fmt.Errorf("artifact name cannot be empty: %w", errors.ErrValidation)
	}

	// Load the installed database
	if err := m.installDB.LoadDatabase(); err != nil {
		return fmt.Errorf("failed to load installed database: %w", err)
	}

	// Check if the artifact is installed
	if !m.installDB.IsArtifactInstalled(artifactName) {
		return fmt.Errorf("artifact %s is not installed: %w", artifactName, errors.ErrArtifactNotFound)
	}

	artifact := m.installDB.FindArtifact(artifactName)
	if artifact == nil {
		return fmt.Errorf("artifact %s not found in database: %w", artifactName, errors.ErrArtifactNotFound)
	}

	metadata, err := ParseMetadataFromPath(filepath.Join(artifact.ArtifactMetaDir, metadataFile))
	if err != nil {
		return err
	}
	err = m.executePreUninstallHook(artifact, metadata)
	if err != nil {
		return err
	}

	script, err := m.preservePostUninstallHookScript(artifact.ArtifactMetaDir, metadata)
	if err != nil {
		return err
	}

	// Handle purge mode
	if purge {
		err = m.uninstallWithPurge(ctx, m.installDB, artifact)
		if err != nil {
			return err
		}
	}

	err = m.uninstallSelectively(ctx, m.installDB, artifact)
	if err != nil {
		return err
	}
	if script == "" {
		return nil
	}
	defer func() {
		_ = os.Remove(script)
	}()

	err = m.executePostUninstallHook(artifact, script)
	if err != nil {
		return err
	}

	return nil
}

// UpdateArtifact updates an installed artifact by replacing it with a new version.
// This method uses the simple approach: uninstall the old version, then install the new version.
// If the installation fails, the old version remains uninstalled.
func (m *ManagerImpl) UpdateArtifact(ctx context.Context, newArtifactPath string, desc *model.IndexArtifactDescriptor) error {
	if desc == nil {
		return errors.Wrap(errors.ErrValidation, "new descriptor cannot be nil")
	}
	if err := desc.Verify(); err != nil {
		return errors.Wrap(err, "new descriptor is invalid")
	}
	if newArtifactPath == "" {
		return errors.Wrap(errors.ErrValidation, "new artifact path cannot be empty")
	}

	extractDir, err := os.MkdirTemp("", fmt.Sprintf("gotya-extract-%s-%s", desc.Name, desc.Version))
	if err != nil {
		return errors.Wrap(err, "failed to create extract directory")
	}
	defer func() { _ = os.RemoveAll(extractDir) }()

	err = m.extractAndVerify(ctx, desc, newArtifactPath, extractDir)
	if err != nil {
		return err
	}

	// Load the installed database
	err = m.loadInstalledDB()
	if err != nil {
		return fmt.Errorf("failed to load installed database: %w", err)
	}

	// Validate the update request and get the installed artifact
	installedArtifact, err := m.validateUpdateRequest(desc)
	if err != nil {
		return err
	}

	// Execute pre-update hook before uninstalling old version
	if err := m.executePreUpdateHook(installedArtifact, desc); err != nil {
		return err
	}

	tempDataDir, tempMetaDir, err := m.backupInstallationFiles(installedArtifact)
	if err != nil {
		return err
	}
	oldArtifact := m.copyDBArtifact(installedArtifact)
	m.installDB.RemoveArtifact(installedArtifact.Name)

	defer func() {
		if err != nil {
			_ = m.restoreInstallationFiles(tempDataDir, tempMetaDir, installedArtifact)
			m.restoreDBArtifact(oldArtifact)
		}
		if tempDataDir != "" {
			_ = os.RemoveAll(tempDataDir)
		}
		if tempMetaDir != "" {
			_ = os.RemoveAll(tempMetaDir)
		}
	}()

	err = m.performInstallation(extractDir, desc, installedArtifact.InstallationReason, installedArtifact.ReverseDependencies)
	if err != nil {
		return err
	}

	// Execute post-update hook after successful update
	err = m.executePostUpdateHook(desc, installedArtifact.Version)
	if err != nil {
		return err
	}

	return nil
}

// VerifyArtifact verifies that an artifact exists and is valid.
func (m *ManagerImpl) VerifyArtifact(ctx context.Context, artifact *model.IndexArtifactDescriptor) error {
	filePath := filepath.Join(m.artifactCacheDir, fmt.Sprintf("%s_%s_%s_%s.gotya", artifact.Name, artifact.Version, artifact.OS, artifact.Arch))
	return m.verifier.VerifyArtifact(ctx, artifact, filePath)
}

// ReverseResolve returns the list of artifacts that depend on the given artifact recursively
func (m *ManagerImpl) ReverseResolve(_ context.Context, req model.ResolveRequest) (model.ResolvedArtifacts, error) {
	// Load the installed database
	err := m.loadInstalledDB()
	if err != nil {
		return model.ResolvedArtifacts{}, err
	}

	// Build reverse dependency graph and collect all dependent artifacts
	dependentArtifacts := m.collectReverseDependencies(req.Name)

	// Convert to ResolvedArtifacts format
	return m.convertToResolvedArtifacts(dependentArtifacts), nil
}

// GetOrphanedAutomaticArtifacts returns all installed artifacts that are automatic and have no reverse dependencies
func (m *ManagerImpl) GetOrphanedAutomaticArtifacts() ([]string, error) {
	// Load the installed database
	if err := m.loadInstalledDB(); err != nil {
		return nil, fmt.Errorf("failed to load installed database: %w", err)
	}
	var orphaned []string

	// Iterate through all installed artifacts
	for _, artifact := range m.installDB.GetInstalledArtifacts() {
		// Only consider installed artifacts (not missing)
		if artifact.Status != model.StatusInstalled {
			continue
		}

		// Only consider automatic installations
		if artifact.InstallationReason != model.InstallationReasonAutomatic {
			continue
		}

		// Check if it has no reverse dependencies
		if len(artifact.ReverseDependencies) == 0 {
			orphaned = append(orphaned, artifact.Name)
		}
	}

	return orphaned, nil
}

// GetInstalledArtifacts returns all installed artifacts
func (m *ManagerImpl) GetInstalledArtifacts() ([]*model.InstalledArtifact, error) {
	// Load the installed database
	if err := m.loadInstalledDB(); err != nil {
		return nil, fmt.Errorf("failed to load installed database: %w", err)
	}

	// Get all installed artifacts
	artifacts := m.installDB.GetInstalledArtifacts()

	// Filter to only return actually installed artifacts (not missing ones)
	var installed []*model.InstalledArtifact
	for _, artifact := range artifacts {
		if artifact.Status == model.StatusInstalled {
			installed = append(installed, artifact)
		}
	}

	return installed, nil
}

// validateUpdateRequest validates the update request parameters and checks if update is needed
func (m *ManagerImpl) validateUpdateRequest(newDescriptor *model.IndexArtifactDescriptor) (*model.InstalledArtifact, error) {
	// Check if the artifact is installed
	if !m.installDB.IsArtifactInstalled(newDescriptor.Name) {
		return nil, errors.Wrapf(errors.ErrArtifactNotFound, "artifact %s is not installed", newDescriptor.Name)
	}

	installedArtifact := m.installDB.FindArtifact(newDescriptor.Name)
	if installedArtifact == nil {
		return nil, errors.Wrapf(errors.ErrArtifactNotFound, "artifact %s not found in database", newDescriptor.Name)
	}

	// Validate that the new artifact name matches the installed artifact name
	if installedArtifact.Name != newDescriptor.Name {
		return nil, errors.Wrapf(errors.ErrValidation, "cannot update artifact %s with artifact %s: name mismatch", newDescriptor.Name, newDescriptor.Name)
	}

	// Check if this is actually an update (different version or URL)
	if installedArtifact.Version == newDescriptor.Version && installedArtifact.InstalledFrom == newDescriptor.URL {
		return nil, errors.Wrapf(errors.ErrValidation, "artifact %s is already at the latest version", newDescriptor.Name)
	}

	return installedArtifact, nil
}

// executePostUpdateHook executes the post-update hook for the artifact
func (m *ManagerImpl) executePostUpdateHook(newDescriptor *model.IndexArtifactDescriptor, oldVersion string) error {
	postUpdateContext := &HookContext{
		ArtifactName:    newDescriptor.Name,
		ArtifactVersion: newDescriptor.Version,
		Operation:       "update",
		MetaDir:         m.getArtifactMetaInstallPath(newDescriptor.Name),
		DataDir:         m.getArtifactDataInstallPath(newDescriptor.Name),
		OldVersion:      oldVersion,
	}

	// Parse metadata from newly installed artifact's metadata file for hook resolution
	metadataPath := filepath.Join(m.getArtifactMetaInstallPath(newDescriptor.Name), metadataFile)
	metadata, err := ParseMetadataFromPath(metadataPath)
	if err != nil {
		return err
	}
	postUpdateHookPath := m.resolveHookPath(m.getArtifactMetaInstallPath(newDescriptor.Name), "post-update", metadata)
	if postUpdateHookPath != "" {
		if err := m.hookExecutor.ExecuteHook(postUpdateHookPath, postUpdateContext); err != nil {
			return errors.Wrap(err, "Hook execution failed")
		}
	}

	return nil
}

// executePreUpdateHook executes the pre-update hook for the artifact
func (m *ManagerImpl) executePreUpdateHook(installedArtifact *model.InstalledArtifact, newDescriptor *model.IndexArtifactDescriptor) error {
	preUpdateContext := &HookContext{
		ArtifactName:    newDescriptor.Name,
		ArtifactVersion: newDescriptor.Version,
		Operation:       "update",
		MetaDir:         installedArtifact.ArtifactMetaDir,
		DataDir:         installedArtifact.ArtifactDataDir,
		OldVersion:      installedArtifact.Version,
	}

	// Parse metadata from installed artifact's metadata file for hook resolution
	metadataPath := filepath.Join(installedArtifact.ArtifactMetaDir, metadataFile)
	metadata, err := ParseMetadataFromPath(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to parse metadata for hook resolution: %w", err)
	}

	preUpdateHookPath := m.resolveHookPath(installedArtifact.ArtifactMetaDir, "pre-update", metadata)
	if preUpdateHookPath != "" {
		if err := m.hookExecutor.ExecuteHook(preUpdateHookPath, preUpdateContext); err != nil {
			return fmt.Errorf("pre-update hook failed: %w", err)
		}
	}

	return nil
}

// extractAndVerify extracts and verifies the artifact to a temp directory
func (m *ManagerImpl) extractAndVerify(ctx context.Context, desc *model.IndexArtifactDescriptor, localPath string, extractDir string) error {
	if err := m.archiveExtractor.ExtractAll(ctx, localPath, extractDir); err != nil {
		return errors.Wrap(err, "failed to extract artifact")
	}

	if err := m.verifier.VerifyArtifactFromPath(ctx, desc, extractDir); err != nil {
		return err
	}
	return nil
}

// handleExistingArtifact updates the installation reason for an existing artifact
// TODO: rework logic so that nothing has to be downloaded when the artifact is already installed but it can still be set to manaual
func (m *ManagerImpl) handleExistingArtifact(name string, reason model.InstallationReason) (bool, *model.InstalledArtifact, error) {
	existingArtifact := m.installDB.FindArtifact(name)
	if existingArtifact == nil {
		return false, nil, nil
	}
	switch existingArtifact.Status {
	case model.StatusInstalled:
		// Check if this is a transition from automatic to manual installation
		if existingArtifact.InstallationReason == model.InstallationReasonAutomatic && reason == model.InstallationReasonManual {
			// User is explicitly installing an artifact that was previously installed as dependency
			// Update it to manual installation
			existingArtifact.InstallationReason = reason
			m.installDB.AddArtifact(existingArtifact)
			if err := m.installDB.SaveDatabase(); err != nil {
				return false, existingArtifact, fmt.Errorf("failed to save database after updating installation reason: %w", err)
			}
		}
		return true, existingArtifact, nil // Successfully updated installation reason
	case model.StatusMissing:
		// This is a placeholder entry, we'll replace it with the real artifact
		// Save the reverse dependencies before removing the placeholder entry
		m.installDB.RemoveArtifact(existingArtifact.Name)
		return false, existingArtifact, nil
	default:
		return false, nil, fmt.Errorf("artifact %s has unknown status: %s: %w", existingArtifact.Name, existingArtifact.Status, errors.ErrValidation)
	}
}

// excutePreInstallHook runs the pre-update hook for the artifact
func (m *ManagerImpl) excutePreInstallHook(desc *model.IndexArtifactDescriptor, extractDir string) error {
	tempMetaDir := filepath.Join(extractDir, artifactMetaDir)
	// Execute pre-install hook from temp directory before moving files
	hookContext := &HookContext{
		ArtifactName:    desc.Name,
		ArtifactVersion: desc.Version,
		Operation:       "install",
		TempMetaDir:     tempMetaDir,
		FinalMetaDir:    m.getArtifactMetaInstallPath(desc.Name),
		FinalDataDir:    m.getArtifactDataInstallPath(desc.Name),
	}

	// Parse metadata from tmpExtractedPath metadata file for hook resolution
	metadataPath := filepath.Join(tempMetaDir, metadataFile)
	metadata, err := ParseMetadataFromPath(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to parse metadata for hook resolution: %w", err)
	}

	preInstallHookPath := m.resolveHookPath(tempMetaDir, "pre-install", metadata)
	if preInstallHookPath != "" {
		if err := m.hookExecutor.ExecuteHook(preInstallHookPath, hookContext); err != nil {
			return fmt.Errorf("pre-install hook failed: %w", err)
		}
	}
	return nil
}

// executePostInstallHook runs the post-install hook for the artifact
func (m *ManagerImpl) executePostInstallHook(desc *model.IndexArtifactDescriptor) error {
	// Execute post-install hook after successful installation
	metaPath := m.getArtifactMetaInstallPath(desc.Name)
	if metaPath != "" {
		postInstallContext := &HookContext{
			ArtifactName:    desc.Name,
			ArtifactVersion: desc.Version,
			Operation:       "install",
			MetaDir:         metaPath,
			DataDir:         m.getArtifactDataInstallPath(desc.Name),
		}

		// Parse metadata from installed metadata file for hook resolution
		metadataPath := filepath.Join(metaPath, metadataFile)
		metadata, err := ParseMetadataFromPath(metadataPath)
		if err != nil {
			return fmt.Errorf("failed to parse metadata for hook resolution: %w", err)
		}

		postInstallHookPath := m.resolveHookPath(metaPath, "post-install", metadata)
		if postInstallHookPath != "" {
			if err := m.hookExecutor.ExecuteHook(postInstallHookPath, postInstallContext); err != nil {
				return fmt.Errorf("post-install hook failed: %w", err)
			}
		}
	}
	return nil
}

// backupInstallationFiles moves the installation files to a new location
func (m *ManagerImpl) backupInstallationFiles(installedArtifact *model.InstalledArtifact) (string, string, error) {
	tempMetaDir, err := os.MkdirTemp(m.artifactMetaInstallDir, fmt.Sprintf(".gotya-update-meta-temp-%s-%s", installedArtifact.Name, installedArtifact.Version))
	if err != nil {
		return "", "", errors.Wrap(err, "failed to create temp meta dir")
	}
	var tempDataDir string
	if len(installedArtifact.DataFiles) > 0 {
		tempDataDir, err := os.MkdirTemp(m.artifactDataInstallDir, fmt.Sprintf(".gotya-update-data-temp-%s-%s", installedArtifact.Name, installedArtifact.Version))
		if err != nil {
			return "", tempMetaDir, errors.Wrap(err, "failed to create temp data dir")
		}
		if err := fsutil.Move(installedArtifact.ArtifactDataDir, tempDataDir); err != nil {
			return tempDataDir, tempMetaDir, errors.Wrapf(err, "failed to move artifact data from %s to %s", installedArtifact.ArtifactDataDir, tempDataDir)
		}
	}

	if err := fsutil.Move(installedArtifact.ArtifactMetaDir, tempMetaDir); err != nil {
		return tempDataDir, tempMetaDir, errors.Wrapf(err, "failed to move artifact meta from %s to %s", installedArtifact.ArtifactMetaDir, tempMetaDir)
	}

	return tempDataDir, tempMetaDir, nil
}

func (m *ManagerImpl) restoreInstallationFiles(tempDataDir, tempMetaDir string, installedArtifact *model.InstalledArtifact) error {
	_ = os.Remove(installedArtifact.ArtifactMetaDir)
	if err := fsutil.Move(filepath.Join(tempMetaDir, installedArtifact.Name), installedArtifact.ArtifactMetaDir); err != nil {
		return errors.Wrapf(err, "failed to move artifact meta from %s to %s", tempMetaDir, m.artifactMetaInstallDir)
	}
	if len(tempDataDir) > 0 {
		_ = os.Remove(installedArtifact.ArtifactDataDir)
		if err := fsutil.Move(filepath.Join(tempDataDir, installedArtifact.Name), installedArtifact.ArtifactDataDir); err != nil {
			return errors.Wrapf(err, "failed to move artifact data from %s to %s", tempDataDir, m.artifactDataInstallDir)
		}
	}
	return nil
}

func (m *ManagerImpl) copyDBArtifact(artifact *model.InstalledArtifact) *model.InstalledArtifact {
	return &model.InstalledArtifact{
		Name:                artifact.Name,
		Version:             artifact.Version,
		InstallationReason:  artifact.InstallationReason,
		ReverseDependencies: artifact.ReverseDependencies,
		Status:              artifact.Status,
		ArtifactMetaDir:     artifact.ArtifactMetaDir,
		ArtifactDataDir:     artifact.ArtifactDataDir,
	}
}

func (m *ManagerImpl) restoreDBArtifact(artifact *model.InstalledArtifact) {
	m.installDB.AddArtifact(artifact)
}

// loadInstalledDB loads or initializes the installed artifacts database.
func (m *ManagerImpl) loadInstalledDB() error {
	if err := m.installDB.LoadDatabase(); err != nil {
		return fmt.Errorf("failed to load installed database: %w", err)
	}
	return nil
}

func (m *ManagerImpl) getArtifactDataInstallPath(artifactName string) string {
	return filepath.Join(m.artifactDataInstallDir, artifactName)
}

func (m *ManagerImpl) getArtifactMetaInstallPath(artifactName string) string {
	return filepath.Join(m.artifactMetaInstallDir, artifactName)
}

// resolveHookPath resolves a hook type to its file path using metadata
func (m *ManagerImpl) resolveHookPath(metaDir string, hookType string, metadata *Metadata) string {
	if metadata != nil && metadata.Hooks != nil {
		if hookFile, exists := metadata.Hooks[hookType]; exists {
			return filepath.Join(metaDir, hookFile)
		}
	}
	return ""
}

func (m *ManagerImpl) findArtifactsDependingOn(targetArtifact string, result map[string]*model.InstalledArtifact) {
	// Iterate through all installed artifacts to find those that depend on the target
	for _, artifact := range m.installDB.GetInstalledArtifacts() {
		if artifact.Status != model.StatusInstalled {
			continue
		}

		// Check if this artifact depends on the target
		for _, depName := range artifact.ReverseDependencies {
			if depName == targetArtifact {
				// This artifact depends on the target, add it to results
				if _, exists := result[artifact.Name]; !exists {
					result[artifact.Name] = artifact
					// Recursively find artifacts that depend on this one
					m.findArtifactsDependingOn(artifact.Name, result)
				}
				break
			}
		}
	}
}

func (m *ManagerImpl) convertToResolvedArtifacts(artifacts map[string]*model.InstalledArtifact) model.ResolvedArtifacts {
	resolved := make([]model.ResolvedArtifact, 0, len(artifacts))

	for _, artifact := range artifacts {
		resolvedArtifact := model.ResolvedArtifact{
			Name:      artifact.Name,
			Version:   artifact.Version,
			OS:        m.os,
			Arch:      m.arch,
			SourceURL: nil,
			Checksum:  artifact.Checksum,
		}
		resolved = append(resolved, resolvedArtifact)
	}

	return model.ResolvedArtifacts{Artifacts: resolved}
}

func (m *ManagerImpl) collectReverseDependencies(targetArtifact string) map[string]*model.InstalledArtifact {
	result := make(map[string]*model.InstalledArtifact)

	// Find all artifacts that depend on the target artifact
	m.findArtifactsDependingOn(targetArtifact, result)
	return result
}
