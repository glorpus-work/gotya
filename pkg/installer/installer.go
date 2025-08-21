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
	hookManager *hook.Manager
}

// New creates a new Installer instance
func New(cfg *config.Config, repoManager repository.Manager, hookManager *hook.Manager) *Installer {
	return &Installer{
		config:      cfg,
		repoManager: repoManager,
		hookManager: hookManager,
	}
}

// InstallPackage installs a package with the given name
func (i *Installer) InstallPackage(packageName string, force, skipDeps bool) error {
	// Find the package in repositories
	pkg, _, err := i.repoManager.FindPackage(packageName)
	if err != nil {
		return fmt.Errorf("failed to find package %s: %w", packageName, err)
	}

	// Check if already installed
	installedDB, err := pkgpkg.LoadInstalledDB(i.config.GetDatabasePath())
	if err != nil {
		return fmt.Errorf("failed to load installed packages database: %w", err)
	}

	if installedDB.IsInstalled(packageName) && !force {
		return fmt.Errorf("package %s is already installed (use --force to reinstall)", packageName)
	}

	// Run pre-install hooks
	if err := i.runHooks("pre-install", packageName, pkg); err != nil {
		return fmt.Errorf("pre-install hook failed: %w", err)
	}

	// Install the package
	if err := i.installPackageFiles(pkg); err != nil {
		return fmt.Errorf("failed to install package files: %w", err)
	}

	// Update installed packages database
	if err := installedDB.Add(packageName, pkg.Version); err != nil {
		return fmt.Errorf("failed to update installed packages database: %w", err)
	}

	// Run post-install hooks
	if err := i.runHooks("post-install", packageName, pkg); err != nil {
		return fmt.Errorf("post-install hook failed: %w", err)
	}

	logrus.Infof("Successfully installed %s %s", packageName, pkg.Version)
	return nil
}

// UpdatePackage updates a package to the latest version
func (i *Installer) UpdatePackage(packageName string) (bool, error) {
	// Find the latest version of the package
	pkg, _, err := i.repoManager.FindPackage(packageName)
	if err != nil {
		return false, fmt.Errorf("failed to find package %s: %w", packageName, err)
	}

	// Check if installed
	installedDB, err := pkgpkg.LoadInstalledDB(i.config.GetDatabasePath())
	if err != nil {
		return false, fmt.Errorf("failed to load installed packages database: %w", err)
	}

	if !installedDB.IsInstalled(packageName) {
		return false, fmt.Errorf("package %s is not installed", packageName)
	}

	// Check if update is needed
	installedVersion := installedDB.GetVersion(packageName)
	if installedVersion == pkg.Version {
		logrus.Infof("Package %s is already up to date (%s)", packageName, installedVersion)
		return false, nil
	}

	// Run pre-update hooks
	if err := i.runHooks("pre-update", packageName, pkg); err != nil {
		return false, fmt.Errorf("pre-update hook failed: %w", err)
	}

	// Update the package
	if err := i.installPackageFiles(pkg); err != nil {
		return false, fmt.Errorf("failed to update package files: %w", err)
	}

	// Update installed packages database
	if err := installedDB.Add(packageName, pkg.Version); err != nil {
		return false, fmt.Errorf("failed to update installed packages database: %w", err)
	}

	// Run post-update hooks
	if err := i.runHooks("post-update", packageName, pkg); err != nil {
		return false, fmt.Errorf("post-update hook failed: %w", err)
	}

	logrus.Infof("Successfully updated %s from %s to %s", packageName, installedVersion, pkg.Version)
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

	ctx := map[string]interface{}{
		"package": map[string]interface{}{
			"name":    packageName,
			"version": pkg.Version,
		},
		"config": map[string]interface{}{
			"install_dir": i.config.Settings.InstallDir,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	return i.hookManager.Execute(event, ctx)
}
