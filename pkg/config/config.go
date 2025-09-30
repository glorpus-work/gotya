// Package config provides configuration management for the gotya package manager.
// It handles loading, validating, and managing application settings, index repositories,
// and platform-specific configurations. The package supports YAML configuration files
// and provides sensible defaults while allowing for customization through configuration files
// and environment variables.
package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/fsutil"
	"github.com/cperrin88/gotya/pkg/platform"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration.
type Config struct {
	// Repository configuration
	Repositories []*RepositoryConfig `yaml:"repositories"`

	// General settings
	Settings Settings `yaml:"settings"`
}

// RepositoryConfig represents a single index configuration.
type RepositoryConfig struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	Enabled  bool   `yaml:"enabled"`
	Priority uint   `yaml:"priority"`
}

// PlatformConfig represents platform-specific configuration.
type PlatformConfig struct {
	// OS overrides the target operating system (e.g., "windows", "linux", "darwin")
	// If empty, the system will auto-detect the current OS
	OS string `yaml:"os,omitempty"`

	// Arch overrides the target architecture (e.g., "amd64", "arm64", "386")
	// If empty, the system will auto-detect the current architecture
	Arch string `yaml:"arch,omitempty"`

	// PreferNative controls whether to prefer native packages when available
	// If true, native packages will be preferred over platform-agnostic packages
	PreferNative bool `yaml:"prefer_native,omitempty"`
}

// Settings represents general application settings.
type Settings struct {
	// Cache settings
	CacheDir string        `yaml:"cache_dir,omitempty"`
	CacheTTL time.Duration `yaml:"cache_ttl"`

	// State settings
	StateDir string `yaml:"state_dir,omitempty"`

	// Installation settings
	InstallDir string `yaml:"install_dir,omitempty"` // Base directory for artifact installations
	MetaDir    string `yaml:"meta_dir,omitempty"`

	// Network settings
	HTTPTimeout   time.Duration `yaml:"http_timeout"`
	MaxConcurrent int           `yaml:"max_concurrent_syncs"`

	// Platform settings
	Platform PlatformConfig `yaml:"platform,omitempty"`

	// Output settings
	OutputFormat string `yaml:"output_format"` // json, table, yaml
	LogLevel     string `yaml:"log_level"`     // panic, fatal, error, warn, info, debug, trace
}

// Default configuration values.
const (
	// DefaultCacheTTL is the default time-to-live for cached data.
	DefaultCacheTTL = 24 * time.Hour

	// DefaultHTTPTimeout is the default timeout for HTTP requests.
	DefaultHTTPTimeout = 30 * time.Second

	// DefaultMaxConcurrent is the default maximum number of concurrent operations.
	DefaultMaxConcurrent = 5

	// YAMLIndent is the number of spaces to use for YAML indentation.
	YAMLIndent = 2
)

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	userConfigDir, err := getUserDataDir()
	if err != nil {
		// Fallback to current directory if we can't determine user config dir
		userConfigDir = "."
	}

	// Get default state directory
	defaultStateDir, err := getStateDir()
	if err != nil {
		// Fallback to userConfigDir if we can't determine state dir
		defaultStateDir = userConfigDir
	}

	return &Config{
		Repositories: []*RepositoryConfig{},
		Settings: Settings{
			CacheTTL:      DefaultCacheTTL,
			HTTPTimeout:   DefaultHTTPTimeout,
			MaxConcurrent: DefaultMaxConcurrent,
			OutputFormat:  "text",
			LogLevel:      "info",
			InstallDir:    filepath.Join(userConfigDir, "bin"),
			MetaDir:       filepath.Join(userConfigDir, "meta"),
			StateDir:      defaultStateDir,
			Platform: PlatformConfig{
				OS:           runtime.GOOS,
				Arch:         runtime.GOARCH,
				PreferNative: true,
			},
		},
	}
}

// LoadConfig loads configuration from a file.
func LoadConfig(path string) (*Config, error) {
	// Validate the config file path
	if path == "" {
		return nil, errors.ErrEmptyConfigPath
	}

	// Ensure the path is clean and absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.Wrap(errors.ErrInvalidConfigPath, err.Error())
	}

	// Check if file exists and is accessible
	file, err := os.Open(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, errors.Wrapf(err, "failed to open config file: %s", path)
	}
	defer func() { _ = file.Close() }()

	return LoadConfigFromReader(file)
}

// LoadConfigFromReader loads configuration from an io.Reader.
func LoadConfigFromReader(reader io.Reader) (*Config, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read config data")
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, errors.Wrap(errors.ErrConfigParse, err.Error())
	}

	// Apply defaults and validate
	config.applyDefaults()

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(errors.ErrConfigValidation, err.Error())
	}

	return &config, nil
}

// SaveConfig saves configuration to a file.
func (c *Config) SaveConfig(path string) error {
	// Validate the config file path
	if path == "" {
		return errors.ErrEmptyConfigPath
	}

	// Ensure the path is clean and absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return errors.Wrap(errors.ErrInvalidConfigPath, err.Error())
	}

	// Ensure the directory exists with secure permissions (0755)
	if err := os.MkdirAll(filepath.Dir(absPath), fsutil.DirModeDefault); err != nil {
		return errors.Wrap(errors.ErrConfigDirectory, err.Error())
	}

	// Create temporary file with secure permissions (0600)
	tempPath := absPath + ".tmp"
	file, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fsutil.FileModeDefault)
	if err != nil {
		return errors.Wrap(errors.ErrConfigFileCreate, err.Error())
	}

	// Write YAML data
	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(YAMLIndent)

	if err := encoder.Encode(c); err != nil {
		_ = file.Close()
		_ = os.Remove(tempPath)
		return errors.Wrap(errors.ErrConfigEncode, err.Error())
	}

	_ = encoder.Close()
	_ = file.Close()

	// Atomically replace the config file
	if err := os.Rename(tempPath, absPath); err != nil {
		// Clean up temp file if rename fails
		_ = os.Remove(tempPath)
		return errors.Wrap(errors.ErrConfigFileRename, err.Error())
	}

	// Ensure the final file has the correct permissions (0644)
	if err := os.Chmod(absPath, fsutil.FileModeDefault); err != nil {
		// This is not fatal, but we should log it
		return errors.Wrap(errors.ErrConfigFileChmod, err.Error())
	}

	return nil
}

// ToYAML converts the config to YAML bytes.
func (c *Config) ToYAML() ([]byte, error) {
	data, err := yaml.Marshal(c)
	if err != nil {
		return nil, errors.Wrap(errors.ErrConfigMarshal, err.Error())
	}
	return data, nil
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c == nil {
		return errors.ErrConfigValidation
	}
	if err := validateRepositories(c.Repositories); err != nil {
		return err
	}
	if err := validatePlatform(c.Settings.Platform); err != nil {
		return err
	}
	if err := validateSettings(c.Settings); err != nil {
		return err
	}
	return nil
}

func validateRepositories(repos []*RepositoryConfig) error {
	repoNames := make(map[string]bool)
	for i, repo := range repos {
		if repo.Name == "" {
			return errors.ErrEmptyRepositoryNameWithIndex(i)
		}
		if repo.URL == "" {
			return errors.ErrRepositoryURLEmptyWithName(repo.Name)
		}
		if repoNames[repo.Name] {
			return errors.ErrRepositoryExistsWithName(repo.Name)
		}
		repoNames[repo.Name] = true
	}
	return nil
}

func validatePlatform(p PlatformConfig) error {
	if p.OS != "" {
		switch p.OS {
		case platform.OSWindows, platform.OSLinux, platform.OSDarwin,
			platform.OSFreeBSD, platform.OSOpenBSD, platform.OSNetBSD:
			// ok
		default:
			return errors.ErrInvalidOSValueWithDetails(p.OS, platform.GetValidOS())
		}
	}
	if p.Arch != "" {
		switch p.Arch {
		case "amd64", "386", "arm", "arm64":
		default:
			return errors.ErrInvalidArchValueWithDetails(p.Arch, platform.GetValidArch())
		}
	}
	return nil
}

func validateSettings(s Settings) error {
	if s.HTTPTimeout < 0 {
		return errors.ErrHTTPTimeoutNegative
	}
	if s.CacheTTL < 0 {
		return errors.ErrCacheTTLNegative
	}
	if s.MaxConcurrent < 1 {
		return errors.ErrMaxConcurrentInvalid
	}
	validFormats := map[string]bool{"text": true, "json": true}
	if !validFormats[s.OutputFormat] {
		return errors.ErrInvalidOutputFormatWithDetails(s.OutputFormat)
	}
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[strings.ToLower(s.LogLevel)] {
		return errors.ErrInvalidLogLevelWithDetails(s.LogLevel)
	}
	return nil
}

// GetDefaultConfigPath returns the default configuration file path.
func GetDefaultConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}

	// Create gotya config directory
	gotyaConfigDir := filepath.Join(configDir, "gotya")
	return filepath.Join(gotyaConfigDir, "config.yaml"), nil
}

// AddRepository adds a index to the configuration.
// Returns an error if a index with the same name already exists.
func (c *Config) AddRepository(name, url string, enabled bool) error {
	// Check if index already exists
	for _, repo := range c.Repositories {
		if repo.Name == name {
			return errors.ErrRepositoryExistsWithName(name)
		}
	}

	// Add new index
	c.Repositories = append(c.Repositories, &RepositoryConfig{
		Name:     name,
		URL:      url,
		Enabled:  enabled,
		Priority: 0,
	})

	return nil
}

// RemoveRepository removes a index from the configuration.
func (c *Config) RemoveRepository(name string) bool {
	for i, repo := range c.Repositories {
		if repo.Name == name {
			c.Repositories = append(c.Repositories[:i], c.Repositories[i+1:]...)
			return true
		}
	}
	return false
}

// GetRepository gets a index configuration by name.
func (c *Config) GetRepository(name string) *RepositoryConfig {
	for i, repo := range c.Repositories {
		if repo.Name == name {
			return c.Repositories[i]
		}
	}
	return nil
}

// EnableRepository enables or disables a index.
func (c *Config) EnableRepository(name string, enabled bool) bool {
	for i, repo := range c.Repositories {
		if repo.Name == name {
			c.Repositories[i].Enabled = enabled
			return true
		}
	}
	return false
}

// GetDatabasePath returns the path to the installed packages database.
func (c *Config) GetDatabasePath() string {
	stateDir := c.GetStateDir()
	if stateDir == "" {
		// Fallback to old behavior if StateDir is not set
		var err error
		stateDir, err = getUserDataDir()
		if err != nil {
			// Fallback to temp directory if we can't determine state dir
			stateDir = filepath.Join(os.TempDir(), "gotya")
		}
	}
	return filepath.Join(stateDir, "gotya", "state", "installed.json")
}

// GetIndexDir returns the path to the index cache directory.
func (c *Config) GetIndexDir() string {
	return filepath.Join(c.GetCacheDir(), "indexes")
}

// GetArtifactCacheDir returns the path to the artifact cache directory.
func (c *Config) GetArtifactCacheDir() string {
	return filepath.Join(c.GetCacheDir(), "artifacts")
}

// GetCacheDir returns the base cache directory from settings.
func (c *Config) GetCacheDir() string {
	return c.Settings.CacheDir
}

// GetStateDir returns the base state directory from settings.
func (c *Config) GetStateDir() string {
	return c.Settings.StateDir
}

// GetMetaDir returns the path to the meta directory.
func (c *Config) GetMetaDir() string {
	return c.Settings.MetaDir
}

// applyDefaults fills in missing values with defaults.
func (c *Config) applyDefaults() {
	defaults := DefaultConfig()

	// Apply default settings if not set
	if c.Settings.CacheTTL == 0 {
		c.Settings.CacheTTL = defaults.Settings.CacheTTL
	}
	if c.Settings.HTTPTimeout == 0 {
		c.Settings.HTTPTimeout = defaults.Settings.HTTPTimeout
	}
	if c.Settings.MaxConcurrent == 0 {
		c.Settings.MaxConcurrent = defaults.Settings.MaxConcurrent
	}
	if c.Settings.OutputFormat == "" {
		c.Settings.OutputFormat = defaults.Settings.OutputFormat
	}
	if c.Settings.LogLevel == "" {
		c.Settings.LogLevel = defaults.Settings.LogLevel
	}
	if c.Settings.CacheDir == "" {
		c.Settings.CacheDir = defaults.Settings.CacheDir
	}
	if c.Settings.StateDir == "" {
		c.Settings.StateDir = defaults.Settings.StateDir
	}
	if c.Settings.MetaDir == "" {
		c.Settings.MetaDir = defaults.Settings.MetaDir
	}
	if c.Settings.InstallDir == "" {
		c.Settings.InstallDir = defaults.Settings.InstallDir
	}
	if c.Settings.Platform.OS == "" {
		c.Settings.Platform.OS = defaults.Settings.Platform.OS
	}
	if c.Settings.Platform.Arch == "" {
		c.Settings.Platform.Arch = defaults.Settings.Platform.Arch
	}

	// Set enabled to true by default for repositories if not explicitly set
	for i := range c.Repositories {
		if c.Repositories[i].Name != "" && c.Repositories[i].URL != "" {
			// In Go, bool defaults to false, so we need to explicitly set enabled repos
			// This assumes all configured repos should be enabled by default
			if c.Repositories[i].Name != "" { // Only enable if repo has a name
				c.Repositories[i].Enabled = true
			}
		}
	}
}

func getUserDataDir() (string, error) {
	// Check for XDG_STATE_HOME environment variable - if set, always use it
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir, nil
	}

	// Special case for Linux: follow XDG Base Directory Specification
	if runtime.GOOS == platform.OSLinux {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		return filepath.Join(homeDir, ".local", "share"), nil
	}

	// For all other platforms (Windows, macOS, etc.), use UserConfigDir + gotya/state
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}
	return configDir, nil
}

func getStateDir() (string, error) {
	stateDir, err := getUserDataDir()
	if err != nil {
		// Fallback to temp directory if we can't determine state dir
		stateDir = filepath.Join(os.TempDir(), "gotya")
	}
	return stateDir, nil
}
