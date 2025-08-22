package cache

import (
	"fmt"
	"time"

	"github.com/cperrin88/gotya/pkg/logger"
	"github.com/sirupsen/logrus"
)

// CacheOperation represents an operation that can be performed on the cache.
type CacheOperation struct {
	manager Manager
}

// NewCacheOperation creates a new cache operation instance.
func NewCacheOperation(manager Manager) *CacheOperation {
	return &CacheOperation{
		manager: manager,
	}
}

// Clean cleans the cache based on the provided options.
func (op *CacheOperation) Clean(all, indexes, packages bool) (string, error) {
	options := CleanOptions{
		All:      all,
		Indexes:  indexes,
		Packages: packages,
	}

	// If no specific option is set, clean both indexes and packages
	if !all && !indexes && !packages {
		options.Indexes = true
		options.Packages = true
	}

	logger.Debug("Cleaning cache", logrus.Fields{
		"all":      options.All,
		"indexes":  options.Indexes,
		"packages": options.Packages,
	})

	result, err := op.manager.Clean(options)
	if err != nil {
		return "", fmt.Errorf("failed to clean cache: %w", err)
	}

	// Generate a human-readable result message
	var msg string
	if result.TotalFreed > 0 {
		msg = fmt.Sprintf("Successfully cleaned cache. Freed %s of disk space.", formatBytes(result.TotalFreed))
		if result.IndexFreed > 0 {
			msg += fmt.Sprintf("\n- Indexes: %s", formatBytes(result.IndexFreed))
		}
		if result.PackageFreed > 0 {
			msg += fmt.Sprintf("\n- Packages: %s", formatBytes(result.PackageFreed))
		}
	} else {
		msg = "No files were removed from the cache."
	}

	return msg, nil
}

// GetInfo returns information about the cache.
func (op *CacheOperation) GetInfo() (string, error) {
	info, err := op.manager.GetInfo()
	if err != nil {
		return "", fmt.Errorf("failed to get cache info: %w", err)
	}

	lastCleaned := "never"
	if !info.LastCleaned.IsZero() {
		lastCleaned = info.LastCleaned.Format(time.RFC1123)
	}

	return fmt.Sprintf(`Cache Information:
  Directory:    %s
  Total Size:   %s
  Indexes:      %s (%d files)
  Packages:     %s (%d files)
  Last Cleaned: %s`,
		info.Directory,
		formatBytes(info.TotalSize),
		formatBytes(info.IndexSize),
		info.IndexFiles,
		formatBytes(info.PackageSize),
		info.PackageFiles,
		lastCleaned,
	), nil
}

// GetDirectory returns the cache directory path.
func (op *CacheOperation) GetDirectory() string {
	return op.manager.GetDirectory()
}

// SetDirectory sets a new cache directory.
func (op *CacheOperation) SetDirectory(dir string) error {
	if dir == "" {
		return fmt.Errorf("cache directory cannot be empty")
	}

	logger.Debug("Setting cache directory", logrus.Fields{"directory": dir})
	return op.manager.SetDirectory(dir)
}

// formatBytes converts bytes to a human-readable string.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"K", "M", "G", "T", "P", "E"}
	if exp < len(units) {
		return fmt.Sprintf("%.1f %sB", float64(bytes)/float64(div), units[exp])
	}
	return fmt.Sprintf("%d B", bytes)
}
