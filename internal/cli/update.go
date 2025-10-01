package cli

import (
	"context"
	"fmt"

	"github.com/glorpus-work/gotya/pkg/errors"
	"github.com/glorpus-work/gotya/pkg/orchestrator"
	"github.com/spf13/cobra"
)

// NewUpdateCmd creates the update command.
func NewUpdateCmd() *cobra.Command {
	var (
		all         bool
		dryRun      bool
		concurrency int
		cacheDir    string
	)

	cmd := &cobra.Command{
		Use:   "update [PACKAGE...]",
		Short: "Update packages",
		Long: `Update one or more installed packages to their latest compatible versions.

Use --all to update all installed packages. If no packages are specified and --all is not used,
the command will return an error.`,
		RunE: func(_ *cobra.Command, args []string) error {
			return runUpdate(args, all, dryRun, concurrency, cacheDir)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Update all installed packages")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Resolve and print actions without executing")
	cmd.Flags().IntVar(&concurrency, "concurrency", 0, "Number of parallel downloads (0=auto)")
	cmd.Flags().StringVar(&cacheDir, "cache-dir", "", "Download cache directory (defaults to config)")

	return cmd
}

func runUpdate(packages []string, all, dryRun bool, concurrency int, cacheDir string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	indexManager := loadIndexManager(cfg)
	artifactManager := loadArtifactManager(cfg)
	dlManager := loadDownloadManager(cfg)

	// default cacheDir from config if not provided
	if cacheDir == "" {
		cacheDir = cfg.GetArtifactCacheDir()
	}

	// Verify interfaces
	planner, ok := indexManager.(orchestrator.ArtifactResolver)
	if !ok {
		return fmt.Errorf("index manager does not support planning (missing Resolve method): %w", errors.ErrValidation)
	}

	// Create progress hooks
	hooks := orchestrator.Hooks{OnEvent: func(e orchestrator.Event) {
		// Simple, human-friendly output
		if e.ID != "" {
			fmt.Printf("%s: %s (%s)\n", e.Phase, e.Msg, e.ID)
		} else {
			fmt.Printf("%s: %s\n", e.Phase, e.Msg)
		}
	}}

	// Create orchestrator with hooks
	orch := orchestrator.New(planner, nil, dlManager, artifactManager, hooks)

	opts := orchestrator.UpdateOptions{
		DryRun:      dryRun,
		Packages:    packages,
		Concurrency: concurrency,
		CacheDir:    cacheDir,
	}

	ctx := context.Background()

	// Validate arguments
	if !all && len(packages) == 0 {
		return fmt.Errorf("no packages specified and --all flag not used: %w", errors.ErrNoArtifactsSpecified)
	}

	// Execute update
	if err := orch.Update(ctx, opts); err != nil {
		return fmt.Errorf("failed to update packages: %w", err)
	}

	return nil
}
