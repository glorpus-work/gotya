package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/glorpus-work/gotya/pkg/fsutil"
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
    arch: amd64`

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
	assert.NotEmpty(t, data)

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
			name: "invalid log level",
			config: &Config{
				Settings: Settings{
					LogLevel:      "invalid-level",
					OutputFormat:  "text",
					MaxConcurrent: 1,
				},
			},
			wantErr: true,
			errMsg:  "invalid log level",
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
		_ = os.Setenv("HOME", originalHome)
		_ = os.Setenv("XDG_DATA_HOME", originalXDGDataHome)
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
				_ = os.Unsetenv("XDG_DATA_HOME")
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
					_ = os.Setenv("HOME", oldHome)
					_ = os.Setenv("XDG_DATA_HOME", oldXDGDataHome)
				}()
				testCase.setup()
			}

			// The "Linux default" expectation applies only on Linux
			if testCase.name == "Linux default" && runtime.GOOS != "linux" {
				t.Skip("skipping Linux-specific default path assertion on non-Linux platform")
			}

			path, err := getUserDataDir()

			if testCase.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
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
	defer func() { _ = os.Setenv("XDG_DATA_HOME", originalXDGDataHome) }()

	// Create a test config
	cfg := &Config{}

	t.Run("default path", func(t *testing.T) {
		// Clear XDG_DATA_HOME for this test case
		_ = os.Unsetenv("XDG_DATA_HOME")

		// Test that it returns a path with the correct suffix
		path := cfg.GetDatabasePath()
		assert.True(t, strings.HasSuffix(path, filepath.Join("gotya", "state", "installed.json")),
			"database path should end with gotya/state/installed.json, got: %s", path)
	})

	t.Run("with XDG_DATA_HOME set", func(t *testing.T) {
		// Set a test XDG_DATA_HOME
		testDataHome := "/test/data/home"
		_ = os.Setenv("XDG_DATA_HOME", testDataHome)

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
	require.Error(t, err)

	// Test GetRepository
	repo := cfg.GetRepository("test-repo")
	require.NotNil(t, repo)
	assert.Equal(t, "test-repo", repo.Name)

	// Test RemoveRepository
	removed := cfg.RemoveRepository("test-repo")
	assert.True(t, removed)
	assert.Empty(t, cfg.Repositories)

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

// Tests for config helpers

func TestSetValue(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		name     string
		key      string
		value    string
		wantErr  bool
		validate func(*testing.T, *Config)
	}{
		{
			name:  "set cache_dir",
			key:   "cache_dir",
			value: "/custom/cache",
			validate: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "/custom/cache", cfg.Settings.CacheDir)
			},
		},
		{
			name:  "set output_format",
			key:   "output_format",
			value: "json",
			validate: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "json", cfg.Settings.OutputFormat)
			},
		},
		{
			name:  "set log_level",
			key:   "log_level",
			value: "debug",
			validate: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "debug", cfg.Settings.LogLevel)
			},
		},
		{
			name:  "set platform.os",
			key:   "platform.os",
			value: "darwin",
			validate: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "darwin", cfg.Settings.Platform.OS)
			},
		},
		{
			name:  "set platform.arch",
			key:   "platform.arch",
			value: "arm64",
			validate: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "arm64", cfg.Settings.Platform.Arch)
			},
		},
		{
			name:    "unknown key",
			key:     "unknown_key",
			value:   "value",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgCopy := *cfg // Create a copy for each test

			err := cfgCopy.SetValue(tt.key, tt.value)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unknown configuration key")
			} else {
				require.NoError(t, err)
				tt.validate(t, &cfgCopy)
			}
		})
	}
}

func TestGetValue(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Settings.CacheDir = "/custom/cache"
	cfg.Settings.OutputFormat = "json"
	cfg.Settings.LogLevel = "debug"
	cfg.Settings.Platform.OS = "darwin"
	cfg.Settings.Platform.Arch = "arm64"

	tests := []struct {
		name     string
		key      string
		expected string
		wantErr  bool
	}{
		{
			name:     "get cache_dir",
			key:      "cache_dir",
			expected: "/custom/cache",
		},
		{
			name:     "get output_format",
			key:      "output_format",
			expected: "json",
		},
		{
			name:     "get log_level",
			key:      "log_level",
			expected: "debug",
		},
		{
			name:     "get platform.os",
			key:      "platform.os",
			expected: "darwin",
		},
		{
			name:     "get platform.arch",
			key:      "platform.arch",
			expected: "arm64",
		},
		{
			name:    "unknown key",
			key:     "unknown_key",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := cfg.GetValue(tt.key)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unknown configuration key")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, value)
			}
		})
	}
}

func TestToMap(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Settings.CacheDir = "/custom/cache"
	cfg.Settings.OutputFormat = "json"
	cfg.Settings.LogLevel = "debug"

	result := cfg.ToMap()

	// Should contain key settings
	assert.Contains(t, result, "cache_dir")
	assert.Contains(t, result, "output_format")
	assert.Contains(t, result, "log_level")
	assert.Equal(t, "/custom/cache", result["cache_dir"])
	assert.Equal(t, "json", result["output_format"])
	assert.Equal(t, "debug", result["log_level"])
}

func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig()

	require.NotNil(t, cfg)
	assert.Equal(t, 24*time.Hour, cfg.Settings.CacheTTL)
	assert.Equal(t, "text", cfg.Settings.OutputFormat)
	assert.Equal(t, "info", cfg.Settings.LogLevel)
}

func TestToYAML(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Settings.LogLevel = "debug"

	yamlData, err := cfg.ToYAML()
	require.NoError(t, err)
	assert.NotEmpty(t, yamlData)

	// Should contain YAML content
	assert.Contains(t, string(yamlData), "settings:")
	assert.Contains(t, string(yamlData), "log_level: debug")
}

func TestGetDefaultConfigPath(t *testing.T) {
	path, err := GetDefaultConfigPath()
	require.NoError(t, err)
	assert.NotEmpty(t, path)

	// Should end with gotya/config.yaml
	assert.True(t, strings.HasSuffix(path, filepath.Join("gotya", "config.yaml")),
		"path should end with gotya/config.yaml, got: %s", path)
}

func TestConfig_GetIndexDir(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Settings.CacheDir = "/custom/cache"

	indexDir := cfg.GetIndexDir()
	expected := filepath.Join("/custom/cache", "indexes")
	assert.Equal(t, expected, indexDir)
}

func TestConfig_GetArtifactCacheDir(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Settings.CacheDir = "/custom/cache"

	cacheDir := cfg.GetArtifactCacheDir()
	expected := filepath.Join("/custom/cache", "artifacts")
	assert.Equal(t, expected, cacheDir)
}

func TestConfig_GetCacheDir(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Settings.CacheDir = "/custom/cache"

	cacheDir := cfg.GetCacheDir()
	assert.Equal(t, "/custom/cache", cacheDir)
}

func TestConfig_GetMetaDir(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Settings.MetaDir = "/custom/meta"

	metaDir := cfg.GetMetaDir()
	assert.Equal(t, "/custom/meta", metaDir)
}

func TestRepositoryConfig_GetURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "valid URL",
			url:      "https://example.com/repo",
			expected: "https://example.com/repo",
		},
		{
			name:     "URL with path",
			url:      "https://github.com/user/repo.git",
			expected: "https://github.com/user/repo.git",
		},
		{
			name:     "invalid URL",
			url:      "not-a-url",
			expected: "",
		},
		{
			name:     "empty URL",
			url:      "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &RepositoryConfig{URL: tt.url}
			result := repo.GetURL()

			if tt.expected == "" {
				// For invalid or empty URLs, the function should return nil
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expected, result.String())
			}
		})
	}
}
