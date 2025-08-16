package cli

import (
	"fmt"

	"github.com/cperrin88/gotya/pkg/config"
	"github.com/cperrin88/gotya/pkg/repository"
	"github.com/sirupsen/logrus"
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
func loadConfigAndManager() (*config.Config, repository.Manager, error) {
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
		cfg.Settings.LogLevel = "debug"
	}

	// Initialize logger with config settings
	InitLogger(cfg.Settings.LogLevel, !cfg.Settings.ColorOutput)

	// Create repository manager with platform settings from config
	manager := repository.NewManagerWithPlatform(
		"", // cacheDir will be set by the manager if empty
		cfg.Settings.Platform.OS,
		cfg.Settings.Platform.Arch,
		cfg.Settings.Platform.PreferNative,
	)

	Debug("Initialized repository manager with platform settings", logrus.Fields{
		"os":           cfg.Settings.Platform.OS,
		"arch":         cfg.Settings.Platform.Arch,
		"preferNative": cfg.Settings.Platform.PreferNative,
	})

	// Apply config repositories to manager
	for _, repo := range cfg.Repositories {
		if err := manager.AddRepository(repo.Name, repo.URL); err != nil {
			return nil, nil, fmt.Errorf("failed to add repository %s: %w", repo.Name, err)
		}
		if err := manager.EnableRepository(repo.Name, repo.Enabled); err != nil {
			return nil, nil, fmt.Errorf("failed to configure repository %s: %w", repo.Name, err)
		}
	}

	return cfg, manager, nil
}
