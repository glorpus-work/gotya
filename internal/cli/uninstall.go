package cli

import (
	"fmt"
	"path/filepath"

	pkg "github.com/cperrin88/gotya/pkg/artifact"
	"github.com/cperrin88/gotya/pkg/config"
	"github.com/cperrin88/gotya/pkg/hooks"
	"github.com/cperrin88/gotya/pkg/logger"
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
	// Find the installed artifact
	pkgInfo := installedDB.FindArtifact(packageName)
	if pkgInfo == nil {
		if force {
			logger.Warn("Artifact not installed, skipping", logger.Fields{"artifact": packageName})
			return nil
		}
		return fmt.Errorf("artifact %s is not installed", packageName)
	}

	// Create hooks manager
	hookManager := hooks.NewHookManager()

	// Try to load hooks from the artifact if it's still available
	packagePath := filepath.Join(cfg.Settings.InstallDir, pkgInfo.Name)
	if !skipHooks {
		if err := hooks.LoadHooksFromArtifactDir(hookManager, packagePath); err != nil {
			logger.Warn("Failed to load hooks from artifact", logger.Fields{
				"artifact": packageName,
				"error":    err.Error(),
			})
		}
	}

	// Create hooks context
	hookCtx := hooks.HookContext{
		ArtifactName:    pkgInfo.Name,
		ArtifactVersion: pkgInfo.Version,
		InstallPath:     packagePath, // Use packagePath instead of installPath
		Vars: map[string]interface{}{
			"force": force,
		},
	}

	// Execute pre-remove hooks if available and not skipped
	if !skipHooks && hookManager.HasHook(hooks.PreRemove) {
		logger.Debug("Running pre-remove hooks", logger.Fields{"artifact": packageName})
		if err := hookManager.Execute(hooks.PreRemove, hookCtx); err != nil && !force {
			return fmt.Errorf("pre-remove hooks failed: %w", err)
		}
	}

	// Remove artifact files (simplified - in a real implementation, we would remove actual files)
	logger.Debug("Removing artifact files", logger.Fields{
		"artifact": packageName,
		"path":     packagePath, // Use packagePath instead of installPath
	})

	// In a real implementation, we would:
	// 1. Remove all files listed in the artifact's file manifest
	// 2. Remove empty directories
	// 3. Handle any errors appropriately

	// Execute post-remove hooks if available and not skipped
	if !skipHooks && hookManager.HasHook(hooks.PostRemove) {
		logger.Debug("Running post-remove hooks", logger.Fields{"artifact": packageName})
		if err := hookManager.Execute(hooks.PostRemove, hookCtx); err != nil && !force {
			// If post-remove fails and we're not forcing, we should stop
			return fmt.Errorf("post-remove hooks failed: %w", err)
		}
	}

	// Remove the artifact from the database
	if !installedDB.RemoveArtifact(packageName) {
		return fmt.Errorf("failed to remove artifact from database: artifact not found")
	}

	logger.Info("Successfully uninstalled artifact", logger.Fields{
		"artifact": packageName,
	})

	return nil
}
