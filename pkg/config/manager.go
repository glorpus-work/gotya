package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	// Repository configuration
	Repositories []RepositoryConfig `yaml:"repositories"`

	// General settings
	Settings Settings `yaml:"settings"`
}

// RepositoryConfig represents a single repository configuration
type RepositoryConfig struct {
	Name    string `yaml:"name"`
	URL     string `yaml:"url"`
	Enabled bool   `yaml:"enabled"`
}

// Settings represents general application settings
type Settings struct {
	// Cache settings
	CacheDir string        `yaml:"cache_dir,omitempty"`
	CacheTTL time.Duration `yaml:"cache_ttl"`
	AutoSync bool          `yaml:"auto_sync"`

	// Network settings
	HTTPTimeout   time.Duration `yaml:"http_timeout"`
	MaxConcurrent int           `yaml:"max_concurrent_syncs"`

	// Output settings
	OutputFormat string `yaml:"output_format"` // json, table, yaml
	ColorOutput  bool   `yaml:"color_output"`
	LogLevel     string `yaml:"log_level"` // panic, fatal, error, warn, info, debug, trace
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Repositories: []RepositoryConfig{},
		Settings: Settings{
			CacheTTL:      24 * time.Hour,
			AutoSync:      false,
			HTTPTimeout:   30 * time.Second,
			MaxConcurrent: 3,
			OutputFormat:  "table",
			ColorOutput:   true,
			LogLevel:      "info",
		},
	}
}

// LoadConfig loads configuration from a file
func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	return LoadConfigFromReader(file)
}

// LoadConfigFromReader loads configuration from an io.Reader
func LoadConfigFromReader(reader io.Reader) (*Config, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read config data: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply defaults for missing values
	if err := config.applyDefaults(); err != nil {
		return nil, fmt.Errorf("failed to apply defaults: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// SaveConfig saves configuration to a file
func (c *Config) SaveConfig(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create temporary file
	tempPath := path + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp config file: %w", err)
	}

	// Write YAML data
	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)

	if err := encoder.Encode(c); err != nil {
		file.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to encode config: %w", err)
	}

	encoder.Close()
	file.Close()

	// Atomically replace the config file
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to replace config file: %w", err)
	}

	return nil
}

// ToYAML converts the config to YAML bytes
func (c *Config) ToYAML() ([]byte, error) {
	return yaml.Marshal(c)
}

// applyDefaults fills in missing values with defaults
func (c *Config) applyDefaults() error {
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

	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate repositories
	repoNames := make(map[string]bool)
	for i, repo := range c.Repositories {
		if repo.Name == "" {
			return fmt.Errorf("repository %d: name cannot be empty", i)
		}
		if repo.URL == "" {
			return fmt.Errorf("repository '%s': URL cannot be empty", repo.Name)
		}
		if repoNames[repo.Name] {
			return fmt.Errorf("repository '%s': duplicate repository name", repo.Name)
		}
		repoNames[repo.Name] = true
	}

	// Validate settings
	if c.Settings.HTTPTimeout < 0 {
		return fmt.Errorf("http_timeout cannot be negative")
	}
	if c.Settings.CacheTTL < 0 {
		return fmt.Errorf("cache_ttl cannot be negative")
	}
	if c.Settings.MaxConcurrent < 1 {
		return fmt.Errorf("max_concurrent_syncs must be at least 1")
	}

	validFormats := map[string]bool{"json": true, "table": true, "yaml": true}
	if !validFormats[c.Settings.OutputFormat] {
		return fmt.Errorf("invalid output_format '%s', must be one of: json, table, yaml", c.Settings.OutputFormat)
	}

	validLogLevels := map[string]bool{"panic": true, "fatal": true, "error": true, "warn": true, "info": true, "debug": true, "trace": true}
	if !validLogLevels[c.Settings.LogLevel] {
		return fmt.Errorf("invalid log_level '%s', must be one of: panic, fatal, error, warn, info, debug, trace", c.Settings.LogLevel)
	}

	return nil
}

// GetDefaultConfigPath returns the default configuration file path
func GetDefaultConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}

	// Create gotya config directory
	gotyaConfigDir := filepath.Join(configDir, "gotya")
	return filepath.Join(gotyaConfigDir, "config.yaml"), nil
}

// AddRepository adds a repository to the configuration
func (c *Config) AddRepository(name, url string, enabled bool) error {
	// Check if repository already exists
	for i, repo := range c.Repositories {
		if repo.Name == name {
			c.Repositories[i].URL = url
			c.Repositories[i].Enabled = enabled
			return nil
		}
	}

	// Add new repository
	c.Repositories = append(c.Repositories, RepositoryConfig{
		Name:    name,
		URL:     url,
		Enabled: enabled,
	})

	return nil
}

// RemoveRepository removes a repository from the configuration
func (c *Config) RemoveRepository(name string) bool {
	for i, repo := range c.Repositories {
		if repo.Name == name {
			c.Repositories = append(c.Repositories[:i], c.Repositories[i+1:]...)
			return true
		}
	}
	return false
}

// GetRepository gets a repository configuration by name
func (c *Config) GetRepository(name string) *RepositoryConfig {
	for i, repo := range c.Repositories {
		if repo.Name == name {
			return &c.Repositories[i]
		}
	}
	return nil
}

// EnableRepository enables or disables a repository
func (c *Config) EnableRepository(name string, enabled bool) bool {
	for i, repo := range c.Repositories {
		if repo.Name == name {
			c.Repositories[i].Enabled = enabled
			return true
		}
	}
	return false
}

// Manager interface defines the methods needed for repository management
type Manager interface {
	AddRepository(name, url string)
	EnableRepository(name string, enabled bool) error
	ListRepositories() []RepositoryInfo
}

// RepositoryInfo represents repository information from a manager
type RepositoryInfo struct {
	Name    string
	URL     string
	Enabled bool
}

// ApplyToManager applies configuration settings to a repository manager
func (c *Config) ApplyToManager(manager interface{}) error {
	// Type assert to get the manager interface we need
	if mgr, ok := manager.(interface {
		AddRepository(name, url string)
		EnableRepository(name string, enabled bool) error
	}); ok {
		// Add configured repositories to manager
		for _, repo := range c.Repositories {
			mgr.AddRepository(repo.Name, repo.URL)
			if err := mgr.EnableRepository(repo.Name, repo.Enabled); err != nil {
				return fmt.Errorf("failed to configure repository %s: %w", repo.Name, err)
			}
		}
	}

	return nil
}

// LoadFromManager loads repository configuration from a manager
func (c *Config) LoadFromManager(manager Manager) {
	c.Repositories = c.Repositories[:0] // Clear existing repositories

	for _, repo := range manager.ListRepositories() {
		c.Repositories = append(c.Repositories, RepositoryConfig{
			Name:    repo.Name,
			URL:     repo.URL,
			Enabled: repo.Enabled,
		})
	}
}
