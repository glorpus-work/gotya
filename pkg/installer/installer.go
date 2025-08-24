package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cperrin88/gotya/pkg/config"
	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/fsutil"
	"github.com/cperrin88/gotya/pkg/hook"
	"github.com/cperrin88/gotya/pkg/logger"
	pkgpkg "github.com/cperrin88/gotya/pkg/package"
	"github.com/cperrin88/gotya/pkg/repository"
)

// Installer handles package installation and updates.
type Installer struct {
	config       *config.Config
	repoManager  repository.RepositoryManager
	hookManager  hook.HookManager
	cacheDir     string
	installedDir string
	metaDir      string
}

// New creates a new Installer instance.
func New(cfg *config.Config, repoManager repository.RepositoryManager, hookManager hook.HookManager) (*Installer, error) {
	// Ensure all required directories exist
	if err := fsutil.EnsureDirs(); err != nil {
		return nil, errors.Wrap(err, "failed to initialize directories")
	}

	// Get platform-specific directories
	cacheDir, err := fsutil.GetPackageCacheDir()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cache directory")
	}

	installedDir, err := fsutil.GetInstalledDir()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get installed packages directory")
	}

	metaDir, err := fsutil.GetMetaDir()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get metadata directory")
	}

	return &Installer{
		config:       cfg,
		repoManager:  repoManager,
		hookManager:  hookManager,
		cacheDir:     cacheDir,
		installedDir: installedDir,
		metaDir:      metaDir,
	}, nil
}

// InstallPackage installs a package with the given name.
func (i *Installer) InstallPackage(packageName string, force, skipDeps bool) error {
	// Find the package in repositories
	pkg, err := i.repoManager.FindPackage(packageName)
	if err != nil {
		return errors.Wrapf(err, "failed to find package %s", packageName)
	}

	// Check if already installed
	installedDB, err := pkgpkg.LoadInstalledDatabase(i.config.GetDatabasePath())
	if err != nil {
		return errors.Wrap(err, "failed to load installed packages database")
	}

	if installedPkg := installedDB.FindPackage(packageName); installedPkg != nil && !force {
		return fmt.Errorf("package %s is already installed (use --force to reinstall)", packageName)
	}

	// Handle dependencies if not skipped
	if !skipDeps && len(pkg.Dependencies) > 0 {
		logger.Infof("Resolving dependencies for %s...", packageName)
		for _, dep := range pkg.Dependencies {
			// Check if dependency is already installed
			if installedDB.FindPackage(dep) == nil {
				logger.Debugf("Installing dependency: %s", dep)
				if err := i.InstallPackage(dep, false, false); err != nil {
					return fmt.Errorf("failed to install dependency: %w", err)
				}
			}
		}
	}

	// Run pre-install hooks
	if err := i.runHooks("pre-install", packageName, pkg); err != nil {
		return errors.Wrap(err, "pre-install hook failed")
	}

	// Install package files
	if err := i.installPackageFiles(pkg); err != nil {
		return errors.Wrap(err, "failed to install package files")
	}

	// Update installed packages database
	installedPkg := &pkgpkg.InstalledPackage{
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
		logger.Errorf("Post-install hook failed: %v", err)
	}

	logger.Infof("Successfully installed %s %s", packageName, pkg.Version)
	return nil
}

// UpdatePackage updates a package to the latest version.
func (i *Installer) UpdatePackage(packageName string) (bool, error) {
	// Find the latest version of the package
	pkg, err := i.repoManager.FindPackage(packageName)
	if err != nil {
		return false, errors.Wrap(err, "failed to find package")
	}

	// Check if the package is installed
	installedDB, err := pkgpkg.LoadInstalledDatabase(i.config.GetDatabasePath())
	if err != nil {
		return false, errors.Wrap(err, "failed to load installed packages database")
	}

	// Check if package is installed and get its version
	installedPkg := installedDB.FindPackage(packageName)
	if installedPkg == nil {
		return false, fmt.Errorf("package %s is not installed", packageName)
	}
	installedVersion := installedPkg.Version

	// Compare versions
	if pkg.Version == installedVersion {
		return false, fmt.Errorf("package %s is already up to date", packageName)
	}

	logger.Debugf("Updating package %s from version %s to %s", packageName, installedVersion, pkg.Version)

	// Run pre-update hooks (using pre-install for now)
	if err := i.runHooks("pre-install", packageName, pkg); err != nil {
		return false, errors.Wrap(err, "pre-update hook failed")
	}

	// Install the new version
	if err := i.installPackageFiles(pkg); err != nil {
		return false, errors.Wrap(err, "failed to install package files")
	}

	// Update the installed packages database
	updatedPkg := &pkgpkg.InstalledPackage{
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
		logger.Errorf("Post-update hook failed: %v", err)
	}

	logger.Infof("Successfully updated %s to %s", packageName, pkg.Version)
	return true, nil
}

// getPackageInstallDir returns the installation directory for a package
func (i *Installer) getPackageInstallDir(pkgName string) string {
	return filepath.Join(i.installedDir, pkgName)
}

// getPackageMetaDir returns the metadata directory for a package
func (i *Installer) getPackageMetaDir(pkgName string) string {
	return filepath.Join(i.metaDir, pkgName)
}

// getPackageCachePath returns the cache path for a package archive
func (i *Installer) getPackageCachePath(pkg *repository.Package) string {
	// Use the filename from the URL if available, otherwise create a default name
	if pkg.URL != "" {
		_, filename := filepath.Split(pkg.URL)
		if filename != "" {
			return filepath.Join(i.cacheDir, filename)
		}
	}
	// Fallback to a default naming scheme if URL is not available
	return filepath.Join(i.cacheDir, fmt.Sprintf("%s_%s.tar.gz", pkg.Name, pkg.Version))
}

// installPackageFiles installs the actual package files
func (i *Installer) installPackageFiles(pkg *repository.Package) error {
	// Create target directories
	targetDir := i.getPackageInstallDir(pkg.Name)
	metaDir := i.getPackageMetaDir(pkg.Name)

	logger.Debugf("Creating target directory: %s", targetDir)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return errors.Wrapf(err, "failed to create target directory: %s", targetDir)
	}

	logger.Debugf("Creating metadata directory: %s", metaDir)
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		return errors.Wrapf(err, "failed to create metadata directory: %s", metaDir)
	}

	// TODO: Implement package download and extraction
	// 1. Download package to cache
	// 2. Verify checksum
	// 3. Extract to target directory
	// 4. Handle file conflicts

	return nil
}

// runHooks executes hooks for a specific event.
func (i *Installer) runHooks(event, packageName string, pkg *repository.Package) error {
	if i.hookManager == nil {
		return nil // No hook manager configured
	}

	// Convert event string to HookType
	var hookType hook.HookType
	switch event {
	case "pre-install", "pre-update":
		hookType = hook.PreInstall
	case "post-install", "post-update":
		hookType = hook.PostInstall
	default:
		return hook.ErrUnsupportedHookEvent(event)
	}

	// Create hook context
	hookCtx := hook.HookContext{
		PackageName:    pkg.Name,
		PackageVersion: pkg.Version,
		PackagePath:    i.getPackageInstallDir(pkg.Name),
		InstallPath:    i.getPackageInstallDir(pkg.Name),
		Vars: map[string]interface{}{
			"config": map[string]interface{}{
				"cache_dir":     i.cacheDir,
				"installed_dir": i.installedDir,
				"meta_dir":      i.metaDir,
			},
			"package": map[string]interface{}{
				"name":    pkg.Name,
				"version": pkg.Version,
				"url":     pkg.URL,
			},
		},
	}

	// Execute the hook
	if err := i.hookManager.Execute(hookType, hookCtx); err != nil {
		return errors.Wrapf(err, "failed to execute %s hook", event)
	}
	return nil
}
