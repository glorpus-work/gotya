package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// NewUninstallCmd creates the uninstall command.
func NewUninstallCmd() *cobra.Command {
	var (
		purge bool
	)

	cmd := &cobra.Command{
		Use:   "uninstall PACKAGE...",
		Short: "Uninstall packages",
		Long: `Uninstall one or more installed packages.
By default, pre-remove and post-remove hooks will be executed.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			manager := loadArtifactManager(cfg)

			// Process each artifact
			for _, pkgName := range args {
				if err := manager.UninstallArtifact(context.Background(), pkgName, purge); err != nil {
					return fmt.Errorf("failed to uninstall %s: %w", pkgName, err)
				}
			}

			return nil
		},
	}

	// Add flags
	cmd.Flags().BoolVar(&purge, "purge", false, "Remove not only tracked files but all files in the installed directories")

	return cmd
}
