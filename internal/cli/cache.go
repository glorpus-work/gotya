package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

// CacheInfo represents cache information
type CacheInfo struct {
	Directory    string
	TotalSize    int64
	IndexSize    int64
	IndexFiles   int
	PackageSize  int64
	PackageFiles int
	LastCleaned  time.Time
}

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

	// Default to cleaning all if no specific flags are set
	if !indexes && !packages {
		all = true
	}

	if config.Settings.VerboseLogging {
		fmt.Println("Cleaning cache...")
	}

	var cleaned int64

	// Get cache directory
	cacheDir := getCacheDir(config)

	if all || indexes {
		if config.Settings.VerboseLogging {
			fmt.Println("Cleaning repository indexes...")
		}
		size, err := cleanIndexCache(cacheDir)
		if err != nil {
			return fmt.Errorf("failed to clean index cache: %w", err)
		}
		cleaned += size
		fmt.Printf("Cleaned %s of repository indexes\n", formatSize(size))
	}

	if all || packages {
		if config.Settings.VerboseLogging {
			fmt.Println("Cleaning downloaded packages...")
		}
		size, err := cleanPackageCache(cacheDir)
		if err != nil {
			return fmt.Errorf("failed to clean package cache: %w", err)
		}
		cleaned += size
		fmt.Printf("Cleaned %s of downloaded packages\n", formatSize(size))
	}

	fmt.Printf("Total space freed: %s\n", formatSize(cleaned))
	return nil
}

func runCacheInfo(*cobra.Command, []string) error {
	config, _, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	cacheDir := getCacheDir(config)
	info, err := getCacheInfo(cacheDir)
	if err != nil {
		return fmt.Errorf("failed to get cache information: %w", err)
	}

	fmt.Printf("Cache Directory: %s\n", info.Directory)
	fmt.Printf("Total Size: %s\n", formatSize(info.TotalSize))
	fmt.Printf("Index Cache: %s (%d files)\n", formatSize(info.IndexSize), info.IndexFiles)
	fmt.Printf("Package Cache: %s (%d files)\n", formatSize(info.PackageSize), info.PackageFiles)
	fmt.Printf("Last Cleaned: %s\n", info.LastCleaned.Format("2006-01-02 15:04:05"))

	return nil
}

func runCacheDir(*cobra.Command, []string) error {
	config, _, err := loadConfigAndManager()
	if err != nil {
		return err
	}

	cacheDir := getCacheDir(config)
	fmt.Println(cacheDir)
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

// Helper function to clean index cache
func cleanIndexCache(cacheDir string) (int64, error) {
	indexDir := filepath.Join(cacheDir, "indexes")
	return cleanDirectory(indexDir)
}

// Helper function to clean package cache
func cleanPackageCache(cacheDir string) (int64, error) {
	packageDir := filepath.Join(cacheDir, "packages")
	return cleanDirectory(packageDir)
}

// Helper function to clean a directory and return bytes freed
func cleanDirectory(dir string) (int64, error) {
	var totalSize int64

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return 0, nil
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to calculate directory size: %w", err)
	}

	// Remove the directory
	if err := os.RemoveAll(dir); err != nil {
		return 0, fmt.Errorf("failed to remove directory %s: %w", dir, err)
	}

	// Recreate empty directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		return totalSize, fmt.Errorf("failed to recreate directory %s: %w", dir, err)
	}

	return totalSize, nil
}

// Helper function to get cache information
func getCacheInfo(cacheDir string) (*CacheInfo, error) {
	info := &CacheInfo{
		Directory:   cacheDir,
		LastCleaned: time.Now(), // This would ideally be stored somewhere
	}

	// Calculate package cache size
	packageDir := filepath.Join(cacheDir, "packages")
	packageSize, packageFiles, err := getDirSizeAndFiles(packageDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to get package cache info: %w", err)
	}
	info.PackageSize = packageSize
	info.PackageFiles = packageFiles

	info.TotalSize = info.IndexSize + info.PackageSize

	return info, nil
}

// Helper function to get directory size and file count
func getDirSizeAndFiles(dir string) (int64, int, error) {
	var totalSize int64
	var fileCount int

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return 0, 0, nil
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
			fileCount++
		}
		return nil
	})

	return totalSize, fileCount, err
}

// Helper function to format file sizes
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1fGB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
