package cli

import (
	"fmt"
	"os"

	"github.com/cperrin88/gotya/pkg/cache"
	"github.com/cperrin88/gotya/pkg/logger"
	"github.com/spf13/cobra"
)

// NewCacheCmd creates the cache command with subcommands
func NewCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage package cache",
		Long:  "Clean, show information about, and manage the package cache",
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
		Short: "Clean package cache",
		Long:  "Remove cached files to free up disk space",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCacheClean(all, indexes, packages)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Clean all cached files")
	cmd.Flags().BoolVar(&indexes, "indexes", false, "Clean only repository indexes")
	cmd.Flags().BoolVar(&packages, "packages", false, "Clean only downloaded packages")

	return cmd
}

func newCacheInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show cache information",
		Long:  "Display information about the package cache",
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
	config, _, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	// Create cache manager and operation
	cacheDir := getCacheDir(config)
	cacheManager := cache.NewManager(cacheDir)
	cacheOp := cache.NewCacheOperation(cacheManager)

	// Clean the cache
	result, err := cacheOp.Clean(all, indexes, packages)
	if err != nil {
		return err
	}

	if result.IndexFreed > 0 {
		logger.Info("Cleaned repository indexes", logrus.Fields{"size": humanize.Bytes(uint64(result.IndexFreed))})
	}
	if result.PackageFreed > 0 {
		logger.Info("Cleaned downloaded packages", logrus.Fields{"size": humanize.Bytes(uint64(result.PackageFreed))})
	}

	logger.Success("Cache cleaning completed", logrus.Fields{"total_freed": humanize.Bytes(uint64(result.TotalFreed))})
	return nil
}

func runCacheInfo(cmd *cobra.Command, args []string) error {
	config, _, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	// Create cache manager and operation
	cacheDir := getCacheDir(config)
	cacheManager := cache.NewManager(cacheDir)
	cacheOp := cache.NewCacheOperation(cacheManager)

	// Get cache info
	info, err := cacheOp.GetInfo()
	if err != nil {
		return err
	}

	fmt.Printf("Cache Directory: %s\n", info.Directory)
	fmt.Printf("Total Size: %s\n", humanize.Bytes(uint64(info.TotalSize)))
	fmt.Printf("Index Cache: %s (%d files)\n", humanize.Bytes(uint64(info.IndexSize)), info.IndexFiles)
	fmt.Printf("Package Cache: %s (%d files)\n", humanize.Bytes(uint64(info.PackageSize)), info.PackageFiles)
	fmt.Printf("Last Cleaned: %s\n", info.LastCleaned.Format("2006-01-02 15:04:05"))

	return nil
}

func runCacheDir(cmd *cobra.Command, args []string) error {
	config, _, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	// Create cache manager and operation
	cacheDir := getCacheDir(config)
	cacheManager := cache.NewManager(cacheDir)
	cacheOp := cache.NewCacheOperation(cacheManager)

	// Get and print cache directory
	fmt.Println(cacheOp.GetDirectory())
	return nil
}

// Helper function to get cache directory
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
