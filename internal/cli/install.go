package cli

import (
	"context"
	"fmt"

	"github.com/cperrin88/gotya/pkg/model"
	"github.com/cperrin88/gotya/pkg/orchestrator"
	"github.com/spf13/cobra"
)

// NewInstallCmd creates the install command.
func NewInstallCmd() *cobra.Command {
	var (
		force       bool
		skipDeps    bool
		dryRun      bool
		concurrency int
		cacheDir    string
	)

	cmd := &cobra.Command{
		Use:   "install [PACKAGE...]",
		Short: "Install packages",
		Long: `Install one or more packages from the configured repositories.
Dependencies will be automatically resolved and installed unless --skip-deps is used.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runInstall(args, force, skipDeps, dryRun, concurrency, cacheDir)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force installation even if artifact already exists")
	cmd.Flags().BoolVar(&skipDeps, "skip-deps", false, "Skip dependency resolution")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Resolve and print actions without executing")
	cmd.Flags().IntVar(&concurrency, "concurrency", 0, "Number of parallel downloads (0=auto)")
	cmd.Flags().StringVar(&cacheDir, "cache-dir", "", "Download cache directory (defaults to config)")

	return cmd
}

/*
// NewUpdateCmd creates the update command.

	func NewUpdateCmd() *cobra.Command {
		var all bool

		cmd := &cobra.Command{
			Use:   "update [PACKAGE...]",
			Short: "Update packages",
			Long: `Update one or more installed packages to their latest versions.

Use --all to update all installed packages.`,

			RunE: func(_ *cobra.Command, args []string) error {
				return runUpdate(args, all)
			},
		}

		cmd.Flags().BoolVar(&all, "all", false, "Update all installed packages")

		return cmd
	}
*/
func runInstall(packages []string, force, skipDeps bool, dryRun bool, concurrency int, cacheDir string) error {
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
		return fmt.Errorf("index manager does not support planning (missing Resolve method)")
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
	orch := orchestrator.New(planner, dlManager, artifactManager, hooks)

	opts := orchestrator.Options{CacheDir: cacheDir, Concurrency: concurrency, DryRun: dryRun}
	ctx := context.Background()

	// Process each artifact
	for _, pkgName := range packages {
		req := model.ResolveRequest{
			Name:    pkgName,
			Version: ">= 0.0.0",
			OS:      cfg.Settings.Platform.OS,
			Arch:    cfg.Settings.Platform.Arch,
		}
		if skipDeps {
			// currently no dependency expansion; reserved for future
		}
		if err := orch.Install(ctx, req, opts); err != nil {
			return fmt.Errorf("failed to install %s: %w", pkgName, err)
		}
	}

	return nil
}

/*
func runUpdate(packages []string, all bool) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Create orchestrator with hooks
	orch := orchestrator.New(planner, dlManager, artifactManager, hooks)

	// Create orchestrator with nil hooks manager for now
	pkgInstaller := orchestrator.New(cfg, dlManager, artifactManager)

	// Get packages to update
	var packagesToUpdate []string
	switch {
	case all:
		// Load installed packages database to get all installed packages
		installedDB, err := pkgpkg.LoadInstalledDatabase(cfg.GetDatabasePath())
		if err != nil {
			return fmt.Errorf("failed to load installed packages database: %w", err)
		}
		// Get list of installed packages
		for _, artifact := range installedDB.Artifacts {
			packagesToUpdate = append(packagesToUpdate, artifact.Name)
		}
	case len(packages) > 0:
		packagesToUpdate = packages
	default:
		return fmt.Errorf("no packages specified and --all flag not used")
	}

	// Process each artifact
	updated := false
	for _, pkgName := range packagesToUpdate {
		wasUpdated, err := pkgInstaller.UpdateArtifact(pkgName)
		if err != nil {
			logger.Warnf("Failed to update %s: %v", pkgName, err)
			continue
		}
		updated = updated || wasUpdated
	}

	if !updated {
		logger.Info("All packages are up to date")
	}

	return nil
}
*/
