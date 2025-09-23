package cli

import (
	"fmt"

	"github.com/cperrin88/gotya/internal/logger"
	"github.com/cperrin88/gotya/pkg/artifact"
	"github.com/cperrin88/gotya/pkg/config"
	"github.com/cperrin88/gotya/pkg/download"
	"github.com/cperrin88/gotya/pkg/http"
	"github.com/cperrin88/gotya/pkg/index"
)

// These variables will be set by the main artifact.
var (
	ConfigPath   *string
	Verbose      *bool
	NoColor      *bool
	OutputFormat *string
)

// This is a bridge function that the CLI commands can use.
func loadConfig() (*config.Config, error) {
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
			return nil, fmt.Errorf("failed to get default config path: %w", pathErr)
		}
		cfg, err = config.LoadConfig(defaultPath)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
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

	return cfg, nil
}

func loadIndexManager(config *config.Config, httpClient http.Client) index.Manager {
	repositories := make([]*index.Repository, len(config.Repositories))
	for _, repo := range config.Repositories {
		repositories = append(repositories, &index.Repository{
			Name:     repo.Name,
			URL:      repo.GetURL(),
			Priority: repo.Priority,
			Enabled:  repo.Enabled,
		})
	}
	return index.NewManager(httpClient, repositories, config.GetIndexDir(), config.Settings.CacheTTL)
}

func loadArtifactManager(config *config.Config) artifact.Manager {
	// Artifact manager now operates purely on local files and no longer depends on index or http
	return artifact.NewManager(config.Settings.Platform.OS, config.Settings.Platform.Arch, config.GetArtifactCacheDir(), config.Settings.InstallDir)
}

func loadHTTPClient(config *config.Config) http.Client {
	return http.NewHTTPClient(config.Settings.HTTPTimeout)
}

func loadDownloadManager(config *config.Config) download.Manager {
	return download.NewManager(config.Settings.HTTPTimeout, "")
}
