package cache

import (
	"os"
	"path/filepath"
	"time"

	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/fsutil"
)

// CacheManager implements the Manager interface.
type CacheManager struct {
	directory string
}

// NewManager creates a new cache manager.
func NewManager(directory string) *CacheManager {
	return &CacheManager{
		directory: directory,
	}
}

// NewDefaultManager creates a new cache manager with default directory.
func NewDefaultManager() (*CacheManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get user home directory")
	}

	cacheDir := filepath.Join(homeDir, ".cache", "gotya")
	if err := os.MkdirAll(cacheDir, os.FileMode(fsutil.DirModeDefault)); err != nil {
		return nil, errors.Wrapf(err, "failed to create cache directory")
	}

	return NewManager(cacheDir), nil
}

// Clean removes cached files according to the specified options.
func (cm *CacheManager) Clean(options CleanOptions) (*CleanResult, error) {
	result := &CleanResult{}

	// Default to cleaning all if no specific flags are set
	if !options.Indexes && !options.Packages {
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

	if options.All || options.Packages {
		size, err := cm.cleanPackageCache()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to clean pkg cache")
		}
		result.PackageFreed = size
		result.TotalFreed += size
	}

	return result, nil
}

// GetInfo returns information about the cache.
func (cm *CacheManager) GetInfo() (*Info, error) {
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

	// Get pkg cache info
	pkgDir := filepath.Join(cm.directory, "packages")
	pkgSize, pkgFiles, err := getDirSizeAndFiles(pkgDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrapf(err, "failed to get pkg cache info")
	}
	info.PackageSize = pkgSize
	info.PackageFiles = pkgFiles

	// Calculate total size
	info.TotalSize = info.IndexSize + info.PackageSize

	return info, nil
}

// GetDirectory returns the cache directory path.
func (cm *CacheManager) GetDirectory() string {
	return cm.directory
}

// SetDirectory sets the cache directory path.
func (cm *CacheManager) SetDirectory(dir string) error {
	if dir == "" {
		return ErrCacheDirectory
	}
	cm.directory = dir
	return nil
}

// cleanIndexCache removes all index cache files.
func (cm *CacheManager) cleanIndexCache() (int64, error) {
	indexDir := filepath.Join(cm.directory, "indexes")
	return cleanDirectory(indexDir)
}

// cleanPackageCache removes all pkg cache files.
func (cm *CacheManager) cleanPackageCache() (int64, error) {
	packageDir := filepath.Join(cm.directory, "packages")
	return cleanDirectory(packageDir)
}

// cleanDirectory removes a directory and returns bytes freed.
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
		return 0, errors.Wrapf(err, "error walking directory %s", dir)
	}

	// Remove the directory
	if err := os.RemoveAll(dir); err != nil {
		return 0, errors.Wrapf(err, "failed to remove directory %s", dir)
	}

	// Recreate empty directory
	if err := fsutil.EnsureDir(dir); err != nil {
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

	err = filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
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
	return
}
