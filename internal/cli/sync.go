package cli

import (
	"context"
	"fmt"

	"github.com/cperrin88/gotya/pkg/logger"
	"github.com/spf13/cobra"
)

// NewSyncCmd creates the sync command.
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

func runSync(_ *cobra.Command, _ []string) error {
	_, manager, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	logger.Debug("Synchronizing repository indexes...")

	if err := manager.SyncRepositories(context.Background()); err != nil {
		return fmt.Errorf("failed to sync repositories: %w", err)
	}

	logger.Success("Repository indexes synchronized successfully")
	return nil
}
