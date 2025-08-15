package repo

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	pkg "github.com/cperrin88/gotya/pkg/package"
)

// Repository represents a single repository with its URL and cached index
type Repository struct {
	Name      string
	URL       string
	Index     *Index
	LastSync  time.Time
	Enabled   bool
	Priority  int // Add this field
	CachePath string
}

// Manager manages multiple repository indexes
type Manager struct {
	repositories map[string]*Repository
	httpClient   *http.Client
	cacheDir     string
	installedDB  *pkg.InstalledDatabase
	installDir   string
	mutex        sync.RWMutex
}

// NewManager creates a new repository manager with default cache directory
func NewManager() (*Manager, error) {
	cacheDir, err := getDefaultCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine cache directory: %w", err)
	}

	return NewManagerWithCacheDir(cacheDir)
}

// NewManagerWithCacheDir creates a new repository manager with specified cache directory
func NewManagerWithCacheDir(cacheDir string) (*Manager, error) {
	// Ensure cache directory exists
	indexDir := filepath.Join(cacheDir, "index")
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Initialize package database
	dbPath := filepath.Join(cacheDir, "packages.db")
	installedDB, err := pkg.LoadInstalledDatabase(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize package database: %w", err)
	}

	// Default install directory (can be overridden)
	installDir := "/usr/local"
	if runtime.GOOS == "windows" {
		installDir = "C:\\Program Files\\gotya"
	}

	manager := &Manager{
		repositories: make(map[string]*Repository),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cacheDir:    indexDir,
		installedDB: installedDB,
		installDir:  installDir,
	}

	// Load existing cached indexes
	if err := manager.loadCachedIndexes(); err != nil {
		return nil, fmt.Errorf("failed to load cached indexes: %w", err)
	}

	return manager, nil
}

// NewManagerWithClient creates a new repository manager with a custom HTTP client
func NewManagerWithClient(client *http.Client, cacheDir string) (*Manager, error) {
	indexDir := filepath.Join(cacheDir, "index")
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Initialize package database
	dbPath := filepath.Join(cacheDir, "packages.db")
	installedDB, err := pkg.LoadInstalledDatabase(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize package database: %w", err)
	}

	// Default install directory
	installDir := "/usr/local"
	if runtime.GOOS == "windows" {
		installDir = "C:\\Program Files\\gotya"
	}

	manager := &Manager{
		repositories: make(map[string]*Repository),
		httpClient:   client,
		cacheDir:     indexDir,
		installedDB:  installedDB,
		installDir:   installDir,
	}

	if err := manager.loadCachedIndexes(); err != nil {
		return nil, fmt.Errorf("failed to load cached indexes: %w", err)
	}

	return manager, nil
}

// getDefaultCacheDir returns the default cache directory based on the OS
func getDefaultCacheDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "windows":
		// Use AppData/Local on Windows
		return filepath.Join(homeDir, "AppData", "Local", "gotya"), nil
	case "darwin":
		// Use Library/Caches on macOS
		return filepath.Join(homeDir, "Library", "Caches", "gotya"), nil
	default:
		// Use .cache on Linux and other Unix-like systems
		return filepath.Join(homeDir, ".cache", "gotya"), nil
	}
}

// loadCachedIndexes loads all cached index files from disk
func (m *Manager) loadCachedIndexes() error {
	entries, err := os.ReadDir(m.cacheDir)
	if os.IsNotExist(err) {
		return nil // Cache directory doesn't exist yet, that's fine
	}
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_index.json") {
			continue
		}

		// Extract repository name from filename
		repoName := strings.TrimSuffix(entry.Name(), "_index.json")
		cachePath := filepath.Join(m.cacheDir, entry.Name())

		// Load the cached index
		index, lastSync, err := m.loadIndexFromCache(cachePath)
		if err != nil {
			// Log error but continue loading other indexes
			continue
		}

		// Create repository entry (URL will be empty until explicitly added)
		m.repositories[repoName] = &Repository{
			Name:      repoName,
			Index:     index,
			LastSync:  lastSync,
			Enabled:   true,
			CachePath: cachePath,
		}
	}

	return nil
}

// loadIndexFromCache loads an index from a cache file
func (m *Manager) loadIndexFromCache(cachePath string) (*Index, time.Time, error) {
	file, err := os.Open(cachePath)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer file.Close()

	index, err := ParseIndexFromReader(file)
	if err != nil {
		return nil, time.Time{}, err
	}

	// Get file modification time as last sync time
	stat, err := file.Stat()
	if err != nil {
		return nil, time.Time{}, err
	}

	return index, stat.ModTime(), nil
}

// saveIndexToCache saves an index to the cache file
func (m *Manager) saveIndexToCache(repoName string, index *Index) error {
	filename := fmt.Sprintf("%s_index.json", sanitizeFilename(repoName))
	cachePath := filepath.Join(m.cacheDir, filename)

	// Create temporary file first
	tempPath := cachePath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp cache file: %w", err)
	}

	// Write index to temporary file
	indexData, err := index.ToJSON()
	if err != nil {
		file.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to serialize index: %w", err)
	}

	if _, err := file.Write(indexData); err != nil {
		file.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	file.Close()

	// Atomically replace the cache file
	if err := os.Rename(tempPath, cachePath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to replace cache file: %w", err)
	}

	return nil
}

// sanitizeFilename removes invalid characters from repository name for filename
func sanitizeFilename(name string) string {
	// Replace invalid filename characters with underscores
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", " "}
	result := name
	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "_")
	}
	return result
}

// AddRepository adds a new repository to the manager
func (m *Manager) AddRepository(name, url string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	filename := fmt.Sprintf("%s_index.json", sanitizeFilename(name))
	cachePath := filepath.Join(m.cacheDir, filename)

	// Check if we already have this repo cached
	if existing, exists := m.repositories[name]; exists {
		existing.URL = url
		existing.CachePath = cachePath
		return
	}

	m.repositories[name] = &Repository{
		Name:      name,
		URL:       url,
		Enabled:   true,
		CachePath: cachePath,
	}
}

// RemoveRepository removes a repository from the manager and deletes its cache
func (m *Manager) RemoveRepository(name string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	repo, exists := m.repositories[name]
	if !exists {
		return false
	}

	// Delete cache file if it exists
	if repo.CachePath != "" {
		os.Remove(repo.CachePath)
	}

	delete(m.repositories, name)
	return true
}

// EnableRepository enables or disables a repository
func (m *Manager) EnableRepository(name string, enabled bool) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	repo, exists := m.repositories[name]
	if !exists {
		return fmt.Errorf("repository '%s' not found", name)
	}

	repo.Enabled = enabled
	return nil
}

// GetRepository returns a repository by name
func (m *Manager) GetRepository(name string) (*Repository, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	repo, exists := m.repositories[name]
	return repo, exists
}

// ListRepositories returns all repositories
func (m *Manager) ListRepositories() []*Repository {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	repos := make([]*Repository, 0, len(m.repositories))
	for _, repo := range m.repositories {
		repos = append(repos, repo)
	}
	return repos
}

// SyncRepository fetches and updates the index for a specific repository
func (m *Manager) SyncRepository(ctx context.Context, name string) error {
	m.mutex.RLock()
	repo, exists := m.repositories[name]
	if !exists {
		m.mutex.RUnlock()
		return fmt.Errorf("repository '%s' not found", name)
	}

	if !repo.Enabled {
		m.mutex.RUnlock()
		return fmt.Errorf("repository '%s' is disabled", name)
	}

	if repo.URL == "" {
		m.mutex.RUnlock()
		return fmt.Errorf("repository '%s' has no URL configured", name)
	}

	url := repo.URL
	m.mutex.RUnlock()

	// Fetch index from URL
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request for '%s': %w", name, err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch index from '%s': %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch index from '%s': HTTP %d", url, resp.StatusCode)
	}

	// Parse the index
	index, err := ParseIndexFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to parse index from '%s': %w", name, err)
	}

	// Save to cache
	if err := m.saveIndexToCache(name, index); err != nil {
		return fmt.Errorf("failed to cache index for '%s': %w", name, err)
	}

	// Update repository with new index
	m.mutex.Lock()
	defer m.mutex.Unlock()

	repo.Index = index
	repo.LastSync = time.Now()

	return nil
}

// SyncAllRepositories syncs all enabled repositories
func (m *Manager) SyncAllRepositories(ctx context.Context) map[string]error {
	m.mutex.RLock()
	enabledRepos := make([]string, 0)
	for name, repo := range m.repositories {
		if repo.Enabled && repo.URL != "" {
			enabledRepos = append(enabledRepos, name)
		}
	}
	m.mutex.RUnlock()

	errors := make(map[string]error)
	var wg sync.WaitGroup

	for _, name := range enabledRepos {
		wg.Add(1)
		go func(repoName string) {
			defer wg.Done()
			if err := m.SyncRepository(ctx, repoName); err != nil {
				errors[repoName] = err
			}
		}(name)
	}

	wg.Wait()
	return errors
}

// GetCacheDir returns the cache directory path
func (m *Manager) GetCacheDir() string {
	return m.cacheDir
}

// GetCacheSize returns the total size of cached index files
func (m *Manager) GetCacheSize() (int64, error) {
	var totalSize int64

	entries, err := os.ReadDir(m.cacheDir)
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), "_index.json") {
			info, err := entry.Info()
			if err == nil {
				totalSize += info.Size()
			}
		}
	}

	return totalSize, nil
}

// ClearCache removes all cached index files
func (m *Manager) ClearCache() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	entries, err := os.ReadDir(m.cacheDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), "_index.json") {
			cachePath := filepath.Join(m.cacheDir, entry.Name())
			if err := os.Remove(cachePath); err != nil {
				return fmt.Errorf("failed to remove cache file %s: %w", cachePath, err)
			}
		}
	}

	// Clear in-memory indexes but keep repository configurations
	for _, repo := range m.repositories {
		repo.Index = nil
		repo.LastSync = time.Time{}
	}

	return nil
}

// IsOfflineMode returns true if working with cached data only
func (m *Manager) IsOfflineMode() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, repo := range m.repositories {
		if repo.Enabled && repo.URL != "" {
			return false
		}
	}
	return true
}

// FindPackage searches for a package across all enabled repositories
func (m *Manager) FindPackage(name string) []*PackageResult {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var results []*PackageResult
	for repoName, repo := range m.repositories {
		if !repo.Enabled || repo.Index == nil {
			continue
		}

		if pkg := repo.Index.FindPackage(name); pkg != nil {
			results = append(results, &PackageResult{
				Package:        *pkg,
				RepositoryName: repoName,
				RepositoryURL:  repo.URL,
			})
		}
	}

	return results
}

// GetAllPackages returns all packages from all enabled repositories
func (m *Manager) GetAllPackages() []*PackageResult {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var results []*PackageResult
	for repoName, repo := range m.repositories {
		if !repo.Enabled || repo.Index == nil {
			continue
		}

		for _, pkg := range repo.Index.Packages {
			results = append(results, &PackageResult{
				Package:        pkg,
				RepositoryName: repoName,
				RepositoryURL:  repo.URL,
			})
		}
	}

	return results
}

// GetStats returns statistics about the repositories
func (m *Manager) GetStats() ManagerStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := ManagerStats{
		TotalRepositories: len(m.repositories),
		CacheDir:          m.cacheDir,
	}

	for _, repo := range m.repositories {
		if repo.Enabled {
			stats.EnabledRepositories++
		}
		if repo.Index != nil {
			stats.SyncedRepositories++
			stats.TotalPackages += len(repo.Index.Packages)
		}
	}

	// Get cache size
	if size, err := m.GetCacheSize(); err == nil {
		stats.CacheSize = size
	}

	return stats
}

// PackageResult represents a package found in a specific repository
type PackageResult struct {
	Package        Package
	RepositoryName string
	RepositoryURL  string
}

// ManagerStats contains statistics about the repository manager
type ManagerStats struct {
	TotalRepositories   int
	EnabledRepositories int
	SyncedRepositories  int
	TotalPackages       int
	CacheDir            string
	CacheSize           int64
}

// containsIgnoreCase performs case-insensitive substring search
func containsIgnoreCase(str, substr string) bool {
	return stringContains(stringToLower(str), stringToLower(substr))
}

// Helper functions for string operations
func stringToLower(s string) string {
	result := make([]byte, len(s))
	for i, b := range []byte(s) {
		if b >= 'A' && b <= 'Z' {
			result[i] = b + 32
		} else {
			result[i] = b
		}
	}
	return string(result)
}

func stringContains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Add these missing methods to the Manager struct

// SearchPackages searches for packageontes with more advanced options
func (m *Manager) SearchPackages(query string, exactMatch bool, limit int) ([]*PackageResult, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var results []*PackageResult
	count := 0

	for _, repo := range m.repositories {
		if !repo.Enabled || repo.Index == nil {
			continue
		}

		for _, pkg := range repo.Index.Packages {
			if count >= limit {
				break
			}

			var matches bool
			if exactMatch {
				matches = pkg.Name == query
			} else {
				matches = strings.Contains(strings.ToLower(pkg.Name), strings.ToLower(query)) ||
					strings.Contains(strings.ToLower(pkg.Description), strings.ToLower(query))
			}

			if matches {
				results = append(results, &PackageResult{
					Package:        pkg,
					RepositoryName: repo.Name,
				})
				count++
			}
		}
	}

	return results, nil
}

// GetInstalledPackages returns all installed packages
func (m *Manager) GetInstalledPackages() ([]*Package, error) {
	// This would typically read from a local database or file system
	// For now, return empty slice
	return []*Package{}, nil
}

// GetAvailablePackages returns all available packages from all repositories
func (m *Manager) GetAvailablePackages() ([]*Package, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var packages []*Package
	for _, repo := range m.repositories {
		if !repo.Enabled || repo.Index == nil {
			continue
		}

		for _, pkg := range repo.Index.Packages {
			packages = append(packages, &pkg)
		}
	}

	return packages, nil
}

// IsPackageInstalled checks if a package is installed
func (m *Manager) IsPackageInstalled(name string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.installedDB == nil {
		return false
	}
	return m.installedDB.IsPackageInstalled(name)
}

// InstallPackage installs a package
func (m *Manager) InstallPackage(ctx context.Context, name string, force, resolveDeps bool) error {
	// Find package in repositories
	searchResults, err := m.SearchPackages(name, true, 1)
	if err != nil {
		return fmt.Errorf("failed to search for package '%s': %w", name, err)
	}

	if len(searchResults) == 0 {
		return fmt.Errorf("package '%s' not found in any repository", name)
	}

	packageResult := searchResults[0]
	pkg := &packageResult.Package

	// Check if already installed
	if m.IsPackageInstalled(name) && !force {
		return fmt.Errorf("package '%s' is already installed (use --force to reinstall)", name)
	}

	// Download package
	packagePath, err := m.downloadPackage(ctx, pkg)
	if err != nil {
		return fmt.Errorf("failed to download package: %w", err)
	}
	defer os.Remove(packagePath) // Clean up downloaded file

	// Create temporary extraction directory
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("gotya-install-%s-", pkg.Name))
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract package
	structure, err := pkg.ExtractPackage(packagePath, tempDir)
	if err != nil {
		return fmt.Errorf("failed to extract package: %w", err)
	}

	// Validate metadata matches what we expect
	if structure.Metadata.Name != pkg.Name {
		return fmt.Errorf("package metadata mismatch: expected '%s', got '%s'", pkg.Name, structure.Metadata.Name)
	}

	// Resolve dependencies if requested
	if resolveDeps {
		if err := m.resolveDependencies(ctx, structure.Metadata.Dependencies, force); err != nil {
			return fmt.Errorf("failed to resolve dependencies: %w", err)
		}
	}

	// Execute pre-install script
	if structure.ScriptsDir != "" {
		preInstallScript := filepath.Join(structure.ScriptsDir, "pre-install")
		env := []string{
			fmt.Sprintf("PACKAGE_NAME=%s", pkg.Name),
			fmt.Sprintf("PACKAGE_VERSION=%s", pkg.Version),
		}
		if err := pkg.ExecuteScript(preInstallScript, env); err != nil {
			return fmt.Errorf("pre-install script failed: %w", err)
		}
	}

	// Install files
	var installedFiles []string
	if structure.FilesDir != "" {
		installedFiles, err = pkg.CopyFiles(structure.FilesDir, m.installDir)
		if err != nil {
			return fmt.Errorf("failed to install files: %w", err)
		}
	}

	// Update installed packages database
	installedPkg := pkg.InstalledPackage{
		Name:          pkg.Name,
		Version:       pkg.Version,
		Description:   pkg.Description,
		InstalledAt:   time.Now(),
		InstalledFrom: fmt.Sprintf("repository:%s", packageResult.RepositoryName),
		Files:         installedFiles,
		Checksum:      pkg.Checksum,
	}

	m.mutex.Lock()
	m.installedDB.AddPackage(installedPkg)
	if err := m.installedDB.Save(m.getInstalledDBPath()); err != nil {
		m.mutex.Unlock()
		// Try to rollback file installation
		pkg.RemoveFiles(installedFiles)
		return fmt.Errorf("failed to update installed packages database: %w", err)
	}
	m.mutex.Unlock()

	// Execute post-install script
	if structure.ScriptsDir != "" {
		postInstallScript := filepath.Join(structure.ScriptsDir, "post-install")
		env := []string{
			fmt.Sprintf("PACKAGE_NAME=%s", pkg.Name),
			fmt.Sprintf("PACKAGE_VERSION=%s", pkg.Version),
		}
		if err := pkg.ExecuteScript(postInstallScript, env); err != nil {
			// Post-install failure is not fatal, but we should warn
			fmt.Printf("Warning: post-install script failed: %v\n", err)
		}
	}

	return nil
}

// UpdatePackage updates a specific package
func (m *Manager) UpdatePackage(ctx context.Context, name string) error {
	// Implementation would update the specific package
	return fmt.Errorf("update functionality not yet implemented")
}

// UpdateAllPackages updates all installed packages
func (m *Manager) UpdateAllPackages(ctx context.Context) error {
	// Implementation would update all packages
	return fmt.Errorf("update all functionality not yet implemented")
}

// SyncRepositories syncs all enabled repositories
func (m *Manager) SyncRepositories(ctx context.Context) error {
	m.mutex.RLock()
	repos := make([]*Repository, 0, len(m.repositories))
	for _, repo := range m.repositories {
		if repo.Enabled && repo.URL != "" {
			repos = append(repos, repo)
		}
	}
	m.mutex.RUnlock()

	for _, repo := range repos {
		if err := m.SyncRepository(ctx, repo.Name); err != nil {
			return fmt.Errorf("failed to sync repository %s: %w", repo.Name, err)
		}
	}

	return nil
}

// generateRepoNameFromURL creates a repository name from URL
func generateRepoNameFromURL(url string) string {
	// Simple implementation - extract domain or use hash
	parts := strings.Split(url, "/")
	if len(parts) >= 3 {
		return strings.ReplaceAll(parts[2], ".", "-")
	}
	return "repo-" + fmt.Sprintf("%x", url)[:8]
}

// getInstalledDBPath returns the path to the installed packages database
func (m *Manager) getInstalledDBPath() string {
	return filepath.Join(filepath.Dir(m.cacheDir), "packages.db")
}

// downloadPackage downloads a package to a temporary file and verifies checksum
func (m *Manager) downloadPackage(ctx context.Context, pkg *Package) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pkg.URL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download package: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	// Create temporary file
	tempFile, err := os.CreateTemp(filepath.Dir(m.cacheDir), "package_*.tar.xz")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer tempFile.Close()

	// Download and verify checksum
	hasher := sha256.New()
	writer := io.MultiWriter(tempFile, hasher)

	if _, err := io.Copy(writer, resp.Body); err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to download package data: %w", err)
	}

	// Verify checksum
	calculatedChecksum := fmt.Sprintf("%x", hasher.Sum(nil))
	if pkg.Checksum != "" && calculatedChecksum != pkg.Checksum {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("checksum verification failed: expected %s, got %s", pkg.Checksum, calculatedChecksum)
	}

	return tempFile.Name(), nil
}

// resolveDependencies resolves and installs package dependencies
func (m *Manager) resolveDependencies(ctx context.Context, dependencies []string, force bool) error {
	for _, dep := range dependencies {
		if !m.IsPackageInstalled(dep) {
			if err := m.InstallPackage(ctx, dep, force, true); err != nil {
				return fmt.Errorf("failed to install dependency %s: %w", dep, err)
			}
		}
	}
	return nil
}

// RemovePackage removes an installed package
func (m *Manager) RemovePackage(ctx context.Context, name string) error {
	// Check if package is installed
	installedPkg := m.installedDB.FindPackage(name)
	if installedPkg == nil {
		return fmt.Errorf("package '%s' is not installed", name)
	}

	// Execute pre-remove script if it exists
	// This would typically be stored in the package metadata or a scripts cache

	// Remove files (in reverse order)
	for i := len(installedPkg.Files) - 1; i >= 0; i-- {
		file := installedPkg.Files[i]
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove file %s: %w", file, err)
		}
	}

	// Remove from database
	if !m.installedDB.RemovePackage(name) {
		return fmt.Errorf("failed to remove package from database")
	}

	if err := m.installedDB.Save(m.getInstalledDBPath()); err != nil {
		return fmt.Errorf("failed to save database after removal: %w", err)
	}

	return nil
}

// SetInstallDir sets the installation directory
func (m *Manager) SetInstallDir(installDir string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.installDir = installDir
}

// GetInstallDir returns the current installation directory
func (m *Manager) GetInstallDir() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.installDir
}
