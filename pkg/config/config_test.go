package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cperrin88/gotya/pkg/fsutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Test default values
	assert.Equal(t, "info", cfg.Settings.LogLevel)
	assert.Equal(t, 30*time.Second, cfg.Settings.HTTPTimeout)
	assert.Equal(t, 5, cfg.Settings.MaxConcurrent)
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `repositories:
  - name: test-repo
    url: https://example.com/repo
    enabled: true
settings:
  log_level: debug
  platform:
    os: linux
    arch: amd64
    prefer_native: true`

	err := os.WriteFile(configPath, []byte(configContent), fsutil.FileModeDefault)
	require.NoError(t, err)

	// Test loading the config
	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify loaded values
	assert.Len(t, cfg.Repositories, 1)
	assert.Equal(t, "test-repo", cfg.Repositories[0].Name)
	assert.Equal(t, "debug", cfg.Settings.LogLevel)
	assert.Equal(t, "linux", cfg.Settings.Platform.OS)
	assert.Equal(t, "amd64", cfg.Settings.Platform.Arch)
	assert.True(t, cfg.Settings.Platform.PreferNative)
}

func TestSaveConfig(t *testing.T) {
	// Create a test config
	cfg := DefaultConfig()
	cfg.Settings.LogLevel = "debug"
	cfg.Settings.Platform.OS = "linux"
	cfg.Settings.Platform.Arch = "amd64"

	// Save to a temporary file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")

	err := cfg.SaveConfig(configPath)
	require.NoError(t, err)

	// Verify the file exists and has content
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.True(t, len(data) > 0)

	// Load it back and verify
	loadedCfg, err := LoadConfig(configPath)
	require.NoError(t, err)
	require.NotNil(t, loadedCfg)

	assert.Equal(t, "debug", loadedCfg.Settings.LogLevel)
	assert.Equal(t, "linux", loadedCfg.Settings.Platform.OS)
	assert.Equal(t, "amd64", loadedCfg.Settings.Platform.Arch)
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "invalid OS",
			config: &Config{
				Settings: Settings{
					Platform: PlatformConfig{
						OS:   "invalid-os",
						Arch: "amd64",
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid OS",
		},
		{
			name: "invalid Arch",
			config: &Config{
				Settings: Settings{
					Platform: PlatformConfig{
						OS:   "linux",
						Arch: "invalid-arch",
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid architecture",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGetUserDataDir(t *testing.T) {
	// Save original environment and restore after test
	originalHome := os.Getenv("HOME")
	originalXDGDataHome := os.Getenv("XDG_DATA_HOME")
	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("XDG_DATA_HOME", originalXDGDataHome)
	}()

	tests := []struct {
		name     string
		setup    func()
		wantPath string
		wantErr  bool
	}{
		{
			name: "XDG_DATA_HOME set",
			setup: func() {
				t.Setenv("XDG_DATA_HOME", "/custom/data/home")
			},
			wantPath: "/custom/data/home",
			wantErr:  false,
		},
		{
			name: "Linux default",
			setup: func() {
				os.Unsetenv("XDG_DATA_HOME")
				t.Setenv("HOME", "/home/testuser")
			},
			wantPath: "/home/testuser/.local/share",
			wantErr:  false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			// Setup test environment
			if testCase.setup != nil {
				oldHome := os.Getenv("HOME")
				oldXDGDataHome := os.Getenv("XDG_DATA_HOME")
				defer func() {
					os.Setenv("HOME", oldHome)
					os.Setenv("XDG_DATA_HOME", oldXDGDataHome)
				}()
				testCase.setup()
			}

			// The "Linux default" expectation applies only on Linux
			if testCase.name == "Linux default" && runtime.GOOS != "linux" {
				t.Skip("skipping Linux-specific default path assertion on non-Linux platform")
			}

			path, err := getUserDataDir()

			if testCase.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.True(t, strings.HasSuffix(path, testCase.wantPath) ||
					filepath.Base(path) == filepath.Base(testCase.wantPath),
					"path %s should end with %s", path, testCase.wantPath)
			}
		})
	}
}

func TestGetDatabasePath(t *testing.T) {
	// Save original environment and restore after test
	originalXDGDataHome := os.Getenv("XDG_DATA_HOME")
	defer os.Setenv("XDG_DATA_HOME", originalXDGDataHome)

	// Create a test config
	cfg := &Config{}

	t.Run("default path", func(t *testing.T) {
		// Clear XDG_DATA_HOME for this test case
		os.Unsetenv("XDG_DATA_HOME")

		// Test that it returns a path with the correct suffix
		path := cfg.GetDatabasePath()
		assert.True(t, strings.HasSuffix(path, filepath.Join("gotya", "state", "installed.json")),
			"database path should end with gotya/state/installed.json, got: %s", path)
	})

	t.Run("with XDG_DATA_HOME set", func(t *testing.T) {
		// Set a test XDG_DATA_HOME
		testDataHome := "/test/data/home"
		os.Setenv("XDG_DATA_HOME", testDataHome)

		path := cfg.GetDatabasePath()
		// On Windows, the path might be converted, so we check the base names
		expectedSuffix := filepath.Join("gotya", "state", "installed.json")
		assert.True(t, strings.HasSuffix(path, expectedSuffix),
			"path %s should end with %s", path, expectedSuffix)

		// Check that it starts with our test path (handle path separators properly)
		expectedPrefix := filepath.Clean(testDataHome) + string(filepath.Separator)
		assert.True(t, strings.HasPrefix(filepath.ToSlash(path), filepath.ToSlash(expectedPrefix)),
			"path %s should start with %s", path, expectedPrefix)
	})
}

func TestRepositoryManagement(t *testing.T) {
	cfg := DefaultConfig()

	// Test AddRepository
	err := cfg.AddRepository("test-repo", "https://example.com/repo", true)
	require.NoError(t, err)
	assert.Len(t, cfg.Repositories, 1)
	assert.Equal(t, "test-repo", cfg.Repositories[0].Name)
	assert.True(t, cfg.Repositories[0].Enabled)

	// Test duplicate index
	err = cfg.AddRepository("test-repo", "https://example.com/another", true)
	assert.Error(t, err)

	// Test GetRepository
	repo := cfg.GetRepository("test-repo")
	require.NotNil(t, repo)
	assert.Equal(t, "test-repo", repo.Name)

	// Test RemoveRepository
	removed := cfg.RemoveRepository("test-repo")
	assert.True(t, removed)
	assert.Len(t, cfg.Repositories, 0)

	// Test Remove non-existent index
	removed = cfg.RemoveRepository("non-existent")
	assert.False(t, removed)

	// Test EnableRepository
	err = cfg.AddRepository("test-repo", "https://example.com/repo", false)
	require.NoError(t, err)

	enabled := cfg.EnableRepository("test-repo", true)
	assert.True(t, enabled)
	assert.True(t, cfg.Repositories[0].Enabled)

	// Test enabling non-existent index
	enabled = cfg.EnableRepository("non-existent", true)
	assert.False(t, enabled)
}
