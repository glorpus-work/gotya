package artifact

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cperrin88/gotya/internal/logger"
	"github.com/cperrin88/gotya/pkg/archive"
	"github.com/cperrin88/gotya/pkg/artifact/database"
	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/model"
)

// ManagerImpl is the default implementation of the Manager interface for artifact operations.
// It handles installation, uninstallation, updates, and verification of artifacts.
type ManagerImpl struct {
	os                     string
	arch                   string
	artifactCacheDir       string
	artifactDataInstallDir string
	artifactMetaInstallDir string
	installedDBPath        string
	verifier               *Verifier
	archiveManager         *archive.Manager
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
		installedDBPath:        installedDBPath,
		verifier:               NewVerifier(),
		archiveManager:         archive.NewManager(),
	}
}

// loadInstalledDB loads or initializes the installed artifacts database.
func (m ManagerImpl) loadInstalledDB() (*database.InstalledManagerImpl, error) {
	db := database.NewInstalledDatabase()
	if err := db.LoadDatabase(m.installedDBPath); err != nil {
		return nil, fmt.Errorf("failed to load installed database: %w", err)
	}
	return db, nil
}

// InstallArtifact installs (verifies/stages) an artifact from a local file path, replacing the previous network-based install.
func (m ManagerImpl) InstallArtifact(ctx context.Context, desc *model.IndexArtifactDescriptor, localPath string, reason model.InstallationReason) (err error) {
	// Input validation
	if desc == nil {
		return fmt.Errorf("artifact descriptor cannot be nil: %w", errors.ErrValidation)
	}
	if desc.Name == "" {
		return fmt.Errorf("artifact name cannot be empty: %w", errors.ErrValidation)
	}
	if localPath == "" {
		return fmt.Errorf("local path cannot be empty: %w", errors.ErrValidation)
	}

	// Set up rollback in case of failure
	var installed bool
	defer func() {
		if err != nil && installed {
			// If we installed files but then failed, clean them up
			m.installRollback(desc.Name)
		}
	}()

	if err = m.verifier.VerifyArtifact(ctx, desc, localPath); err != nil {
		return err
	}

	// Load or create the installed database
	db, err := m.loadInstalledDB()
	if err != nil {
		return err
	}

	// Check if the artifact is already installed
	existingArtifact := db.FindArtifact(desc.Name)
	var existingReverseDeps []string
	if existingArtifact != nil {
		switch existingArtifact.Status {
		case model.StatusInstalled:
			// Check if this is a transition from automatic to manual installation
			if existingArtifact.InstallationReason == model.InstallationReasonAutomatic && reason == model.InstallationReasonManual {
				// User is explicitly installing an artifact that was previously installed as dependency
				// Update it to manual installation
				existingArtifact.InstallationReason = model.InstallationReasonManual
				existingArtifact.InstalledAt = time.Now() // Update installation time
				db.AddArtifact(existingArtifact)
				if err := db.SaveDatabase(m.installedDBPath); err != nil {
					return fmt.Errorf("failed to save database after updating installation reason: %w", err)
				}
				return nil // Successfully updated installation reason
			}
			// Never downgrade from manual to automatic
			if existingArtifact.InstallationReason == model.InstallationReasonManual {
				reason = model.InstallationReasonManual
			}
		case model.StatusMissing:
			// This is a dummy entry, we'll replace it with the real artifact
			// Save the reverse dependencies before removing the dummy entry
			existingReverseDeps = existingArtifact.ReverseDependencies
			db.RemoveArtifact(desc.Name)
		default:
			return fmt.Errorf("artifact %s has unknown status: %s: %w", desc.Name, existingArtifact.Status, errors.ErrValidation)
		}
	}

	// Extract and install files
	if installed, err = m.extractAndInstall(ctx, desc, localPath); err != nil {
		return err
	}

	// Add the installed artifact to the database
	err = m.addArtifactToDatabase(db, desc, existingReverseDeps, reason)
	if err != nil {
		return fmt.Errorf("failed to update artifact database: %w", err)
	}

	// TODO: post-install hooks

	return nil
}

// UninstallArtifact removes an installed artifact from the system.
func (m ManagerImpl) UninstallArtifact(ctx context.Context, artifactName string, purge bool) error {
	// Input validation
	if artifactName == "" {
		return fmt.Errorf("artifact name cannot be empty: %w", errors.ErrValidation)
	}

	// Load the installed database
	db := database.NewInstalledDatabase()
	if err := db.LoadDatabase(m.installedDBPath); err != nil {
		return fmt.Errorf("failed to load installed database: %w", err)
	}

	// Check if the artifact is installed
	if !db.IsArtifactInstalled(artifactName) {
		return fmt.Errorf("artifact %s is not installed: %w", artifactName, errors.ErrArtifactNotFound)
	}

	artifact := db.FindArtifact(artifactName)
	if artifact == nil {
		return fmt.Errorf("artifact %s not found in database: %w", artifactName, errors.ErrArtifactNotFound)
	}

	// Handle purge mode
	if purge {
		return m.uninstallWithPurge(ctx, db, artifact)
	}

	return m.uninstallSelectively(ctx, db, artifact)
}

// UpdateArtifact updates an installed artifact by replacing it with a new version.
// This method uses the simple approach: uninstall the old version, then install the new version.
// If the installation fails, the old version remains uninstalled.
func (m ManagerImpl) UpdateArtifact(ctx context.Context, newArtifactPath string, newDescriptor *model.IndexArtifactDescriptor) error {
	// Input validation
	if newArtifactPath == "" {
		return fmt.Errorf("new artifact path cannot be empty: %w", errors.ErrValidation)
	}
	if newDescriptor == nil {
		return fmt.Errorf("new artifact descriptor cannot be nil: %w", errors.ErrValidation)
	}
	if newDescriptor.Name == "" {
		return fmt.Errorf("artifact name cannot be empty: %w", errors.ErrValidation)
	}

	// Load the installed database
	db := database.NewInstalledDatabase()
	if err := db.LoadDatabase(m.installedDBPath); err != nil {
		return fmt.Errorf("failed to load installed database: %w", err)
	}

	// Check if the artifact is installed
	if !db.IsArtifactInstalled(newDescriptor.Name) {
		return fmt.Errorf("artifact %s is not installed: %w", newDescriptor.Name, errors.ErrArtifactNotFound)
	}

	installedArtifact := db.FindArtifact(newDescriptor.Name)
	if installedArtifact == nil {
		return fmt.Errorf("artifact %s not found in database: %w", newDescriptor.Name, errors.ErrArtifactNotFound)
	}

	// Verify the new artifact before proceeding
	if err := m.verifier.VerifyArtifact(ctx, newDescriptor, newArtifactPath); err != nil {
		return fmt.Errorf("failed to verify new artifact: %w", err)
	}

	// Validate that the new artifact name matches the installed artifact name
	if installedArtifact.Name != newDescriptor.Name {
		return fmt.Errorf("cannot update artifact %s with artifact %s: name mismatch: %w", newDescriptor.Name, newDescriptor.Name, errors.ErrValidation)
	}

	// Check if this is actually an update (different version or URL)
	// But allow automatic -> manual upgrades even with same version/URL
	if installedArtifact.Version == newDescriptor.Version && installedArtifact.InstalledFrom == newDescriptor.URL {
		// If trying to upgrade from automatic to manual, allow it
		if installedArtifact.InstallationReason == model.InstallationReasonAutomatic {
			// This is an automatic -> manual upgrade, proceed with installation
			logger.Debug("Upgrading automatic installation to manual", logger.Fields{"artifact": newDescriptor.Name})
		} else {
			return fmt.Errorf("artifact %s is already at the latest version: %w", newDescriptor.Name, errors.ErrValidation)
		}
	}

	// Check installation reason transitions
	// Only prevent downgrades from manual to automatic, allow updates within the same reason or upgrades
	if installedArtifact.InstallationReason == model.InstallationReasonManual {
		// Manual installations can only be updated (version/URL changes are allowed)
		// The installation reason stays manual
		logger.Debug("Updating manually installed artifact", logger.Fields{"artifact": newDescriptor.Name})
	}

	// Step 1: Uninstall the old version (with purge=true for clean slate)
	if err := m.uninstallWithPurge(ctx, db, installedArtifact); err != nil {
		return fmt.Errorf("failed to uninstall old version of %s: %w", newDescriptor.Name, err)
	}

	// Step 2: Install the new version
	// Note: We need to reload the database since uninstallWithPurge modifies it
	db = database.NewInstalledDatabase()
	if err := db.LoadDatabase(m.installedDBPath); err != nil {
		// If we can't reload the database, the uninstall succeeded but we can't install
		// This leaves the user in a bad state, but we should report the error
		return fmt.Errorf("failed to reload database after uninstall: %w", err)
	}

	if err := m.InstallArtifact(ctx, newDescriptor, newArtifactPath, model.InstallationReasonManual); err != nil {
		// Installation failed - we should try to rollback by reinstalling the old version
		// However, we don't have the old artifact file anymore, so we can only log a warning
		logger.Warn("Failed to install new version - old version uninstalled but cannot be restored", logger.Fields{"artifact": newDescriptor.Name, "error": err})
		return fmt.Errorf("failed to install new version of %s: %w", newDescriptor.Name, err)
	}

	return nil
}

// VerifyArtifact verifies that an artifact exists and is valid.
func (m ManagerImpl) VerifyArtifact(ctx context.Context, artifact *model.IndexArtifactDescriptor) error {
	filePath := filepath.Join(m.artifactCacheDir, fmt.Sprintf("%s_%s_%s_%s.gotya", artifact.Name, artifact.Version, artifact.OS, artifact.Arch))
	return m.verifier.VerifyArtifact(ctx, artifact, filePath)
}

// ReverseResolve returns the list of artifacts that depend on the given artifact recursively
func (m ManagerImpl) ReverseResolve(_ context.Context, req model.ResolveRequest) (model.ResolvedArtifacts, error) {
	// Load the installed database
	db := database.NewInstalledDatabase()
	if err := db.LoadDatabase(m.installedDBPath); err != nil {
		return model.ResolvedArtifacts{}, fmt.Errorf("failed to load installed database: %w", err)
	}

	// Build reverse dependency graph and collect all dependent artifacts
	dependentArtifacts := m.collectReverseDependencies(db, req.Name)

	// Convert to ResolvedArtifacts format
	return m.convertToResolvedArtifacts(dependentArtifacts), nil
}

// GetOrphanedAutomaticArtifacts returns all installed artifacts that are automatic and have no reverse dependencies
func (m ManagerImpl) GetOrphanedAutomaticArtifacts() ([]string, error) {
	// Load the installed database
	db := database.NewInstalledDatabase()
	if err := db.LoadDatabase(m.installedDBPath); err != nil {
		return nil, fmt.Errorf("failed to load installed database: %w", err)
	}
	var orphaned []string

	// Iterate through all installed artifacts
	for _, artifact := range db.GetInstalledArtifacts() {
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
func (m ManagerImpl) GetInstalledArtifacts() ([]*model.InstalledArtifact, error) {
	// Load the installed database
	db := database.NewInstalledDatabase()
	if err := db.LoadDatabase(m.installedDBPath); err != nil {
		return nil, fmt.Errorf("failed to load installed database: %w", err)
	}

	// Get all installed artifacts
	artifacts := db.GetInstalledArtifacts()

	// Filter to only return actually installed artifacts (not missing ones)
	var installed []*model.InstalledArtifact
	for _, artifact := range artifacts {
		if artifact.Status == model.StatusInstalled {
			installed = append(installed, artifact)
		}
	}

	return installed, nil
}

func (m ManagerImpl) getArtifactDataInstallPath(artifactName string) string {
	return filepath.Join(m.artifactDataInstallDir, artifactName)
}

func (m ManagerImpl) getArtifactMetaInstallPath(artifactName string) string {
	return filepath.Join(m.artifactMetaInstallDir, artifactName)
}

func (m ManagerImpl) findArtifactsDependingOn(db *database.InstalledManagerImpl, targetArtifact string, result map[string]*model.InstalledArtifact) {
	// Iterate through all installed artifacts to find those that depend on the target
	for _, artifact := range db.GetInstalledArtifacts() {
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
					m.findArtifactsDependingOn(db, artifact.Name, result)
				}
				break
			}
		}
	}
}

func (m ManagerImpl) convertToResolvedArtifacts(artifacts map[string]*model.InstalledArtifact) model.ResolvedArtifacts {
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

func (m ManagerImpl) extractAndInstall(ctx context.Context, desc *model.IndexArtifactDescriptor, localPath string) (bool, error) {
	extractDir, err := os.MkdirTemp("", fmt.Sprintf("gotya-extract-%s", desc.Name))
	if err != nil {
		return false, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(extractDir) }()

	if err := m.archiveManager.ExtractAll(ctx, localPath, extractDir); err != nil {
		return false, fmt.Errorf("failed to extract artifact: %w", err)
	}

	// TODO: pre-install hooks

	if err := m.installArtifactFiles(desc.Name, extractDir); err != nil {
		return false, fmt.Errorf("failed to install artifact files: %w", err)
	}
	return true, nil
}

func (m ManagerImpl) collectReverseDependencies(db *database.InstalledManagerImpl, targetArtifact string) map[string]*model.InstalledArtifact {
	result := make(map[string]*model.InstalledArtifact)

	// Find all artifacts that depend on the target artifact
	m.findArtifactsDependingOn(db, targetArtifact, result)
	return result
}
