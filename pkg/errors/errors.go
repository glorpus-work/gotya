package errors

import (
	"fmt"

	"github.com/cperrin88/gotya/pkg/platform"
)

// Common error types.
var (
	// Config errors.
	ErrEmptyConfigPath   = fmt.Errorf("config file path cannot be empty")
	ErrInvalidConfigPath = fmt.Errorf("invalid config file path")
	ErrConfigParse       = fmt.Errorf("failed to parse config")
	ErrConfigValidation  = fmt.Errorf("invalid configuration")
	ErrConfigEncode      = fmt.Errorf("failed to encode config")
	ErrConfigDirectory   = fmt.Errorf("failed to create config directory")
	ErrConfigFileCreate  = fmt.Errorf("failed to create config file")

	// Cache errors.
	ErrCacheClean        = fmt.Errorf("failed to clean cache")
	ErrCacheInfo         = fmt.Errorf("failed to get cache info")
	ErrCacheDirectory    = fmt.Errorf("cache directory cannot be empty")
	ErrCacheCleanIndex   = fmt.Errorf("failed to clean index cache")
	ErrCacheCleanPackage = fmt.Errorf("failed to clean package cache")

	// Hook errors.
	ErrHookTypeEmpty = fmt.Errorf("hook type cannot be empty")
	ErrHookExecution = fmt.Errorf("error executing hook")
	ErrHookScript    = fmt.Errorf("hook script error")
	ErrHookLoad      = fmt.Errorf("failed to load hook")

	// Installer errors.
	ErrPackageAlreadyInstalled = func(pkgName string) error {
		return fmt.Errorf("package %s is already installed (use --force to reinstall)", pkgName)
	}
	ErrPackageNotInstalled = func(pkgName string) error {
		return fmt.Errorf("package %s is not installed", pkgName)
	}
	ErrUnsupportedHookEvent = func(event string) error {
		return fmt.Errorf("unsupported hook event: %s", event)
	}

	// Config errors.
	ErrInvalidOSValue = func(value string) error {
		return fmt.Errorf("invalid OS value: %s. Valid values are: %v", value, platform.GetValidOS())
	}
	ErrInvalidArchValue = func(value string) error {
		return fmt.Errorf("invalid architecture value: %s. Valid values are: %v", value, platform.GetValidArch())
	}
	ErrConfigFileExists = func(configPath string) error {
		return fmt.Errorf("configuration file already exists at %s (use --force to overwrite)", configPath)
	}

	// CLI errors.
	ErrNoPackagesSpecified = fmt.Errorf("no packages specified and --all flag not used")
	ErrNoRepositories      = fmt.Errorf("no repositories configured")
	ErrPackageNotFound     = fmt.Errorf("failed to remove package from database: package not found")

	// Repository errors.
	ErrEmptyRepositoryName = func(i int) error {
		return fmt.Errorf("repository %d: name cannot be empty", i)
	}
	ErrEmptyRepositoryURL = func(name string) error {
		return fmt.Errorf("repository '%s': URL cannot be empty", name)
	}
	ErrDuplicateRepository = func(name string) error {
		return fmt.Errorf("repository '%s': duplicate repository name", name)
	}

	// Package errors
	ErrFileNotFound   = fmt.Errorf("file not found")
	ErrInvalidPath    = fmt.Errorf("invalid path")
	ErrInvalidFile    = fmt.Errorf("invalid file")
	ErrValidation     = fmt.Errorf("validation error")
	ErrPackageInvalid = fmt.Errorf("invalid package")

	// Validation errors.
	ErrInvalidOS = func(os string) error {
		return fmt.Errorf("invalid OS: %s, must be one of: windows, linux, darwin, freebsd, openbsd, netbsd", os)
	}
	ErrInvalidArch = func(arch string) error {
		return fmt.Errorf("invalid architecture: %s, must be one of: amd64, 386, arm, arm64", arch)
	}
	ErrNegativeHTTPTimeout = fmt.Errorf("http_timeout cannot be negative")
	ErrNegativeCacheTTL    = fmt.Errorf("cache_ttl cannot be negative")
	ErrInvalidConcurrency  = fmt.Errorf("max_concurrent_syncs must be at least 1")
	ErrInvalidOutputFormat = func(format string) error {
		return fmt.Errorf("invalid output_format '%s', must be one of: json, table, yaml", format)
	}
	ErrInvalidLogLevel = func(level string) error {
		return fmt.Errorf("invalid log_level '%s', must be one of: panic, fatal, error, warn, info, debug, trace", level)
	}
	ErrRepositoryExists = func(name string) error {
		return fmt.Errorf("repository '%s' already exists", name)
	}

	// Configuration errors.
	ErrInvalidBoolValue = func(key, value string) error {
		return fmt.Errorf("invalid boolean value for %s: %s", key, value)
	}
	ErrUnknownConfigKey = func(key string) error {
		return fmt.Errorf("unknown configuration key: %s", key)
	}
)

// Wrap wraps an error with additional context.
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// Wrapf wraps an error with additional formatted context.
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}
