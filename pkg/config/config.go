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

// Config represents the application configuration.
type Config struct {
	// Repository configuration
	Repositories []RepositoryConfig `yaml:"repositories"`

	// General settings
	Settings Settings `yaml:"settings"`
}

// RepositoryConfig represents a single repository configuration.
type RepositoryConfig struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	Enabled  bool   `yaml:"enabled"`
	Priority int    `yaml:"priority"`
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
	AutoSync bool          `yaml:"auto_sync"`

	// Installation settings
	InstallDir string `yaml:"install_dir,omitempty"` // Base directory for package installations

	// Network settings
	HTTPTimeout   time.Duration `yaml:"http_timeout"`
	MaxConcurrent int           `yaml:"max_concurrent_syncs"`

	// Platform settings
	Platform PlatformConfig `yaml:"platform,omitempty"`

	// Output settings
	OutputFormat string `yaml:"output_format"` // json, table, yaml
	ColorOutput  bool   `yaml:"color_output"`
	LogLevel     string `yaml:"log_level"` // panic, fatal, error, warn, info, debug, trace
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	// Get the default install directory (usually ~/.local/share/gotya/install on Linux)
	var installDir string
	userDataDir, err := getUserDataDir()
	if err == nil && userDataDir != "" {
		installDir = filepath.Join(userDataDir, "install")
	}

	return &Config{
		Repositories: []RepositoryConfig{},
		Settings: Settings{
			CacheTTL:      DefaultCacheTTL,
			AutoSync:      false,
			HTTPTimeout:   DefaultHTTPTimeout,
			MaxConcurrent: DefaultMaxConcurrent,
			InstallDir:    installDir,
			Platform: PlatformConfig{
				OS:   runtime.GOOS,
				Arch: runtime.GOARCH,
				// OS and Arch are empty by default for auto-detection
				PreferNative: true,
			},
			OutputFormat: "table",
			ColorOutput:  true,
			LogLevel:     "info",
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
	defer file.Close()

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
		file.Close()
		os.Remove(tempPath)
		return errors.Wrap(errors.ErrConfigEncode, err.Error())
	}

	encoder.Close()
	file.Close()

	// Atomically replace the config file
	if err := os.Rename(tempPath, absPath); err != nil {
		// Clean up temp file if rename fails
		os.Remove(tempPath)
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

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c == nil {
		return errors.ErrConfigValidation
	}

	repoNames := make(map[string]bool)
	for i, repo := range c.Repositories {
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

	// Validate platform settings
	if c.Settings.Platform.OS != "" {
		switch c.Settings.Platform.OS {
		case platform.OSWindows, platform.OSLinux, platform.OSDarwin,
			platform.OSFreeBSD, platform.OSOpenBSD, platform.OSNetBSD:
			// Valid OS
		default:
			return errors.ErrInvalidOSValueWithDetails(c.Settings.Platform.OS, platform.GetValidOS())
		}
	}

	if c.Settings.Platform.Arch != "" {
		switch c.Settings.Platform.Arch {
		case "amd64", "386", "arm", "arm64":
			// Valid architecture
		default:
			return errors.ErrInvalidArchValueWithDetails(c.Settings.Platform.Arch, platform.GetValidArch())
		}
	}

	// Validate settings
	if c.Settings.HTTPTimeout < 0 {
		return errors.ErrHTTPTimeoutNegative
	}
	if c.Settings.CacheTTL < 0 {
		return errors.ErrCacheTTLNegative
	}
	if c.Settings.MaxConcurrent < 1 {
		return errors.ErrMaxConcurrentInvalid
	}

	// Validate output format
	validFormats := map[string]bool{
		"json":  true,
		"table": true,
		"yaml":  true,
	}
	if !validFormats[c.Settings.OutputFormat] {
		return errors.ErrInvalidOutputFormatWithDetails(c.Settings.OutputFormat)
	}

	// Validate log level
	validLevels := map[string]bool{
		"panic": true,
		"fatal": true,
		"error": true,
		"warn":  true,
		"info":  true,
		"debug": true,
		"trace": true,
	}
	if !validLevels[strings.ToLower(c.Settings.LogLevel)] {
		return errors.ErrInvalidLogLevelWithDetails(c.Settings.LogLevel)
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

// AddRepository adds a repository to the configuration.
// Returns an error if a repository with the same name already exists.
func (c *Config) AddRepository(name, url string, enabled bool) error {
	// Check if repository already exists
	for _, repo := range c.Repositories {
		if repo.Name == name {
			return errors.ErrRepositoryExistsWithName(name)
		}
	}

	// Add new repository
	c.Repositories = append(c.Repositories, RepositoryConfig{
		Name:     name,
		URL:      url,
		Enabled:  enabled,
		Priority: 0,
	})

	return nil
}

// RemoveRepository removes a repository from the configuration.
func (c *Config) RemoveRepository(name string) bool {
	for i, repo := range c.Repositories {
		if repo.Name == name {
			c.Repositories = append(c.Repositories[:i], c.Repositories[i+1:]...)
			return true
		}
	}
	return false
}

// GetRepository gets a repository configuration by name.
func (c *Config) GetRepository(name string) *RepositoryConfig {
	for i, repo := range c.Repositories {
		if repo.Name == name {
			return &c.Repositories[i]
		}
	}
	return nil
}

// EnableRepository enables or disables a repository.
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
	stateDir, err := getUserDataDir()
	if err != nil {
		// Fallback to temp directory if we can't determine state dir
		stateDir = filepath.Join(os.TempDir(), "gotya")
	}

	return filepath.Join(stateDir, "gotya", "state", "installed.json")
}

// getUserDataDir returns the user state directory following platform conventions.
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
		return filepath.Join(homeDir, ".local", "share", "gotya", "state"), nil
	}

	// For all other platforms (Windows, macOS, etc.), use UserConfigDir + gotya/state
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}
	return filepath.Join(configDir, "gotya", "state"), nil
}
