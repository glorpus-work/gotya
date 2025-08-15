package cli

import (
	"fmt"
	"strings"

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
	config, manager, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	if config.Settings.VerboseLogging {
		fmt.Printf("Installing packages: %s\n", strings.Join(packages, ", "))
		if skipDeps {
			fmt.Println("Dependency resolution disabled")
		}
	}

	for _, pkg := range packages {
		if config.Settings.VerboseLogging {
			fmt.Printf("Installing package: %s\n", pkg)
		}

		if err := manager.InstallPackage(cmd.Context(), pkg, force, !skipDeps); err != nil {
			return fmt.Errorf("failed to install package %s: %w", pkg, err)
		}

		fmt.Printf("Successfully installed: %s\n", pkg)
	}

	return nil
}

func runUpdate(cmd *cobra.Command, packages []string, all bool) error {
	config, manager, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	if all {
		if config.Settings.VerboseLogging {
			fmt.Println("Updating all installed packages...")
		}

		if err := manager.UpdateAllPackages(cmd.Context()); err != nil {
			return fmt.Errorf("failed to update packages: %w", err)
		}

		fmt.Println("All packages updated successfully")
		return nil
	}

	if len(packages) == 0 {
		return fmt.Errorf("specify packages to update or use --all flag")
	}

	if config.Settings.VerboseLogging {
		fmt.Printf("Updating packages: %s\n", strings.Join(packages, ", "))
	}

	for _, pkg := range packages {
		if config.Settings.VerboseLogging {
			fmt.Printf("Updating package: %s\n", pkg)
		}

		if err := manager.UpdatePackage(cmd.Context(), pkg); err != nil {
			return fmt.Errorf("failed to update package %s: %w", pkg, err)
		}

		fmt.Printf("Successfully updated: %s\n", pkg)
	}

	return nil
}
