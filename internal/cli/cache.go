package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cperrin88/gotya/internal/logger"
	"github.com/cperrin88/gotya/pkg/cache"
	"github.com/spf13/cobra"
)

// NewCacheCmd creates the cache command with subcommands.
func NewCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage artifact cache",
		Long:  "Clean, show information about, and manage the artifact cache",
	}

	cmd.AddCommand(
		newCacheCleanCmd(),
		newCacheInfoCmd(),
		newCacheDirCmd(),
	)

	return cmd
}

func newCacheCleanCmd() *cobra.Command {
	var (
		all      bool
		indexes  bool
		packages bool
	)

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean artifact cache",
		Long:  "Remove cached files to free up disk space",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runCacheClean(all, indexes, packages)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Clean all cached files")
	cmd.Flags().BoolVar(&indexes, "indexes", false, "Clean only index indexes")
	cmd.Flags().BoolVar(&packages, "packages", false, "Clean only downloaded packages")

	return cmd
}

func newCacheInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show cache information",
		Long:  "Display information about the artifact cache",
		RunE:  runCacheInfo,
	}

	return cmd
}

func newCacheDirCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dir",
		Short: "Show cache directory path",
		Long:  "Display the path to the cache directory",
		RunE:  runCacheDir,
	}

	return cmd
}

func runCacheClean(all, indexes, packages bool) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}

	// Create cache manager and operation
	cacheDir := getCacheDir(config)
	cacheManager := cache.NewManager(cacheDir)
	cacheOp := cache.NewOperation(cacheManager)

	// Clean the cache
	// Use the correct method signature for Clean
	result, err := cacheOp.Clean(all || (indexes && packages), indexes, packages)
	if err != nil {
		return fmt.Errorf("failed to clean cache: %w", err)
	}

	// Log the result message
	logger.Info(result)
	return nil
}

func runCacheInfo(_ *cobra.Command, _ []string) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}

	// Create cache manager and operation
	cacheDir := getCacheDir(config)
	cacheManager := cache.NewManager(cacheDir)
	cacheOp := cache.NewOperation(cacheManager)

	// Get cache info
	info, err := cacheOp.GetInfo()
	if err != nil {
		return fmt.Errorf("failed to get cache info: %w", err)
	}

	// Print cache information (info is already a formatted string from GetInfo)
	fmt.Println(info)

	return nil
}

func runCacheDir(_ *cobra.Command, _ []string) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}

	// Create cache manager and operation
	cacheDir := getCacheDir(config)
	cacheManager := cache.NewManager(cacheDir)
	cacheOp := cache.NewOperation(cacheManager)

	// Get and print cache directory
	fmt.Println(cacheOp.GetDirectory())
	return nil
}

// Helper function to get cache directory.
func getCacheDir(config interface{}) string {
	// Try to get from config first
	if cfg, ok := config.(interface{ GetCacheDir() string }); ok {
		if dir := cfg.GetCacheDir(); dir != "" {
			return dir
		}
	}

	// Fallback to default cache directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/gotya-cache"
	}
	return filepath.Join(homeDir, ".cache", "gotya")
}
