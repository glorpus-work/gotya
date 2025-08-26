package cli

import (
	"fmt"

	"github.com/cperrin88/gotya/pkg/config"
	"github.com/cperrin88/gotya/pkg/http"
	"github.com/cperrin88/gotya/pkg/index"
	"github.com/cperrin88/gotya/pkg/logger"
	"github.com/cperrin88/gotya/pkg/pkg"
	"github.com/cperrin88/gotya/pkg/repository"
)

// These variables will be set by the main pkg.
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
	repositories := make([]*repository.Repository, len(config.Repositories))
	for _, repo := range config.Repositories {
		repositories = append(repositories, &repository.Repository{
			Name:     repo.Name,
			Url:      repo.GetUrl(),
			Priority: uint(repo.Priority),
			Enabled:  repo.Enabled,
		})
	}
	return index.NewManager(httpClient, repositories, config.GetIndexDir(), config.Settings.CacheTTL)
}

func loadPackageManager(config *config.Config, indexManager index.Manager, httpClient http.Client) pkg.Manager {
	return pkg.NewManager(indexManager, httpClient, config.Settings.Platform.OS, config.Settings.Platform.Arch, config.GetPackageCacheDir())
}

func loadHttpClient(config *config.Config) http.Client {
	return http.NewHTTPClient(config.Settings.HTTPTimeout)
}
