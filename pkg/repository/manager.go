package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RepositoryManager implements the Manager interface
type RepositoryManager struct {
	repositories map[string]*repositoryEntry
	syncer       *Syncer
	mu           sync.RWMutex
}

type repositoryEntry struct {
	info  Info
	index Index
}

// NewManager creates a new repository manager
func NewManager() *RepositoryManager {
	return NewManagerWithCacheDir("")
}

// NewManagerWithCacheDir creates a new repository manager with a specific cache directory
func NewManagerWithCacheDir(cacheDir string) *RepositoryManager {
	if cacheDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			cacheDir = "/tmp/gotya-cache"
		} else {
			cacheDir = filepath.Join(homeDir, ".cache", "gotya")
		}
	}

	return &RepositoryManager{
		repositories: make(map[string]*repositoryEntry),
		syncer:       NewSyncer(cacheDir, 30*time.Second),
	}
}

// AddRepository adds a new repository
func (rm *RepositoryManager) AddRepository(name, url string) error {
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

// RemoveRepository removes a repository
func (rm *RepositoryManager) RemoveRepository(name string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.repositories[name]; !exists {
		return fmt.Errorf("repository '%s' not found", name)
	}

	delete(rm.repositories, name)
	return nil
}

// GetRepositoryIndex gets the cached index for a repository
func (rm *RepositoryManager) GetRepositoryIndex(name string) (Index, error) {
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

// IsCacheStale checks if a repository's cache is stale
func (rm *RepositoryManager) IsCacheStale(name string, maxAge time.Duration) bool {
	return rm.syncer.IsCacheStale(name, maxAge)
}

// GetCacheAge returns the age of a repository's cache
func (rm *RepositoryManager) GetCacheAge(name string) (time.Duration, error) {
	return rm.syncer.GetCacheAge(name)
}

// SyncIfStale syncs a repository only if its cache is stale
func (rm *RepositoryManager) SyncIfStale(ctx context.Context, name string, maxAge time.Duration) error {
	if rm.IsCacheStale(name, maxAge) {
		return rm.SyncRepository(ctx, name)
	}
	return nil
}

// EnableRepository enables a repository
func (rm *RepositoryManager) EnableRepository(name string, enabled bool) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	repo, exists := rm.repositories[name]
	if !exists {
		return fmt.Errorf("repository '%s' not found", name)
	}

	repo.info.Enabled = enabled
	return nil
}

// DisableRepository disables a repository
func (rm *RepositoryManager) DisableRepository(name string) error {
	return rm.EnableRepository(name, false)
}

// ListRepositories returns all repository information
func (rm *RepositoryManager) ListRepositories() []Info {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	infos := make([]Info, 0, len(rm.repositories))
	for _, repo := range rm.repositories {
		infos = append(infos, repo.info)
	}

	return infos
}

// GetRepository gets repository information by name
func (rm *RepositoryManager) GetRepository(name string) *Info {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if repo, exists := rm.repositories[name]; exists {
		info := repo.info
		return &info
	}
	return nil
}

// SyncRepository syncs a specific repository
func (rm *RepositoryManager) SyncRepository(ctx context.Context, name string) error {
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

// SyncRepositories syncs all enabled repositories
func (rm *RepositoryManager) SyncRepositories(ctx context.Context) error {
	rm.mu.RLock()
	var enabledRepos []string
	for name, repo := range rm.repositories {
		if repo.info.Enabled {
			enabledRepos = append(enabledRepos, name)
		}
	}
	rm.mu.RUnlock()

	for _, name := range enabledRepos {
		if err := rm.SyncRepository(ctx, name); err != nil {
			return fmt.Errorf("failed to sync repository '%s': %w", name, err)
		}
	}

	return nil
}
