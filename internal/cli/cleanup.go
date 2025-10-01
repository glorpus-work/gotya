package cli

import (
	"context"
	"fmt"

	"github.com/glorpus-work/gotya/pkg/orchestrator"
	"github.com/spf13/cobra"
)

// NewCleanupCmd creates the cleanup command.
func NewCleanupCmd() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up orphaned automatic artifacts",
		Long: `Clean up orphaned automatic artifacts that have no reverse dependencies.

This command removes artifacts that were installed as dependencies but are no longer needed.
Use --dry-run to see what would be cleaned up without actually removing anything.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runCleanup(dryRun)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be cleaned up without actually removing anything")

	return cmd
}

func runCleanup(dryRun bool) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	artifactManager := loadArtifactManager(cfg)

	ctx := context.Background()

	if dryRun {
		// For dry-run, just show what would be cleaned up without actually doing it
		orphaned, err := artifactManager.GetOrphanedAutomaticArtifacts()
		if err != nil {
			return fmt.Errorf("failed to get orphaned artifacts for dry-run: %w", err)
		}

		if len(orphaned) > 0 {
			fmt.Printf("Would clean up %d orphaned artifacts: %v\n", len(orphaned), orphaned)
		} else {
			fmt.Println("No orphaned artifacts found to clean up")
		}
		return nil
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
	orch := orchestrator.New(nil, nil, nil, artifactManager, hooks)

	// Execute cleanup
	cleaned, err := orch.Cleanup(ctx)
	if err != nil {
		return fmt.Errorf("failed to cleanup orphaned artifacts: %w", err)
	}

	if len(cleaned) > 0 {
		fmt.Printf("Successfully cleaned up %d orphaned artifacts: %v\n", len(cleaned), cleaned)
	} else {
		fmt.Println("No orphaned artifacts found to clean up")
	}

	return nil
}
