package repository

import (
	"context"
	"fmt"
	goos "os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/cperrin88/gotya/pkg/platform"
)

// repositoryManagerImpl implements the RepositoryManager interface.
// It provides methods for managing, syncing, and querying repositories.
type repositoryManagerImpl struct {
	repositories map[string]*repositoryEntry
	syncer       *Syncer
	mu           sync.RWMutex

	// Platform settings
	platformOS   string
	platformArch string
	preferNative bool
}

type repositoryEntry struct {
	info  Info
	index *IndexImpl
}

// NewManager creates a new repository manager.
func NewManager() RepositoryManager {
	return NewManagerWithCacheDir("")
}

// NewManagerWithCacheDir creates a new repository manager with a specific cache directory.
func NewManagerWithCacheDir(cacheDir string) RepositoryManager {
	return NewManagerWithPlatform(cacheDir, "", "", true)
}

// NewManagerWithPlatform creates a new repository manager with platform settings.
func NewManagerWithPlatform(cacheDir, os, arch string, preferNative bool) RepositoryManager {
	if cacheDir == "" {
		userCacheDir, err := goos.UserCacheDir()
		if err != nil {
			cacheDir = "/tmp/gotya-cache"
		} else {
			cacheDir = filepath.Join(userCacheDir, "gotya")
		}
	}

	// If OS/arch are empty, use the current platform
	if os == "" || arch == "" {
		os, arch = platform.Detect()
	}

	return &repositoryManagerImpl{
		repositories: make(map[string]*repositoryEntry),
		syncer:       NewSyncer(cacheDir, 30*time.Second),
		platformOS:   os,
		platformArch: arch,
		preferNative: preferNative,
	}
}

// AddRepository adds a new repository.
func (rm *repositoryManagerImpl) AddRepository(name, url string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if name == "" {
		return fmt.Errorf("repository name cannot be empty")
	}
	if url == "" {
		return fmt.Errorf("repository URL cannot be empty")
	}

	rm.repositories[name] = &repositoryEntry{
		info: Info{
			Name:     name,
			URL:      url,
			Enabled:  true,
			Priority: 0,
		},
	}

	return nil
}

// RemoveRepository removes a repository.
func (rm *repositoryManagerImpl) RemoveRepository(name string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.repositories[name]; !exists {
		return fmt.Errorf("repository '%s' not found", name)
	}

	delete(rm.repositories, name)
	return nil
}

// GetRepositoryIndex gets the cached index for a repository, filtered by the current platform settings.
func (rm *repositoryManagerImpl) GetRepositoryIndex(name string) (*IndexImpl, error) {
	index, err := rm.getRawRepositoryIndex(name)
	if err != nil {
		return nil, err
	}

	// Always filter packages based on the current platform settings
	// The platform settings are either from config or auto-detected
	var filteredPkgs []Package

	// First, try to find exact matches
	filteredPkgs = FilterPackages(index.GetPackages(), rm.platformOS, rm.platformArch)

	// If no exact matches and preferNative is false, try to find packages with "any" platform
	if len(filteredPkgs) == 0 && !rm.preferNative {
		anyPkgs := FilterPackages(index.GetPackages(), "", "")
		if len(anyPkgs) > 0 {
			filteredPkgs = anyPkgs
		}
	}

	// Create a filtered copy of the index
	filteredIndex := &IndexImpl{
		FormatVersion: index.GetFormatVersion(),
		LastUpdate:    index.LastUpdate,
		Packages:      filteredPkgs,
	}

	return filteredIndex, nil
}

// getRawRepositoryIndex gets the raw repository index without platform filtering.
func (rm *repositoryManagerImpl) getRawRepositoryIndex(name string) (*IndexImpl, error) {
	rm.mu.RLock()
	repo, exists := rm.repositories[name]
	rm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("repository '%s' not found", name)
	}

	// If we have a cached index, return it
	if repo.index != nil {
		return repo.index, nil
	}

	// Try to load from cache
	index, err := rm.syncer.LoadCachedIndex(name)
	if err != nil {
		return nil, fmt.Errorf("no cached index available for repository '%s': %w", name, err)
	}

	// Cache the loaded index
	rm.mu.Lock()
	if repoEntry, exists := rm.repositories[name]; exists {
		repoEntry.index = index
	}
	rm.mu.Unlock()

	return index, nil
}

// IsCacheStale checks if a repository's cache is stale.
func (rm *repositoryManagerImpl) IsCacheStale(name string, maxAge time.Duration) bool {
	return rm.syncer.IsCacheStale(name, maxAge)
}

// GetCacheAge returns the age of a repository's cache.
func (rm *repositoryManagerImpl) GetCacheAge(name string) (time.Duration, error) {
	return rm.syncer.GetCacheAge(name)
}

// SyncIfStale syncs a repository only if its cache is stale.
func (rm *repositoryManagerImpl) SyncIfStale(ctx context.Context, name string, maxAge time.Duration) error {
	if rm.IsCacheStale(name, maxAge) {
		return rm.SyncRepository(ctx, name)
	}
	return nil
}

// EnableRepository enables a repository.
func (rm *repositoryManagerImpl) EnableRepository(name string, enabled bool) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	repo, exists := rm.repositories[name]
	if !exists {
		return fmt.Errorf("repository '%s' not found", name)
	}

	repo.info.Enabled = enabled
	return nil
}

// DisableRepository disables a repository.
func (rm *repositoryManagerImpl) DisableRepository(name string) error {
	return rm.EnableRepository(name, false)
}

// ListRepositories returns all repository information.
func (rm *repositoryManagerImpl) ListRepositories() []Info {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	infos := make([]Info, 0, len(rm.repositories))
	for _, repo := range rm.repositories {
		infos = append(infos, repo.info)
	}

	return infos
}

// GetRepository gets repository information by name.
func (rm *repositoryManagerImpl) GetRepository(name string) *Info {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if repo, exists := rm.repositories[name]; exists {
		info := repo.info
		return &info
	}
	return nil
}

// SyncRepository syncs a specific repository.
func (rm *repositoryManagerImpl) SyncRepository(ctx context.Context, name string) error {
	rm.mu.RLock()
	repo, exists := rm.repositories[name]
	rm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("repository '%s' not found", name)
	}

	if !repo.info.Enabled {
		return fmt.Errorf("repository '%s' is disabled", name)
	}

	// Sync the repository and get the updated index
	index, err := rm.syncer.SyncRepository(ctx, repo.info)
	if err != nil {
		return fmt.Errorf("failed to sync repository: %w", err)
	}

	// Update the repository entry with the new index
	rm.mu.Lock()
	if repoEntry, exists := rm.repositories[name]; exists {
		repoEntry.index = index
	}
	rm.mu.Unlock()

	return nil
}

// SyncRepositories syncs all enabled repositories.
func (rm *repositoryManagerImpl) SyncRepositories(ctx context.Context) error {
	repos := rm.ListRepositories()
	for _, repo := range repos {
		if repo.Enabled {
			if err := rm.SyncRepository(ctx, repo.Name); err != nil {
				return fmt.Errorf("failed to sync repository '%s': %w", repo.Name, err)
			}
		}
	}
	return nil
}

// FindPackage finds a package by name across all repositories.
// Returns the first matching package found and any error encountered.
func (rm *repositoryManagerImpl) FindPackage(name string) (*Package, error) {
	if name == "" {
		return nil, fmt.Errorf("package name cannot be empty")
	}

	repos := rm.ListRepositories()
	if len(repos) == 0 {
		return nil, fmt.Errorf("no repositories configured")
	}

	// Search through repositories in order of priority (highest first)
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].Priority > repos[j].Priority
	})

	for _, repo := range repos {
		if !repo.Enabled {
			continue
		}

		index, err := rm.GetRepositoryIndex(repo.Name)
		if err != nil {
			// Log the error but continue with other repositories
			continue
		}

		if pkg := index.FindPackage(name); pkg != nil {
			return pkg, nil
		}
	}

	return nil, fmt.Errorf("package '%s' not found in any repository", name)
}
