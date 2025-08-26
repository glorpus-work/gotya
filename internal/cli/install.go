package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// NewInstallCmd creates the install command.
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
		RunE: func(_ *cobra.Command, args []string) error {
			return runInstall(args, force, skipDeps)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force installation even if pkg already exists")
	cmd.Flags().BoolVar(&skipDeps, "skip-deps", false, "Skip dependency resolution")

	return cmd
}

/*
// NewUpdateCmd creates the update command.

	func NewUpdateCmd() *cobra.Command {
		var all bool

		cmd := &cobra.Command{
			Use:   "update [PACKAGE...]",
			Short: "Update packages",
			Long: `Update one or more installed packages to their latest versions.

Use --all to update all installed packages.`,

			RunE: func(_ *cobra.Command, args []string) error {
				return runUpdate(args, all)
			},
		}

		cmd.Flags().BoolVar(&all, "all", false, "Update all installed packages")

		return cmd
	}
*/
func runInstall(packages []string, force, skipDeps bool) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	httpClient := loadHttpClient(cfg)
	indexManager := loadIndexManager(cfg, httpClient)
	pkgManger := loadPackageManager(cfg, indexManager, httpClient)

	// Process each pkg
	for _, pkgName := range packages {
		if err := pkgManger.InstallPackage(context.Background(), pkgName, ">= 0.0.0", force); err != nil {
			return fmt.Errorf("failed to install %s: %w", pkgName, err)
		}
	}

	return nil
}

/*
func runUpdate(packages []string, all bool) error {
	cfg, repoManager, err := loadConfig()
	if err != nil {
		return err
	}

	// Create installer with nil hooks manager for now
	pkgInstaller := installer.New(cfg, repoManager, nil)

	// Get packages to update
	var packagesToUpdate []string
	switch {
	case all:
		// Load installed packages database to get all installed packages
		installedDB, err := pkgpkg.LoadInstalledDatabase(cfg.GetDatabasePath())
		if err != nil {
			return fmt.Errorf("failed to load installed packages database: %w", err)
		}
		// Get list of installed packages
		for _, pkg := range installedDB.Packages {
			packagesToUpdate = append(packagesToUpdate, pkg.Name)
		}
	case len(packages) > 0:
		packagesToUpdate = packages
	default:
		return fmt.Errorf("no packages specified and --all flag not used")
	}

	// Process each pkg
	updated := false
	for _, pkgName := range packagesToUpdate {
		wasUpdated, err := pkgInstaller.UpdatePackage(pkgName)
		if err != nil {
			logger.Warnf("Failed to update %s: %v", pkgName, err)
			continue
		}
		updated = updated || wasUpdated
	}

	if !updated {
		logger.Info("All packages are up to date")
	}

	return nil
}
*/
