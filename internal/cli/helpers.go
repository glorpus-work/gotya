package cli

import (
	"fmt"

	"github.com/cperrin88/gotya/pkg/config"
	"github.com/cperrin88/gotya/pkg/index"
	"github.com/cperrin88/gotya/pkg/logger"
	"github.com/sirupsen/logrus"
)

// These variables will be set by the main pkg.
var (
	ConfigPath   *string
	Verbose      *bool
	NoColor      *bool
	OutputFormat *string
)

// This is a bridge function that the CLI commands can use.
func loadConfigAndManager() (*config.Config, *index.ManagerImpl, error) {
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
	logger.InitLogger(cfg.Settings.LogLevel, !cfg.Settings.ColorOutput)

	// Create index manager with platform settings from config
	manager := index.NewRepositoryManager(cfg)

	logger.Debug("Initialized index manager with platform settings", logrus.Fields{
		"os":           cfg.Settings.Platform.OS,
		"arch":         cfg.Settings.Platform.Arch,
		"preferNative": cfg.Settings.Platform.PreferNative,
	})

	return cfg, manager, nil
}
