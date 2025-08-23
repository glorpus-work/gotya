package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/fsutil"
)

// Syncer handles repository synchronization operations.
type Syncer struct {
	httpClient *HTTPClient
	cacheDir   string
}

// NewSyncer creates a new repository syncer.
func NewSyncer(cacheDir string, httpTimeout time.Duration) *Syncer {
	return &Syncer{
		httpClient: NewHTTPClient(httpTimeout),
		cacheDir:   cacheDir,
	}
}

// SyncRepository synchronizes a single repository.
func (s *Syncer) SyncRepository(ctx context.Context, info Info) (*IndexImpl, error) {
	if !info.Enabled {
		return nil, errors.Wrapf(ErrRepositoryDisabled, "repository '%s' is disabled", info.Name)
	}

	// Create cache directory for this repository
	repoCacheDir := filepath.Join(s.cacheDir, "repositories", info.Name)
	if err := fsutil.EnsureDir(repoCacheDir); err != nil {
		return nil, errors.Wrapf(err, "failed to create repository cache directory: %s", repoCacheDir)
	}

	// Get the last modified time of the cached index if it exists
	indexPath := filepath.Join(repoCacheDir, "index.json")
	var lastModified time.Time
	if info, err := os.Stat(indexPath); err == nil {
		lastModified = info.ModTime()
	}

	// Download the repository index
	index, modifiedTime, err := s.httpClient.DownloadIndex(ctx, info.URL, lastModified)
	if err != nil {
		if err == ErrNotModified {
			// Index not modified, load from cache
			return s.LoadCachedIndex(info.Name)
		}
		return nil, errors.Wrap(err, "failed to download repository index")
	}

	// Validate the downloaded index
	if err := s.validateIndex(index); err != nil {
		return nil, errors.Wrap(err, "invalid repository index")
	}

	// Save the index to cache
	if err := s.saveIndexToCache(index, indexPath); err != nil {
		return nil, errors.Wrapf(err, "failed to save index to cache: %s", indexPath)
	}

	// Update the last modified time of the cached file to match the server
	if !modifiedTime.IsZero() {
		if err := os.Chtimes(indexPath, modifiedTime, modifiedTime); err != nil {
			// Non-fatal error, just print it
			_ = fmt.Sprintf("warning: failed to update index modification time: %v", err)
		}
	}

	return index, nil
}

// safePathJoin joins path elements and ensures the result is within the base directory.
func safePathJoin(baseDir string, elems ...string) (string, error) {
	// Clean and join all path elements
	path := filepath.Join(append([]string{baseDir}, elems...)...)

	// Clean the path to remove any .. or .
	cleanPath := filepath.Clean(path)

	// Verify the final path is still within the base directory
	relPath, err := filepath.Rel(baseDir, cleanPath)
	if err != nil || strings.HasPrefix(relPath, "..") || strings.HasPrefix(relPath, ".") {
		return "", fmt.Errorf("invalid path: path traversal detected")
	}

	return cleanPath, nil
}

// LoadCachedIndex loads a repository index from cache.
func (s *Syncer) LoadCachedIndex(repoName string) (*IndexImpl, error) {
	// Use safe path construction
	indexPath, err := safePathJoin(s.cacheDir, "repositories", repoName, "index.json")
	if err != nil {
		return nil, errors.Wrap(err, "invalid repository path")
	}

	file, err := os.Open(filepath.Clean(indexPath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.Wrapf(ErrRepositoryIndexMissing, "no cached index for repository '%s'", repoName)
		}
		return nil, errors.Wrapf(err, "failed to open cached index: %s", indexPath)
	}
	defer file.Close()

	// Get file info for last modified time
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Parse the index
	index, err := ParseIndexFromReader(file)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse cached index")
	}

	// Update the LastUpdate field from the file's modification time if not set
	if index.LastUpdate.IsZero() {
		index.LastUpdate = fileInfo.ModTime()
	}

	return index, nil
}

// GetCacheAge returns the age of the cached index.
func (s *Syncer) GetCacheAge(repoName string) (time.Duration, error) {
	indexPath := filepath.Join(s.cacheDir, "repositories", repoName, "index.json")

	info, err := os.Stat(indexPath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat cached index: %w", err)
	}

	return time.Since(info.ModTime()), nil
}

// IsCacheStale checks if the cache is older than the given duration.
func (s *Syncer) IsCacheStale(repoName string, maxAge time.Duration) bool {
	age, err := s.GetCacheAge(repoName)
	if err != nil {
		return true // If we can't determine age, consider it stale
	}
	return age > maxAge
}

// validateIndex performs basic validation on the downloaded index.
func (s *Syncer) validateIndex(index *IndexImpl) error {
	if index == nil {
		return fmt.Errorf("index is nil")
	}

	if index.FormatVersion == "" {
		return fmt.Errorf("index missing format version")
	}

	// Basic validation - ensure we have some packages
	packages := index.GetPackages()
	if len(packages) == 0 {
		return fmt.Errorf("index contains no packages")
	}

	// Validate each package has required fields
	for i := range packages {
		pkg := &packages[i]
		if pkg.Name == "" {
			return fmt.Errorf("package %d: missing name", i)
		}
		if pkg.Version == "" {
			return fmt.Errorf("package '%s': missing version", pkg.Name)
		}
		if pkg.URL == "" {
			return fmt.Errorf("package '%s': missing URL", pkg.Name)
		}
		if pkg.Checksum == "" {
			return fmt.Errorf("package '%s': missing checksum", pkg.Name)
		}
	}

	return nil
}

// saveIndexToCache saves the index to the cache directory.
func (s *Syncer) saveIndexToCache(index *IndexImpl, indexPath string) (err error) {
	// Ensure directory exists with secure permissions
	if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
		return errors.Wrapf(err, "failed to create cache directory: %s", filepath.Dir(indexPath))
	}

	// Create temporary file with safe path
	tempPath, err := safePathJoin(filepath.Dir(indexPath), filepath.Base(indexPath)+".tmp")
	if err != nil {
		return errors.Wrapf(err, "invalid temp file path: %s", tempPath)
	}

	// Use filepath.Clean to ensure path is clean before opening
	file, err := os.Create(filepath.Clean(tempPath))
	if err != nil {
		return errors.Wrapf(err, "failed to create cache file: %s", tempPath)
	}

	// Ensure cleanup on error
	defer func() {
		if err != nil {
			// Try to clean up the temp file on error
			_ = os.Remove(tempPath)
		}
	}()

	// Write index data
	data, err := index.ToJSON()
	if err != nil {
		return errors.Wrap(err, "failed to marshal index to JSON")
	}

	if _, err := file.Write(data); err != nil {
		return errors.Wrapf(err, "failed to write index to file: %s", tempPath)
	}

	// Ensure the file is synced to disk
	if err = file.Sync(); err != nil {
		return fmt.Errorf("failed to sync index to disk: %s: %w", tempPath, err)
	}

	// Close the file before renaming
	if err = file.Close(); err != nil {
		return fmt.Errorf("failed to close index file: %w", err)
	}

	// Atomically replace the index file
	if err = os.Rename(tempPath, indexPath); err != nil {
		return fmt.Errorf("failed to replace index file: %w", err)
	}

	return nil
}
