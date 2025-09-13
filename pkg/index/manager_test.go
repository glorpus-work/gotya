package index

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/fsutil"
	mockHttp "github.com/cperrin88/gotya/pkg/http/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestManager_Sync(t *testing.T) {
	tests := []struct {
		name        string
		repoName    string
		repos       []*Repository
		setupMocks  func(*mockHttp.MockClient, string)
		expectErr   bool
		expectPath  string
		expectErrAs string
	}{
		{
			name:     "successful sync",
			repoName: "test-repo",
			repos: []*Repository{
				{
					Name:     "test-repo",
					URL:      &url.URL{Scheme: "https", Host: "example.com", Path: "/"},
					Priority: 1,
					Enabled:  true,
				},
			},
			setupMocks: func(mockClient *mockHttp.MockClient, indexPath string) {
				expectedURL, err := url.Parse("https://example.com/")
				if err != nil {
					panic(fmt.Sprintf("failed to parse test URL: %v", err))
				}
				mockClient.EXPECT().
					DownloadIndex(gomock.Any(), expectedURL, gomock.Any()).
					Return(nil)
			},
			expectPath: filepath.Join("indexes", "test-repo.yaml"),
		},
		{
			name:     "repository not found",
			repoName: "nonexistent",
			repos: []*Repository{
				{
					Name:     "test-repo",
					URL:      &url.URL{Scheme: "https", Host: "example.com", Path: "/"},
					Priority: 1,
					Enabled:  true,
				},
			},
			setupMocks:  func(*mockHttp.MockClient, string) {},
			expectErr:   true,
			expectErrAs: errors.ErrRepositoryNotFound("nonexistent").Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mockHttp.NewMockClient(ctrl)
			if tt.setupMocks != nil {
				tt.setupMocks(mockClient, tt.expectPath)
			}

			tempDir := t.TempDir()
			manager := &ManagerImpl{
				httpClient:   mockClient,
				repositories: tt.repos,
				indexPath:    filepath.Join(tempDir, "indexes"),
				cacheTTL:     time.Hour,
			}

			err := manager.Sync(context.Background(), tt.repoName)
			if tt.expectErr {
				assert.Error(t, err)
				if tt.expectErrAs != "" {
					assert.Contains(t, err.Error(), tt.expectErrAs)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestManager_IsCacheStale(t *testing.T) {
	// Create a test repository
	repo := &Repository{
		Name:     "test-repo",
		URL:      &url.URL{Scheme: "https", Host: "example.com", Path: "/index.json"},
		Priority: 1,
		Enabled:  true,
	}

	tests := []struct {
		name        string
		repoName    string
		setupFiles  func(string) error
		cacheTTL    time.Duration
		expectStale bool
		expectErr   bool
	}{
		{
			name:     "cache is fresh",
			repoName: "test-repo",
			setupFiles: func(basePath string) error {
				// Create a fresh index file
				createTestIndexFile(t, basePath, "test-repo")
				return nil
			},
			cacheTTL:    time.Hour,
			expectStale: false,
		},
		{
			name:     "cache is stale",
			repoName: "test-repo",
			setupFiles: func(basePath string) error {
				// Create a stale index file
				indexPath := filepath.Join(basePath, "indexes", "test-repo.json")
				if err := os.MkdirAll(filepath.Dir(indexPath), fsutil.DirModeDefault); err != nil {
					return err
				}
				if err := os.WriteFile(indexPath, []byte("{}"), fsutil.FileModeDefault); err != nil {
					return err
				}
				// Set modification time to 2 hours ago
				oldTime := time.Now().Add(-2 * time.Hour)
				return os.Chtimes(indexPath, oldTime, oldTime)
			},
			cacheTTL:    time.Hour,
			expectStale: true,
		},
		{
			name:     "cache file does not exist",
			repoName: "nonexistent",
			setupFiles: func(string) error {
				return nil // No files created
			},
			cacheTTL:    time.Hour,
			expectStale: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			if tt.setupFiles != nil {
				require.NoError(t, tt.setupFiles(tempDir))
			}

			manager := &ManagerImpl{
				httpClient:   nil, // Not used in this test
				repositories: []*Repository{repo},
				indexPath:    filepath.Join(tempDir, "indexes"),
				cacheTTL:     tt.cacheTTL,
				indexes:      make(map[string]*Index),
			}

			isStale := manager.IsCacheStale(tt.repoName)
			assert.Equal(t, tt.expectStale, isStale)
		})
	}
}

func createTestIndexFile(t *testing.T, dir, repoName string) string {
	indexPath := filepath.Join(dir, "indexes", repoName+".json")
	require.NoError(t, os.MkdirAll(filepath.Dir(indexPath), fsutil.DirModeDefault))

	// Create a test index file
	testIndex := `{
  "format_version": "1.0",
  "last_updated": "2024-08-16T10:00:00Z",
  "packages": [
    {
      "name": "test-artifact",
      "version": "1.0.0",
      "description": "A simple test package for testing",
      "author": "Test Author",
      "license": "MIT",
      "homepage": "https://example.com/hello-world",
      "dependencies": [],
      "keywords": ["test", "hello", "example"],
      "architecture": "any",
      "url": "http://localhost:63342/gotya/test/repo/packages/test-package_1.0.0_any_any.tar.gz",
      "checksum": "abc123def456",
      "created_at": "2024-08-16T09:00:00Z"
    }
  ]
}`

	require.NoError(t, os.WriteFile(indexPath, []byte(testIndex), fsutil.FileModeDefault))
	return indexPath
}

func TestManager_ResolveArtifact(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tempDir := t.TempDir()
	repoName := "test-repo"
	indexFile := createTestIndexFile(t, tempDir, repoName)

	repo := &Repository{
		Name:     "test-repo",
		URL:      &url.URL{Scheme: "https", Host: "example.com", Path: "/"},
		Priority: 1,
		Enabled:  true,
	}

	tests := []struct {
		name        string
		pkgName     string
		version     string
		os          string
		arch        string
		expectVer   string
		expectErr   bool
		expectErrAs error
	}{
		{
			name:      "find latest version",
			pkgName:   "test-artifact",
			version:   ">= 0.0.0",
			os:        "linux",
			arch:      "amd64",
			expectVer: "1.0.0",
		},
		{
			name:      "find specific version",
			pkgName:   "test-artifact",
			version:   "1.0.0",
			os:        "darwin",
			arch:      "amd64",
			expectVer: "1.0.0",
		},
		{
			name:        "package not found",
			pkgName:     "nonexistent",
			version:     "",
			os:          "linux",
			arch:        "amd64",
			expectErr:   true,
			expectErrAs: errors.ErrArtifactNotFound,
		},
	}

	for _, testExec := range tests {
		t.Run(testExec.name, func(t *testing.T) {
			manager := &ManagerImpl{
				httpClient:   nil, // Not used in this test
				repositories: []*Repository{repo},
				indexPath:    filepath.Dir(indexFile),
				cacheTTL:     time.Hour,
				indexes:      make(map[string]*Index, 1),
			}

			pkg, err := manager.ResolveArtifact(testExec.pkgName, testExec.version, testExec.os, testExec.arch)
			if testExec.expectErr {
				assert.Error(t, err)
				if testExec.expectErrAs != nil {
					assert.ErrorContains(t, err, testExec.expectErrAs.Error())
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, testExec.pkgName, pkg.Name)
			assert.Equal(t, testExec.expectVer, pkg.Version)
		})
	}
}

func TestManager_GetCacheAge(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test repository
	repo := &Repository{
		Name:     "test-repo",
		URL:      &url.URL{Scheme: "https", Host: "example.com", Path: "/index.json"},
		Priority: 1,
		Enabled:  true,
	}

	// Create a test index file
	indexPath := createTestIndexFile(t, tempDir, "test-repo")

	tests := []struct {
		name      string
		repoName  string
		setup     func()
		expectErr bool
	}{
		{
			name:     "get cache age",
			repoName: "test-repo",
			setup:    func() {},
		},
		{
			name:      "repository not found",
			repoName:  "nonexistent",
			setup:     func() {},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			manager := &ManagerImpl{
				httpClient:   nil, // Not used in this test
				repositories: []*Repository{repo},
				indexPath:    filepath.Dir(indexPath),
				cacheTTL:     time.Hour,
			}

			age, err := manager.GetCacheAge(tt.repoName)
			if tt.expectErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.True(t, age >= 0)
		})
	}
}

func TestManager_SyncAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tempDir := t.TempDir()

	repo1URL, err := url.Parse("https://example.com/repo1/index.json")
	if err != nil {
		t.Fatalf("failed to parse repo1 URL: %v", err)
	}
	repo2URL, err := url.Parse("https://example.com/repo2/index.json")
	if err != nil {
		t.Fatalf("failed to parse repo2 URL: %v", err)
	}

	repos := []*Repository{
		{
			Name:     "repo1",
			URL:      repo1URL,
			Priority: 1,
			Enabled:  true,
		},
		{
			Name:     "repo2",
			URL:      repo2URL,
			Priority: 2,
			Enabled:  true,
		},
	}

	tests := []struct {
		name       string
		setupMocks func(*mockHttp.MockClient)
		expectErr  bool
	}{
		{
			name: "successful sync all",
			setupMocks: func(mockClient *mockHttp.MockClient) {
				mockClient.EXPECT().
					DownloadIndex(gomock.Any(), repo1URL, gomock.Any()).
					Return(nil)
				mockClient.EXPECT().
					DownloadIndex(gomock.Any(), repo2URL, gomock.Any()).
					Return(nil)
			},
		},
		{
			name: "error during sync",
			setupMocks: func(mockClient *mockHttp.MockClient) {
				mockClient.EXPECT().
					DownloadIndex(gomock.Any(), repo1URL, gomock.Any()).
					Return(nil)
				mockClient.EXPECT().
					DownloadIndex(gomock.Any(), repo2URL, gomock.Any()).
					Return(errors.ErrDownloadFailed)
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := mockHttp.NewMockClient(ctrl)
			if tt.setupMocks != nil {
				tt.setupMocks(mockClient)
			}

			manager := &ManagerImpl{
				httpClient:   mockClient,
				repositories: repos,
				indexPath:    tempDir,
				cacheTTL:     time.Hour,
			}

			err := manager.SyncAll(context.Background())
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
