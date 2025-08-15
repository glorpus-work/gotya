package cli

import (
	"fmt"

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
	_, manager, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	Debug("Installing packages", logrus.Fields{
		"packages":  packages,
		"force":     force,
		"skip_deps": skipDeps,
	})

	if skipDeps {
		Debug("Dependency resolution disabled")
	}

	for _, pkg := range packages {
		Debug("Installing package", logrus.Fields{"package": pkg})

		if err := manager.InstallPackage(cmd.Context(), pkg, force, !skipDeps); err != nil {
			Error("Failed to install package", logrus.Fields{
				"package": pkg,
				"error":   err.Error(),
			})
			return fmt.Errorf("failed to install package %s: %w", pkg, err)
		}

		Success("Successfully installed package", logrus.Fields{"package": pkg})
	}

	return nil
}

func runUpdate(cmd *cobra.Command, packages []string, all bool) error {
	_, manager, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	if all {
		Debug("Updating all installed packages")

		if err := manager.UpdateAllPackages(cmd.Context()); err != nil {
			Error("Failed to update all packages", logrus.Fields{"error": err.Error()})
			return fmt.Errorf("failed to update packages: %w", err)
		}

		Success("All packages updated successfully")
		return nil
	}

	if len(packages) == 0 {
		return fmt.Errorf("specify packages to update or use --all flag")
	}

	Debug("Updating packages", logrus.Fields{"packages": packages})

	for _, pkg := range packages {
		Debug("Updating package", logrus.Fields{"package": pkg})

		if err := manager.UpdatePackage(cmd.Context(), pkg); err != nil {
			Error("Failed to update package", logrus.Fields{
				"package": pkg,
				"error":   err.Error(),
			})
			return fmt.Errorf("failed to update package %s: %w", pkg, err)
		}

		Success("Successfully updated package", logrus.Fields{"package": pkg})
	}

	return nil
}
