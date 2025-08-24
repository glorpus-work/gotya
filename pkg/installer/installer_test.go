package installer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cperrin88/gotya/pkg/config"
	"github.com/cperrin88/gotya/pkg/fsutil"
	"github.com/cperrin88/gotya/pkg/hook"
	"github.com/cperrin88/gotya/pkg/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockHookManager struct {
	hooks map[hook.HookType]bool
}

func (m *mockHookManager) Execute(hookType hook.HookType, ctx hook.HookContext) error {
	return nil
}

func (m *mockHookManager) AddHook(h hook.Hook) error {
	if m.hooks == nil {
		m.hooks = make(map[hook.HookType]bool)
	}
	m.hooks[h.Type] = true
	return nil
}

func (m *mockHookManager) RemoveHook(hookType hook.HookType) error {
	if m.hooks != nil {
		delete(m.hooks, hookType)
	}
	return nil
}

func (m *mockHookManager) HasHook(hookType hook.HookType) bool {
	if m.hooks == nil {
		return false
	}
	return m.hooks[hookType]
}

type mockRepoManager struct{}

func (m *mockRepoManager) FindPackage(name string) (*repository.Package, error) {
	return &repository.Package{
		Name:    "test-package",
		Version: "1.0.0",
		URL:     "http://example.com/test-package.tar.gz",
	}, nil
}

func (m *mockRepoManager) ListPackages() ([]*repository.Package, error) {
	return nil, nil
}

func (m *mockRepoManager) AddRepository(name, url string) error {
	return nil
}

func (m *mockRepoManager) DisableRepository(name string) error {
	return nil
}

func (m *mockRepoManager) RemoveRepository(name string) error {
	return nil // For testing, just return nil
}

func (m *mockRepoManager) EnableRepository(name string, enabled bool) error {
	return nil
}

func (m *mockRepoManager) GetCacheAge(repoName string) (time.Duration, error) {
	return 0, nil
}

func (m *mockRepoManager) GetRepository(name string) *repository.Info {
	return &repository.Info{
		Name:     name,
		URL:      "http://example.com/repo",
		Enabled:  true,
		Priority: 0,
	}
}

func (m *mockRepoManager) GetRepositoryIndex(name string) (*repository.IndexImpl, error) {
	return &repository.IndexImpl{
		FormatVersion: "1.0",
		LastUpdate:    time.Now(),
		Packages:      []repository.Package{},
	}, nil
}

func (m *mockRepoManager) IsCacheStale(name string, maxAge time.Duration) bool {
	return false // For testing, we'll assume cache is never stale
}

func (m *mockRepoManager) ListRepositories() []repository.Info {
	// Return a single test repository for testing
	return []repository.Info{
		{
			Name:     "test-repo",
			URL:      "http://example.com/repo",
			Enabled:  true,
			Priority: 0,
		},
	}
}

func (m *mockRepoManager) SyncIfStale(ctx context.Context, name string, maxAge time.Duration) error {
	// For testing, we'll assume the cache is never stale, so no sync is needed
	return nil
}

func (m *mockRepoManager) SyncRepositories(ctx context.Context) error {
	// For testing, just return nil
	return nil
}

func (m *mockRepoManager) SyncRepository(ctx context.Context, name string) error {
	// For testing, just return nil
	return nil
}

func TestNewInstaller(t *testing.T) {
	cfg := &config.Config{}
	repoMgr := &mockRepoManager{}
	hookMgr := &mockHookManager{}

	// Test successful creation
	inst, err := New(cfg, repoMgr, hookMgr)
	require.NoError(t, err)
	require.NotNil(t, inst)

	// Verify directories are set
	assert.NotEmpty(t, inst.cacheDir)
	assert.NotEmpty(t, inst.installedDir)
	assert.NotEmpty(t, inst.metaDir)

	// Verify directories exist
	assert.DirExists(t, inst.cacheDir)
	assert.DirExists(t, inst.installedDir)
	assert.DirExists(t, inst.metaDir)
}

func TestGetPackageDirs(t *testing.T) {
	// Setup test environment
	cfg := &config.Config{}
	repoMgr := &mockRepoManager{}
	hookMgr := &mockHookManager{}

	inst, err := New(cfg, repoMgr, hookMgr)
	require.NoError(t, err)

	tests := []struct {
		name    string
		pkgName string
		setup   func() string
	}{
		{
			name:    "install dir",
			pkgName: "test-pkg",
			setup: func() string {
				dir, _ := fsutil.GetInstalledDir()
				return filepath.Join(dir, "test-pkg")
			},
		},
		{
			name:    "meta dir",
			pkgName: "test-pkg",
			setup: func() string {
				dir, _ := fsutil.GetMetaDir()
				return filepath.Join(dir, "test-pkg")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer os.RemoveAll(tt.setup())

			var got string
			switch tt.name {
			case "install dir":
				got = inst.getPackageInstallDir(tt.pkgName)
			case "meta dir":
				got = inst.getPackageMetaDir(tt.pkgName)
			}

			// Verify the directory path is correct
			expected := tt.setup()
			assert.Equal(t, expected, got)

			// Verify the directory can be created
			err := os.MkdirAll(got, 0755)
			require.NoError(t, err)
			defer os.RemoveAll(got)

			assert.DirExists(t, got)
		})
	}
}

func TestGetPackageCachePath(t *testing.T) {
	cfg := &config.Config{}
	repoMgr := &mockRepoManager{}
	hookMgr := &mockHookManager{}

	inst, err := New(cfg, repoMgr, hookMgr)
	require.NoError(t, err)

	pkg := &repository.Package{
		Name:    "test-pkg",
		Version: "1.0.0",
		URL:     "http://example.com/test-pkg-1.0.0.tar.gz",
	}

	expected := filepath.Join(inst.cacheDir, "test-pkg-1.0.0.tar.gz")
	assert.Equal(t, expected, inst.getPackageCachePath(pkg))
}
