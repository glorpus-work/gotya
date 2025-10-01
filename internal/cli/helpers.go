package cli

import (
	"fmt"
	"strings"

	"github.com/glorpus-work/gotya/internal/logger"
	"github.com/glorpus-work/gotya/pkg/artifact"
	"github.com/glorpus-work/gotya/pkg/config"
	"github.com/glorpus-work/gotya/pkg/download"
	"github.com/glorpus-work/gotya/pkg/errors"
	"github.com/glorpus-work/gotya/pkg/index"
	"github.com/glorpus-work/gotya/pkg/model"
)

// ManagerFactory encapsulates the creation of various managers from configuration.
type ManagerFactory struct {
	config *config.Config
}

// These variables will be set by the main artifact.
var (
	ConfigPath   *string
	Verbose      *bool
	NoColor      *bool
	OutputFormat *string
)

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
func loadIndexManager(cfg *config.Config) index.Manager {
	factory := NewManagerFactory(cfg)
	return factory.CreateIndexManager()
}

// loadArtifactManager creates an artifact manager from the configuration.
func loadArtifactManager(cfg *config.Config) artifact.Manager {
	factory := NewManagerFactory(cfg)
	return factory.CreateArtifactManager()
}

// loadDownloadManager creates a download manager from the configuration.
func loadDownloadManager(cfg *config.Config) download.Manager {
	factory := NewManagerFactory(cfg)
	return factory.CreateDownloadManager()
}

// ParseDependencies parses a list of dependency strings in the format "package_name[:version_constraint][,package_name[:version_constraint],...]"
// If no version constraint is provided, it defaults to ">= 0.0.0"
func ParseDependencies(deps []string) ([]model.Dependency, error) {
	var dependencies []model.Dependency

	for _, depStr := range deps {
		depStr = strings.TrimSpace(depStr)
		if depStr == "" {
			continue
		}

		parts := strings.SplitN(depStr, ":", 2)
		name := strings.TrimSpace(parts[0])
		if name == "" {
			return nil, fmt.Errorf("invalid dependency format: empty package name in '%s': %w", depStr, errors.ErrValidation)
		}

		var versionConstraint string
		if len(parts) == 2 {
			versionConstraint = strings.TrimSpace(parts[1])
			if versionConstraint == "" {
				return nil, fmt.Errorf("invalid dependency format: empty version constraint for package '%s': %w", name, errors.ErrValidation)
			}
		} else {
			// Default version constraint if none provided
			versionConstraint = ">= 0.0.0"
		}

		dependencies = append(dependencies, model.Dependency{
			Name:              name,
			VersionConstraint: versionConstraint,
		})
	}

	return dependencies, nil
}
