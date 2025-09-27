package cli

import (
	"fmt"

	"github.com/cperrin88/gotya/internal/logger"
	"github.com/cperrin88/gotya/pkg/artifact"
	"github.com/cperrin88/gotya/pkg/config"
	"github.com/cperrin88/gotya/pkg/download"
	"github.com/cperrin88/gotya/pkg/index"
)

// These variables will be set by the main artifact.
var (
	ConfigPath   *string
	Verbose      *bool
	NoColor      *bool
	OutputFormat *string
)

// ManagerFactory encapsulates the creation of various managers from configuration.
type ManagerFactory struct {
	config *config.Config
}

// NewManagerFactory creates a new manager factory with the given configuration.
func NewManagerFactory(cfg *config.Config) *ManagerFactory {
	return &ManagerFactory{config: cfg}
}

// CreateIndexManager creates an index manager from the configuration.
func (f *ManagerFactory) CreateIndexManager() index.Manager {
	repositories := make([]*index.Repository, 0, len(f.config.Repositories))
	for _, repo := range f.config.Repositories {
		repositories = append(repositories, &index.Repository{
			Name:     repo.Name,
			URL:      repo.GetURL(),
			Priority: repo.Priority,
			Enabled:  repo.Enabled,
		})
	}
	return index.NewManager(repositories, f.config.GetIndexDir())
}

// CreateArtifactManager creates an artifact manager from the configuration.
func (f *ManagerFactory) CreateArtifactManager() artifact.Manager {
	return artifact.NewManager(
		f.config.Settings.Platform.OS,
		f.config.Settings.Platform.Arch,
		f.config.GetArtifactCacheDir(),
		f.config.Settings.InstallDir,
		f.config.GetMetaDir(),
		f.config.GetDatabasePath(),
	)
}

// CreateDownloadManager creates a download manager from the configuration.
func (f *ManagerFactory) CreateDownloadManager() download.Manager {
	return download.NewManager(f.config.Settings.HTTPTimeout, "")
}

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
	if Verbose != nil && *Verbose {
		cfg.Settings.LogLevel = "debug"
	}

	// Initialize logger with config settings
	logger.InitLogger(cfg.Settings.LogLevel, logger.OutputFormat(cfg.Settings.OutputFormat))

	return cfg, nil
}

// loadIndexManager creates an index manager from the configuration.
func loadIndexManager(config *config.Config) index.Manager {
	factory := NewManagerFactory(config)
	return factory.CreateIndexManager()
}

// loadArtifactManager creates an artifact manager from the configuration.
func loadArtifactManager(config *config.Config) artifact.Manager {
	factory := NewManagerFactory(config)
	return factory.CreateArtifactManager()
}

// loadDownloadManager creates a download manager from the configuration.
func loadDownloadManager(config *config.Config) download.Manager {
	factory := NewManagerFactory(config)
	return factory.CreateDownloadManager()
}
