package cli

import (
	"fmt"
	"time"

	"github.com/cperrin88/gotya/pkg/config"
	pkg "github.com/cperrin88/gotya/pkg/package"
	"github.com/cperrin88/gotya/pkg/repository"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// NewInstallCmd creates the install command
func NewInstallCmd() *cobra.Command {
	var (
		force    bool
		skipDeps bool
	)

	cmd := &cobra.Command{
		Use:   "install [PACKAGE...]",
		Short: "Install packages",
		Long: `Install one or more packages from the configured repositories.
Dependencies will be automatically resolved and installed unless --skip-deps is used.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall(cmd, args, force, skipDeps)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force installation even if package already exists")
	cmd.Flags().BoolVar(&skipDeps, "skip-deps", false, "Skip dependency resolution")

	return cmd
}

// NewUpdateCmd creates the update command
func NewUpdateCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "update [PACKAGE...]",
		Short: "Update packages",
		Long: `Update one or more installed packages to their latest versions.
Use --all to update all installed packages.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd, args, all)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Update all installed packages")

	return cmd
}

func runInstall(cmd *cobra.Command, packages []string, force, skipDeps bool) error {
	cfg, manager, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	Debug("Installing packages", logrus.Fields{
		"packages":  packages,
		"force":     force,
		"skip_deps": skipDeps,
	})

	// Load installed packages database
	installedDB, err := pkg.LoadInstalledDatabase(cfg.GetDatabasePath())
	if err != nil {
		return fmt.Errorf("failed to load installed packages database: %w", err)
	}

	if skipDeps {
		Debug("Dependency resolution disabled")
	}

	for _, packageName := range packages {
		Debug("Installing package", logrus.Fields{"package": packageName})

		// Check if package is already installed
		if !force && installedDB.IsPackageInstalled(packageName) {
			Warn("Package already installed", logrus.Fields{"package": packageName})
			continue
		}

		// Find package in repositories
		packageInfo, repoName, err := findPackageInRepositories(manager, packageName)
		if err != nil {
			Error("Package not found", logrus.Fields{"package": packageName, "error": err.Error()})
			continue
		}

		Info("Found package", logrus.Fields{
			"package":    packageName,
			"version":    packageInfo.Version,
			"repository": repoName,
		})

		// Resolve dependencies if enabled
		var dependenciesToInstall []string
		if !skipDeps && len(packageInfo.Dependencies) > 0 {
			Debug("Resolving dependencies", logrus.Fields{
				"package":      packageName,
				"dependencies": packageInfo.Dependencies,
			})

			for _, dep := range packageInfo.Dependencies {
				if !installedDB.IsPackageInstalled(dep) {
					dependenciesToInstall = append(dependenciesToInstall, dep)
				}
			}

			// Install dependencies first
			if len(dependenciesToInstall) > 0 {
				Info("Installing dependencies", logrus.Fields{
					"package":      packageName,
					"dependencies": dependenciesToInstall,
				})

				for _, dep := range dependenciesToInstall {
					if err := installSinglePackage(cfg, manager, installedDB, dep, false); err != nil {
						Error("Failed to install dependency", logrus.Fields{
							"package":    packageName,
							"dependency": dep,
							"error":      err.Error(),
						})
						return fmt.Errorf("failed to install dependency %s: %w", dep, err)
					}
				}
			}
		}

		// Install the main package
		if err := installSinglePackage(cfg, manager, installedDB, packageName, force); err != nil {
			Error("Failed to install package", logrus.Fields{
				"package": packageName,
				"error":   err.Error(),
			})
			return fmt.Errorf("failed to install package %s: %w", packageName, err)
		}

		Success("Package installed successfully", logrus.Fields{"package": packageName})
	}

	// Save the updated database
	if err := installedDB.Save(cfg.GetDatabasePath()); err != nil {
		return fmt.Errorf("failed to save installed packages database: %w", err)
	}

	return nil
}

func findPackageInRepositories(manager repository.Manager, packageName string) (*pkg.PackageMetadata, string, error) {
	repos := manager.ListRepositories()

	for _, repo := range repos {
		if !repo.Enabled {
			continue
		}

		index, err := manager.GetRepositoryIndex(repo.Name)
		if err != nil {
			continue
		}

		packages := index.GetPackages()
		for _, packageInfo := range packages {
			if packageInfo.Name == packageName {
				return &packageInfo, repo.Name, nil
			}
		}
	}

	return nil, "", fmt.Errorf("package not found: %s", packageName)
}

func installSinglePackage(cfg *config.Config, manager repository.Manager, installedDB *pkg.InstalledDatabase, packageName string, force bool) error {
	// Find package
	packageInfo, repoName, err := findPackageInRepositories(manager, packageName)
	if err != nil {
		return err
	}

	// Check if already installed
	if !force && installedDB.IsPackageInstalled(packageName) {
		return fmt.Errorf("package %s is already installed", packageName)
	}

	// For now, we'll create a simplified installation process
	// In a real implementation, this would:
	// 1. Download the package file from the repository
	// 2. Extract and validate the package
	// 3. Copy files to their destinations
	// 4. Run pre/post install scripts
	// 5. Update the installed packages database

	Debug("Simulating package installation", logrus.Fields{"package": packageName})

	// Create installed package entry
	installedPkg := pkg.InstalledPackage{
		Name:          packageInfo.Name,
		Version:       packageInfo.Version,
		Description:   packageInfo.Description,
		InstalledAt:   time.Now(),
		InstalledFrom: repoName,
		Files:         []string{}, // Would be populated during real installation
		Checksum:      "",         // Would be calculated from package file
	}

	// Add to database
	installedDB.AddPackage(installedPkg)

	return nil
}

func runUpdate(cmd *cobra.Command, packages []string, all bool) error {
	cfg, manager, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	// Load installed packages database
	installedDB, err := pkg.LoadInstalledDatabase(cfg.GetDatabasePath())
	if err != nil {
		return fmt.Errorf("failed to load installed packages database: %w", err)
	}

	if all {
		Debug("Updating all installed packages")

		installedPackages := installedDB.GetInstalledPackages()
		if len(installedPackages) == 0 {
			Info("No packages installed")
			return nil
		}

		Info("Checking for updates", logrus.Fields{"installed_packages": len(installedPackages)})

		var updatedCount int
		for _, installedPkg := range installedPackages {
			updated, err := updateSinglePackage(cfg, manager, installedDB, installedPkg.Name)
			if err != nil {
				Warn("Failed to update package", logrus.Fields{
					"package": installedPkg.Name,
					"error":   err.Error(),
				})
				continue
			}
			if updated {
				updatedCount++
			}
		}

		Info("Update completed", logrus.Fields{
			"checked": len(installedPackages),
			"updated": updatedCount,
		})
	} else {
		if len(packages) == 0 {
			return fmt.Errorf("specify packages to update or use --all flag")
		}

		Debug("Updating packages", logrus.Fields{"packages": packages})

		for _, packageName := range packages {
			Debug("Updating package", logrus.Fields{"package": packageName})

			// Check if package is installed
			if !installedDB.IsPackageInstalled(packageName) {
				Warn("Package not installed", logrus.Fields{"package": packageName})
				continue
			}

			updated, err := updateSinglePackage(cfg, manager, installedDB, packageName)
			if err != nil {
				Error("Failed to update package", logrus.Fields{
					"package": packageName,
					"error":   err.Error(),
				})
				continue
			}

			if updated {
				Success("Package updated successfully", logrus.Fields{"package": packageName})
			} else {
				Info("Package is already up to date", logrus.Fields{"package": packageName})
			}
		}
	}

	// Save the updated database
	if err := installedDB.Save(cfg.GetDatabasePath()); err != nil {
		return fmt.Errorf("failed to save installed packages database: %w", err)
	}

	return nil
}

func updateSinglePackage(cfg *config.Config, manager repository.Manager, installedDB *pkg.InstalledDatabase, packageName string) (bool, error) {
	// Get currently installed version
	installedPkg := installedDB.FindPackage(packageName)
	if installedPkg == nil {
		return false, fmt.Errorf("package %s is not installed", packageName)
	}

	// Find package in repositories
	packageInfo, repoName, err := findPackageInRepositories(manager, packageName)
	if err != nil {
		return false, fmt.Errorf("package not found in repositories: %w", err)
	}

	// Compare versions (simple string comparison for now)
	if packageInfo.Version == installedPkg.Version {
		Debug("Package is already up to date", logrus.Fields{
			"package": packageName,
			"version": packageInfo.Version,
		})
		return false, nil
	}

	Info("Updating package", logrus.Fields{
		"package":     packageName,
		"old_version": installedPkg.Version,
		"new_version": packageInfo.Version,
		"repository":  repoName,
	})

	// For now, simulate the update by updating the database entry
	// In a real implementation, this would:
	// 1. Download the new package version
	// 2. Remove old files
	// 3. Install new files
	// 4. Run update scripts
	// 5. Update the database

	updatedPkg := pkg.InstalledPackage{
		Name:          packageInfo.Name,
		Version:       packageInfo.Version,
		Description:   packageInfo.Description,
		InstalledAt:   time.Now(),
		InstalledFrom: repoName,
		Files:         installedPkg.Files, // Keep existing files for now
		Checksum:      "",                 // Would be recalculated
	}

	installedDB.AddPackage(updatedPkg)

	return true, nil
}
