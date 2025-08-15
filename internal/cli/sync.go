package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewSyncCmd creates the sync command
func NewSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Synchronize package repository indexes",
		Long: `Synchronize package repository indexes by downloading the latest
package lists from configured repositories.`,
		RunE: runSync,
	}

	return cmd
}

func runSync(cmd *cobra.Command, args []string) error {
	config, manager, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	if config.Settings.VerboseLogging {
		fmt.Println("Synchronizing repository indexes...")
	}

	if err := manager.SyncRepositories(cmd.Context()); err != nil {
		return fmt.Errorf("failed to sync repositories: %w", err)
	}

	fmt.Println("Repository indexes synchronized successfully")
	return nil
}
