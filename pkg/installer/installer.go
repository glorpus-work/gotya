package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cperrin88/gotya/pkg/config"
	"github.com/cperrin88/gotya/pkg/hook"
	pkgpkg "github.com/cperrin88/gotya/pkg/package"
	"github.com/cperrin88/gotya/pkg/repository"
	"github.com/sirupsen/logrus"
)

// Installer handles package installation and updates
type Installer struct {
	config      *config.Config
	repoManager repository.Manager
	hookManager hook.HookManager
}

// New creates a new Installer instance
func New(cfg *config.Config, repoManager repository.Manager, hookManager hook.HookManager) *Installer {
	return &Installer{
		config:      cfg,
		repoManager: repoManager,
		hookManager: hookManager,
	}
}

// InstallPackage installs a package with the given name
func (i *Installer) InstallPackage(packageName string, force, skipDeps bool) error {
	// Find the package in repositories
	pkg, err := i.repoManager.FindPackage(packageName)
	if err != nil {
		return fmt.Errorf("failed to find package %s: %w", packageName, err)
	}

	// Check if already installed
	installedDB, err := pkgpkg.LoadInstalledDatabase(i.config.GetDatabasePath())
	if err != nil {
		return fmt.Errorf("failed to load installed packages database: %w", err)
	}

	if installedPkg := installedDB.FindPackage(packageName); installedPkg != nil && !force {
		return fmt.Errorf("package %s is already installed (use --force to reinstall)", packageName)
	}

	// Run pre-install hooks
	if err := i.runHooks("pre-install", packageName, pkg); err != nil {
		return fmt.Errorf("pre-install hook failed: %w", err)
	}

	// Install package files
	if err := i.installPackageFiles(pkg); err != nil {
		return fmt.Errorf("failed to install package files: %w", err)
	}

	// Update installed packages database
	installedPkg := pkgpkg.InstalledPackage{
		Name:          pkg.Name,
		Version:       pkg.Version,
		Description:   pkg.Description,
		InstalledAt:   time.Now(),
		InstalledFrom: pkg.URL,
	}
	installedDB.AddPackage(installedPkg)

	// Run post-install hooks
	if err := i.runHooks("post-install", packageName, pkg); err != nil {
		// Don't fail the installation if post-install hooks fail, just log the error
		logrus.WithError(err).Error("Post-install hook failed")
	}

	logrus.Infof("Successfully installed %s %s", packageName, pkg.Version)
	return nil
}

// UpdatePackage updates a package to the latest version
func (i *Installer) UpdatePackage(packageName string) (bool, error) {
	// Find the latest version of the package
	pkg, err := i.repoManager.FindPackage(packageName)
	if err != nil {
		return false, fmt.Errorf("failed to find package %s: %w", packageName, err)
	}

	// Check if the package is installed
	installedDB, err := pkgpkg.LoadInstalledDatabase(i.config.GetDatabasePath())
	if err != nil {
		return false, fmt.Errorf("failed to load installed packages database: %w", err)
	}

	// Check if package is installed and get its version
	installedPkg := installedDB.FindPackage(packageName)
	if installedPkg == nil {
		return false, fmt.Errorf("package %s is not installed", packageName)
	}
	installedVersion := installedPkg.Version

	// Compare versions
	if pkg.Version == installedVersion {
		return false, nil // Already up to date
	}

	logrus.Infof("Updating %s from %s to %s", packageName, installedVersion, pkg.Version)

	// Run pre-update hooks (using pre-install for now)
	if err := i.runHooks("pre-install", packageName, pkg); err != nil {
		return false, fmt.Errorf("pre-update hook failed: %w", err)
	}

	// Install the new version
	if err := i.installPackageFiles(pkg); err != nil {
		return false, fmt.Errorf("failed to install package files: %w", err)
	}

	// Update the installed packages database
	updatedPkg := pkgpkg.InstalledPackage{
		Name:          pkg.Name,
		Version:       pkg.Version,
		Description:   pkg.Description,
		InstalledAt:   time.Now(),
		InstalledFrom: pkg.URL,
	}
	installedDB.AddPackage(updatedPkg)

	// Run post-update hooks (using post-install for now)
	if err := i.runHooks("post-install", packageName, pkg); err != nil {
		// Don't fail the update if post-update hooks fail, just log the error
		logrus.WithError(err).Error("Post-update hook failed")
	}

	logrus.Infof("Successfully updated %s to %s", packageName, pkg.Version)
	return true, nil
}

// installPackageFiles installs the actual package files
func (i *Installer) installPackageFiles(pkg *repository.Package) error {
	// Create target directories
	targetDir := filepath.Join(i.config.Settings.InstallDir, pkg.Name)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Download and extract package
	// This is a simplified example - in a real implementation, you would:
	// 1. Download the package archive
	// 2. Verify checksum
	// 3. Extract to target directory
	// 4. Handle file conflicts

	return nil
}

// runHooks executes hooks for a specific event
func (i *Installer) runHooks(event, packageName string, pkg *repository.Package) error {
	if i.hookManager == nil {
		return nil // No hook manager configured
	}

	// Create hook context
	hookCtx := hook.HookContext{
		PackageName:    pkg.Name,
		PackageVersion: pkg.Version,
		PackagePath:    pkg.URL, // Using URL as the package path
		InstallPath:    filepath.Join(i.config.Settings.InstallDir, pkg.Name),
		Vars: map[string]interface{}{
			"config": map[string]interface{}{
				"install_dir": i.config.Settings.InstallDir,
			},
		},
	}

	// Convert event string to HookType
	var hookType hook.HookType
	switch event {
	case "pre-install", "pre-update":
		hookType = hook.PreInstall
	case "post-install", "post-update":
		hookType = hook.PostInstall
	default:
		return fmt.Errorf("unsupported hook event: %s", event)
	}

	// Execute the hook
	return i.hookManager.Execute(hookType, hookCtx)
}
