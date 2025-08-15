package cli

import (
	"fmt"

	"github.com/cperrin88/gotya/pkg/config"
	"github.com/cperrin88/gotya/pkg/repo"
)

// These variables will be set by the main package
var (
	ConfigPath   *string
	Verbose      *bool
	NoColor      *bool
	OutputFormat *string
)

// loadConfigAndManager loads the configuration and creates a manager
// This is a bridge function that the CLI commands can use
func loadConfigAndManager() (*config.Config, *repo.Manager, error) {
	var cfg *config.Config
	var err error

	configPath := ""
	if ConfigPath != nil {
		configPath = *ConfigPath
	}

	if configPath != "" {
		cfg, err = config.LoadConfig(configPath)
	} else {
		defaultPath, pathErr := config.GetDefaultConfigPath()
		if pathErr != nil {
			return nil, nil, fmt.Errorf("failed to get default config path: %w", pathErr)
		}
		cfg, err = config.LoadConfig(defaultPath)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Override config with CLI flags if provided
	if OutputFormat != nil && *OutputFormat != "" {
		cfg.Settings.OutputFormat = *OutputFormat
	}
	if NoColor != nil && *NoColor {
		cfg.Settings.ColorOutput = false
	}
	if Verbose != nil && *Verbose {
		cfg.Settings.VerboseLogging = true
	}

	// Create manager
	var manager *repo.Manager
	if cfg.Settings.CacheDir != "" {
		manager, err = repo.NewManagerWithCacheDir(cfg.Settings.CacheDir)
	} else {
		manager, err = repo.NewManager()
	}

	if err != nil {
		return nil, nil, fmt.Errorf("failed to create manager: %w", err)
	}

	// Apply config to manager
	if err := cfg.ApplyToManager(manager); err != nil {
		return nil, nil, fmt.Errorf("failed to apply config: %w", err)
	}

	return cfg, manager, nil
}
