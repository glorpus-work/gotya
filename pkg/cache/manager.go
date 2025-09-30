package cache

import (
	"os"
	"path/filepath"
	"time"

	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/fsutil"
)

// DefaultManager implements the Manager interface for cache operations.
type DefaultManager struct {
	directory string
}

// NewManager creates a new cache manager.
func NewManager(directory string) *DefaultManager {
	return &DefaultManager{
		directory: directory,
	}
}

// NewDefaultManager creates a new cache manager with default directory.
func NewDefaultManager() (*DefaultManager, error) {
	cacheDir, err := fsutil.GetCacheDir()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get user cache directory")
	}

	// Ensure the cache directory exists with appropriate permissions
	if err := os.MkdirAll(cacheDir, os.FileMode(CacheDirPerm)); err != nil {
		return nil, errors.Wrapf(err, "failed to create cache directory")
	}

	return NewManager(cacheDir), nil
}

// Clean removes cached files according to the specified options.
func (cm *DefaultManager) Clean(options CleanOptions) (*CleanResult, error) {
	result := &CleanResult{}

	// Default to cleaning all if no specific flags are set
	if !options.Indexes && !options.Artifacts {
		options.All = true
	}

	if options.All || options.Indexes {
		size, err := cm.cleanIndexCache()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to clean index cache")
		}
		result.IndexFreed = size
		result.TotalFreed += size
	}

	if options.All || options.Artifacts {
		size, err := cm.cleanArtifactCache()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to clean artifact cache")
		}
		result.ArtifactFreed = size
		result.TotalFreed += size
	}

	return result, nil
}

// GetInfo returns information about the cache.
func (cm *DefaultManager) GetInfo() (*Info, error) {
	info := &Info{
		Directory:   cm.directory,
		LastCleaned: time.Now(), // Set current time as last cleaned time
	}

	// Get index cache info
	indexDir := filepath.Join(cm.directory, "indexes")
	indexSize, indexFiles, err := getDirSizeAndFiles(indexDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrapf(err, "failed to get index cache info")
	}
	info.IndexSize = indexSize
	info.IndexFiles = indexFiles

	// Get artifact cache info
	pkgDir := filepath.Join(cm.directory, "packages")
	pkgSize, pkgFiles, err := getDirSizeAndFiles(pkgDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrapf(err, "failed to get artifact cache info")
	}
	info.ArtifactSize = pkgSize
	info.ArtifactFiles = pkgFiles

	// Calculate total size
	info.TotalSize = info.IndexSize + info.ArtifactSize

	return info, nil
}

// GetDirectory returns the cache directory path.
func (cm *DefaultManager) GetDirectory() string {
	return cm.directory
}

// SetDirectory sets the cache directory path.
func (cm *DefaultManager) SetDirectory(dir string) error {
	if dir == "" {
		return ErrCacheDirectory
	}
	cm.directory = dir
	return nil
}

// cleanIndexCache removes all index cache files.
func (cm *DefaultManager) cleanIndexCache() (int64, error) {
	indexDir := filepath.Join(cm.directory, "indexes")
	return cleanDirectory(indexDir)
}

// cleanArtifactCache removes all artifact cache files.
func (cm *DefaultManager) cleanArtifactCache() (int64, error) {
	packageDir := filepath.Join(cm.directory, "packages")
	return cleanDirectory(packageDir)
}

// cleanDirectory removes a directory and returns bytes freed.
func cleanDirectory(dir string) (int64, error) {
	var totalSize int64

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return 0, nil
	}

	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, errors.Wrapf(err, "error walking directory %s", dir)
	}

	// Remove the directory
	if err := os.RemoveAll(dir); err != nil {
		return 0, errors.Wrapf(err, "failed to remove directory %s", dir)
	}

	// Recreate empty directory with cache-specific permissions
	if err := os.MkdirAll(dir, os.FileMode(CacheDirPerm)); err != nil {
		return totalSize, errors.Wrapf(err, "failed to recreate directory %s", dir)
	}

	return totalSize, nil
}

// getDirSizeAndFiles calculates directory size and file count.
// Returns:
//   - size: total size of all files in bytes
//   - count: total number of files
//   - err: any error that occurred during the operation
func getDirSizeAndFiles(dir string) (size int64, count int, err error) {
	if _, err = os.Stat(dir); os.IsNotExist(err) {
		return 0, 0, nil
	}

	err = filepath.Walk(dir, func(_ string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() {
			size += info.Size()
			count++
		}
		return nil
	})
	if err != nil {
		err = errors.Wrapf(err, "error walking directory %s", dir)
	}
	return size, count, err
}
