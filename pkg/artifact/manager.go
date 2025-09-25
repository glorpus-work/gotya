package artifact

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/cperrin88/gotya/pkg/artifact/database"
	"github.com/cperrin88/gotya/pkg/model"
)

type ManagerImpl struct {
	os                     string
	arch                   string
	artifactCacheDir       string
	artifactDataInstallDir string
	artifactMetaInstallDir string
	installedDBPath        string
	verifier               *Verifier
}

func NewManager(os, arch, artifactCacheDir, artifactInstallDir, artifactMetaInstallDir, installedDBPath string) *ManagerImpl {
	return &ManagerImpl{
		os:                     os,
		arch:                   arch,
		artifactCacheDir:       artifactCacheDir,
		artifactDataInstallDir: artifactInstallDir,
		artifactMetaInstallDir: artifactMetaInstallDir,
		installedDBPath:        installedDBPath,
		verifier:               NewVerifier(),
	}
}

// InstallArtifact installs (verifies/stages) an artifact from a local file path, replacing the previous network-based install.
func (m ManagerImpl) InstallArtifact(ctx context.Context, desc *model.IndexArtifactDescriptor, localPath string) (err error) {
	// Input validation
	if desc == nil {
		return fmt.Errorf("artifact descriptor cannot be nil")
	}
	if desc.Name == "" {
		return fmt.Errorf("artifact name cannot be empty")
	}
	if localPath == "" {
		return fmt.Errorf("local path cannot be empty")
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
	db := database.NewInstalledDatabase()
	if err = db.LoadDatabase(m.installedDBPath); err != nil {
		return fmt.Errorf("failed to load installed database: %w", err)
	}

	// Check if the artifact is already installed
	if db.IsArtifactInstalled(desc.Name) {
		return fmt.Errorf("artifact %s is already installed", desc.Name)
	}

	extractDir, err := os.MkdirTemp("", fmt.Sprintf("gotya-extract-%s", desc.Name))
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(extractDir)

	if err = m.verifier.extractArtifact(ctx, localPath, extractDir); err != nil {
		return fmt.Errorf("failed to extract artifact: %w", err)
	}

	//TODO pre-install hooks

	// Install the artifact files
	if err = m.installArtifactFiles(desc.Name, extractDir); err != nil {
		return fmt.Errorf("failed to install artifact files: %w", err)
	}
	installed = true // Mark that we've installed files that might need cleanup

	// Add the installed artifact to the database
	err = m.addArtifactToDatabase(db, desc)
	if err != nil {
		return fmt.Errorf("failed to update artifact database: %w", err)
	}

	//TODO post-install hooks

	return nil
}

func (m ManagerImpl) UninstallArtifact(ctx context.Context, artifactName string, purge bool) error {
	// Input validation
	if artifactName == "" {
		return fmt.Errorf("artifact name cannot be empty")
	}

	// Load the installed database
	db := database.NewInstalledDatabase()
	if err := db.LoadDatabase(m.installedDBPath); err != nil {
		return fmt.Errorf("failed to load installed database: %w", err)
	}

	// Check if the artifact is installed
	if !db.IsArtifactInstalled(artifactName) {
		return fmt.Errorf("artifact %s is not installed", artifactName)
	}

	artifact := db.FindArtifact(artifactName)
	if artifact == nil {
		return fmt.Errorf("artifact %s not found in database", artifactName)
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
func (m ManagerImpl) UpdateArtifact(ctx context.Context, artifactName string, newArtifactPath string, newDescriptor *model.IndexArtifactDescriptor) error {
	// Input validation
	if artifactName == "" {
		return fmt.Errorf("artifact name cannot be empty")
	}
	if newArtifactPath == "" {
		return fmt.Errorf("new artifact path cannot be empty")
	}
	if newDescriptor == nil {
		return fmt.Errorf("new artifact descriptor cannot be nil")
	}

	// Load the installed database
	db := database.NewInstalledDatabase()
	if err := db.LoadDatabase(m.installedDBPath); err != nil {
		return fmt.Errorf("failed to load installed database: %w", err)
	}

	// Check if the artifact is installed
	if !db.IsArtifactInstalled(artifactName) {
		return fmt.Errorf("artifact %s is not installed", artifactName)
	}

	installedArtifact := db.FindArtifact(artifactName)
	if installedArtifact == nil {
		return fmt.Errorf("artifact %s not found in database", artifactName)
	}

	// Verify the new artifact before proceeding
	if err := m.verifier.VerifyArtifact(ctx, newDescriptor, newArtifactPath); err != nil {
		return fmt.Errorf("failed to verify new artifact: %w", err)
	}

	// Validate that the new artifact name matches the installed artifact name
	if installedArtifact.Name != newDescriptor.Name {
		return fmt.Errorf("cannot update artifact %s with artifact %s: name mismatch", artifactName, newDescriptor.Name)
	}

	// Check if this is actually an update (different version or URL)
	if installedArtifact.Version == newDescriptor.Version && installedArtifact.InstalledFrom == newDescriptor.URL {
		return fmt.Errorf("artifact %s is already at the latest version", artifactName)
	}

	// Step 1: Uninstall the old version (with purge=true for clean slate)
	if err := m.uninstallWithPurge(ctx, db, installedArtifact); err != nil {
		return fmt.Errorf("failed to uninstall old version of %s: %w", artifactName, err)
	}

	// Step 2: Install the new version
	// Note: We need to reload the database since uninstallWithPurge modifies it
	db = database.NewInstalledDatabase()
	if err := db.LoadDatabase(m.installedDBPath); err != nil {
		// If we can't reload the database, the uninstall succeeded but we can't install
		// This leaves the user in a bad state, but we should report the error
		return fmt.Errorf("failed to reload database after uninstall: %w", err)
	}

	if err := m.InstallArtifact(ctx, newDescriptor, newArtifactPath); err != nil {
		// Installation failed - we should try to rollback by reinstalling the old version
		// However, we don't have the old artifact file anymore, so we can only log a warning
		log.Printf("Warning: Failed to install new version of %s: %v. The old version has been uninstalled but cannot be restored.", artifactName, err)
		return fmt.Errorf("failed to install new version of %s: %w", artifactName, err)
	}

	return nil
}

func (m ManagerImpl) VerifyArtifact(ctx context.Context, artifact *model.IndexArtifactDescriptor) error {
	filePath := filepath.Join(m.artifactCacheDir, fmt.Sprintf("%s_%s_%s_%s.gotya", artifact.Name, artifact.Version, artifact.OS, artifact.Arch))
	return m.verifier.VerifyArtifact(ctx, artifact, filePath)
}

func (m ManagerImpl) getArtifactMetaInstallPath(artifactName string) string {
	return filepath.Join(m.artifactMetaInstallDir, artifactName)
}

func (m ManagerImpl) getArtifactDataInstallPath(artifactName string) string {
	return filepath.Join(m.artifactDataInstallDir, artifactName)
}

// installRollback cleans up any partially installed files in case of an error
func (m ManagerImpl) installRollback(artifactName string) {
	metaPath := m.getArtifactMetaInstallPath(artifactName)
	_ = os.RemoveAll(metaPath)

	dataPath := m.getArtifactDataInstallPath(artifactName)
	_ = os.RemoveAll(dataPath)
}
