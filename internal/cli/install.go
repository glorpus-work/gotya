package cli

import (
	"context"
	"fmt"

	"github.com/glorpus-work/gotya/pkg/errors"
	"github.com/glorpus-work/gotya/pkg/model"
	"github.com/glorpus-work/gotya/pkg/orchestrator"
	"github.com/spf13/cobra"
)

// NewInstallCmd creates the install command.
func NewInstallCmd() *cobra.Command {
	var (
		dryRun      bool
		concurrency int
		cacheDir    string
	)

	cmd := &cobra.Command{
		Use:   "install [PACKAGE...]",
		Short: "Install packages",
		Long: `Install one or more packages from the configured repositories.
Dependencies will be automatically resolved and installed.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runInstall(args, dryRun, concurrency, cacheDir)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Resolve and print actions without executing")
	cmd.Flags().IntVar(&concurrency, "concurrency", 0, "Number of parallel downloads (0=auto)")
	cmd.Flags().StringVar(&cacheDir, "cache-dir", "", "Download cache directory (defaults to config)")

	return cmd
}

func runInstall(packages []string, dryRun bool, concurrency int, cacheDir string) error {
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

	// Parse dependencies from package arguments
	dependencies, err := ParseDependencies(packages)
	if err != nil {
		return fmt.Errorf("failed to parse dependencies: %w", err)
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
	orch := orchestrator.New(planner, artifactManager, dlManager, artifactManager, hooks)

	opts := orchestrator.InstallOptions{CacheDir: cacheDir, Concurrency: concurrency, DryRun: dryRun}
	ctx := context.Background()

	// Build all resolve requests
	var requests []*model.ResolveRequest
	for _, dep := range dependencies {
		requests = append(requests, &model.ResolveRequest{
			Name:              dep.Name,
			VersionConstraint: dep.VersionConstraint,
			OS:                cfg.Settings.Platform.OS,
			Arch:              cfg.Settings.Platform.Arch,
		})
	}

	if err := orch.Install(ctx, requests, opts); err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}

	return nil
}
