package cli

import (
	"context"
	"fmt"

	"github.com/cperrin88/gotya/internal/logger"
	installer "github.com/cperrin88/gotya/pkg/orchestrator"
	"github.com/spf13/cobra"
)

// NewSyncCmd creates the sync command.
func NewSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Synchronize artifact index indexes",
		Long: `Synchronize artifact index indexes by downloading the latest
artifact lists from configured repositories.`,
		RunE: runSync,
	}

	return cmd
}

func runSync(_ *cobra.Command, _ []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Build components
	dl := loadDownloadManager(cfg)
	idx := loadIndexManager(cfg)
	orch := &installer.Orchestrator{DL: dl}

	logger.Debug("Synchronizing index indexes...")

	repos := idx.ListRepositories()
	if err := orch.SyncAll(context.Background(), repos, cfg.GetIndexDir(), installer.Options{Concurrency: cfg.Settings.MaxConcurrent}); err != nil {
		return fmt.Errorf("failed to sync repositories: %w", err)
	}

	logger.Success("Repository indexes synchronized successfully")
	return nil
}
