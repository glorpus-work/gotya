package repository

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Syncer handles repository synchronization operations
type Syncer struct {
	httpClient *HTTPClient
	cacheDir   string
}

// NewSyncer creates a new repository syncer
func NewSyncer(cacheDir string, httpTimeout time.Duration) *Syncer {
	return &Syncer{
		httpClient: NewHTTPClient(httpTimeout),
		cacheDir:   cacheDir,
	}
}

// SyncRepository synchronizes a single repository
func (s *Syncer) SyncRepository(ctx context.Context, info Info) (*IndexImpl, error) {
	if !info.Enabled {
		return nil, fmt.Errorf("repository '%s' is disabled", info.Name)
	}

	// Create cache directory for this repository
	repoCacheDir := filepath.Join(s.cacheDir, "repositories", info.Name)
	if err := os.MkdirAll(repoCacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create repository cache directory: %w", err)
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
		return nil, fmt.Errorf("failed to download repository index: %w", err)
	}

	// Validate the downloaded index
	if err := s.validateIndex(index); err != nil {
		return nil, fmt.Errorf("invalid repository index: %w", err)
	}

	// Save the index to cache
	if err := s.saveIndexToCache(index, indexPath); err != nil {
		return nil, fmt.Errorf("failed to save index to cache: %w", err)
	}

	// Update the last modified time of the cached file to match the server
	if !modifiedTime.IsZero() {
		if err := os.Chtimes(indexPath, modifiedTime, modifiedTime); err != nil {
			// Non-fatal error, just log it
			log.Printf("warning: failed to update index modification time: %v", err)
		}
	}

	return index, nil
}

// LoadCachedIndex loads a repository index from cache
func (s *Syncer) LoadCachedIndex(repoName string) (*IndexImpl, error) {
	indexPath := filepath.Join(s.cacheDir, "repositories", repoName, "index.json")

	file, err := os.Open(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no cached index for repository '%s'", repoName)
		}
		return nil, fmt.Errorf("failed to open cached index: %w", err)
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
		return nil, fmt.Errorf("failed to parse cached index: %w", err)
	}

	// Update the LastUpdate field from the file's modification time if not set
	if index.LastUpdate.IsZero() {
		index.LastUpdate = fileInfo.ModTime()
	}

	return index, nil
}

// GetCacheAge returns the age of the cached index
func (s *Syncer) GetCacheAge(repoName string) (time.Duration, error) {
	indexPath := filepath.Join(s.cacheDir, "repositories", repoName, "index.json")

	info, err := os.Stat(indexPath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat cached index: %w", err)
	}

	return time.Since(info.ModTime()), nil
}

// IsCacheStale checks if the cache is older than the given duration
func (s *Syncer) IsCacheStale(repoName string, maxAge time.Duration) bool {
	age, err := s.GetCacheAge(repoName)
	if err != nil {
		return true // If we can't determine age, consider it stale
	}
	return age > maxAge
}

// validateIndex performs basic validation on the downloaded index
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
	for i, pkg := range packages {
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

// saveIndexToCache saves the index to the cache directory
func (s *Syncer) saveIndexToCache(index *IndexImpl, indexPath string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Create temporary file first
	tempPath := indexPath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	// Write index data
	data, err := index.ToJSON()
	if err != nil {
		file.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to serialize index: %w", err)
	}

	if _, err := file.Write(data); err != nil {
		file.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to write index: %w", err)
	}

	file.Close()

	// Atomically replace the index file
	if err := os.Rename(tempPath, indexPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to replace index file: %w", err)
	}

	return nil
}
