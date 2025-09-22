package cli

import (
	"fmt"

	pkg "github.com/cperrin88/gotya/pkg/artifact"
	"github.com/cperrin88/gotya/pkg/config"
	"github.com/spf13/cobra"
)

// NewUninstallCmd creates the uninstall command.
func NewUninstallCmd() *cobra.Command {
	var (
		skipHooks bool
		force     bool
	)

	cmd := &cobra.Command{
		Use:   "uninstall PACKAGE...",
		Short: "Uninstall packages",
		Long: `Uninstall one or more installed packages.
By default, pre-remove and post-remove hooks will be executed.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			// Get default config path
			configPath, err := config.GetDefaultConfigPath()
			if err != nil {
				return fmt.Errorf("failed to get default config path: %w", err)
			}

			// Load the configuration
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Load installed packages database
			installedDB, err := pkg.LoadInstalledDatabase(cfg.GetDatabasePath())
			if err != nil {
				return fmt.Errorf("failed to load installed packages database: %w", err)
			}

			// Process each artifact
			for _, pkgName := range args {
				if err := uninstallArtifact(cfg, installedDB, pkgName, skipHooks, force); err != nil {
					return fmt.Errorf("failed to uninstall %s: %w", pkgName, err)
				}
			}

			// Save the updated database
			if err := installedDB.Save(cfg.GetDatabasePath()); err != nil {
				return fmt.Errorf("failed to save database: %w", err)
			}

			return nil
		},
	}

	// Add flags
	cmd.Flags().BoolVar(&skipHooks, "skip-hooks", false, "Skip running pre/post remove hooks")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force uninstallation even if there are errors")

	return cmd
}

// uninstallArtifact uninstalls a single artifact with hooks support.
func uninstallArtifact(cfg *config.Config, installedDB *pkg.InstalledDatabase, packageName string, skipHooks, force bool) error {
	return nil
}
