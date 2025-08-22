package errors

import (
	"fmt"

	"github.com/cperrin88/gotya/pkg/platform"
)

// Common error types used throughout the application.
// Errors are grouped by their domain or functionality.
var (
	// Config errors are related to configuration file operations and validation.
	// These errors typically occur during application startup or config reload.
	ErrEmptyConfigPath   = fmt.Errorf("config file path cannot be empty")  // Returned when a configuration file path is empty
	ErrInvalidConfigPath = fmt.Errorf("invalid config file path")          // Returned when the provided config file path is invalid
	ErrConfigParse       = fmt.Errorf("failed to parse config")            // Returned when the config file cannot be parsed (e.g., invalid YAML/JSON)
	ErrConfigValidation  = fmt.Errorf("invalid configuration")             // Returned when configuration values fail validation
	ErrConfigEncode      = fmt.Errorf("failed to encode config")           // Returned when the config cannot be encoded (e.g., during save)
	ErrConfigDirectory   = fmt.Errorf("failed to create config directory") // Returned when the config directory cannot be created
	ErrConfigFileCreate  = fmt.Errorf("failed to create config file")      // Returned when the config file cannot be created

	// Cache errors are related to cache management operations.
	// These errors occur during cache cleanup or access operations.
	ErrCacheCleanIndex   = fmt.Errorf("failed to clean index cache")   // Returned when index cache cleanup fails
	ErrCacheCleanPackage = fmt.Errorf("failed to clean package cache") // Returned when package cache cleanup fails

	// Platform and configuration validation errors.
	// These errors are used to validate system-specific configuration values.

	// ErrInvalidOSValue is returned when an invalid operating system value is provided.
	// The error includes the invalid value and a list of valid OS values.
	ErrInvalidOSValue = func(value string) error {
		return fmt.Errorf("invalid OS value: %s. Valid values are: %v", value, platform.GetValidOS())
	}

	// ErrInvalidArchValue is returned when an invalid architecture value is provided.
	// The error includes the invalid value and a list of valid architecture values.
	ErrInvalidArchValue = func(value string) error {
		return fmt.Errorf("invalid architecture value: %s. Valid values are: %v", value, platform.GetValidArch())
	}

	// ErrConfigFileExists is returned when attempting to create a configuration file that already exists.
	// The error includes the path to the existing file and suggests using --force to overwrite.
	ErrConfigFileExists = func(configPath string) error {
		return fmt.Errorf("configuration file already exists at %s (use --force to overwrite)", configPath)
	}

	// CLI errors are returned during command-line interface operations.
	// These errors help users understand and correct their command usage.

	// ErrNoPackagesSpecified is returned when a command requires package arguments
	// but none were provided and the --all flag was not used.
	ErrNoPackagesSpecified = fmt.Errorf("no packages specified and --all flag not used")

	// ErrNoRepositories is returned when no repositories are configured
	// and an operation requires at least one repository.
	ErrNoRepositories = fmt.Errorf("no repositories configured")

	// ErrPackageNotFound is returned when an operation is attempted on a package
	// that doesn't exist in the database.
	ErrPackageNotFound = fmt.Errorf("failed to remove package from database: package not found")

	// Repository errors are related to repository management operations.

	// ErrEmptyRepositoryName is returned when a repository configuration is missing a name.
	// The parameter 'i' is the index of the repository in the configuration.
	ErrEmptyRepositoryName = func(i int) error {
		return fmt.Errorf("repository %d: name cannot be empty", i)
	}

	// ErrEmptyRepositoryURL is returned when a repository configuration is missing a URL.
	// The parameter 'name' is the name of the repository.
	ErrEmptyRepositoryURL = func(name string) error {
		return fmt.Errorf("repository '%s': URL cannot be empty", name)
	}

	// ErrDuplicateRepository is returned when a repository with the same name already exists.
	// The parameter 'name' is the duplicate repository name.
	ErrDuplicateRepository = func(name string) error {
		return fmt.Errorf("repository '%s': duplicate repository name", name)
	}

	// Package errors are related to package management operations.

	// ErrFileNotFound is returned when a required file cannot be found.
	ErrFileNotFound = fmt.Errorf("file not found")

	// ErrInvalidPath is returned when a file or directory path is invalid.
	ErrInvalidPath = fmt.Errorf("invalid path")

	// ErrInvalidFile is returned when a file exists but is invalid or corrupted.
	ErrInvalidFile = fmt.Errorf("invalid file")

	// ErrPackageInvalid is returned when a package is malformed or contains invalid data.
	ErrPackageInvalid = fmt.Errorf("invalid package")

	// ErrValidationFailed is returned when package validation fails.
	// This is the preferred error for validation failures.
	ErrValidationFailed = fmt.Errorf("validation failed") // Alias for backward compatibility

	// ErrNameRequired is returned when a package name is required but not provided.
	ErrNameRequired = fmt.Errorf("name is required")

	// ErrInvalidPackageName is returned when a package name contains invalid characters.
	ErrInvalidPackageName = fmt.Errorf("invalid package name")

	// ErrVersionRequired is returned when a package version is required but not provided.
	ErrVersionRequired = fmt.Errorf("version is required")

	// ErrInvalidVersionString is returned when a version string is malformed.
	ErrInvalidVersionString = fmt.Errorf("invalid version string")

	// ErrTargetOSEmpty is returned when a target operating system is required but not provided.
	ErrTargetOSEmpty = fmt.Errorf("target OS cannot be empty")

	// ErrTargetArchEmpty is returned when a target architecture is required but not provided.
	ErrTargetArchEmpty = fmt.Errorf("target architecture cannot be empty")

	// Validation errors are used to validate various configuration values and inputs.

	// ErrInvalidOS is returned when an invalid operating system is specified.
	// The error includes the invalid value and a list of valid OS values.
	ErrInvalidOS = func(os string) error {
		return fmt.Errorf("invalid OS: %s, must be one of: windows, linux, darwin, freebsd, openbsd, netbsd", os)
	}

	// ErrInvalidArch is returned when an invalid architecture is specified.
	// The error includes the invalid value and a list of valid architecture values.
	ErrInvalidArch = func(arch string) error {
		return fmt.Errorf("invalid architecture: %s, must be one of: amd64, 386, arm, arm64", arch)
	}

	// ErrNegativeHTTPTimeout is returned when a negative HTTP timeout value is provided.
	ErrNegativeHTTPTimeout = fmt.Errorf("http_timeout cannot be negative")

	// ErrNegativeCacheTTL is returned when a negative cache TTL value is provided.
	ErrNegativeCacheTTL = fmt.Errorf("cache_ttl cannot be negative")

	// ErrInvalidConcurrency is returned when an invalid concurrency value is provided.
	// The value must be at least 1.
	ErrInvalidConcurrency = fmt.Errorf("max_concurrent_syncs must be at least 1")

	// ErrInvalidOutputFormat is returned when an invalid output format is specified.
	// The error includes the invalid format and a list of valid formats.
	ErrInvalidOutputFormat = func(format string) error {
		return fmt.Errorf("invalid output_format '%s', must be one of: json, table, yaml", format)
	}

	// ErrInvalidLogLevel is returned when an invalid log level is specified.
	// The error includes the invalid level and a list of valid log levels.
	ErrInvalidLogLevel = func(level string) error {
		return fmt.Errorf("invalid log_level '%s', must be one of: panic, fatal, error, warn, info, debug, trace", level)
	}

	// ErrRepositoryExists is returned when attempting to add a repository that already exists.
	// The error includes the name of the existing repository.
	ErrRepositoryExists = func(name string) error {
		return fmt.Errorf("repository '%s' already exists", name)
	}

	// Configuration errors are related to configuration value validation and processing.

	// ErrInvalidBoolValue is returned when an invalid boolean value is provided in the configuration.
	// The error includes the configuration key and the invalid value.
	ErrInvalidBoolValue = func(key, value string) error {
		return fmt.Errorf("invalid boolean value for %s: %s", key, value)
	}

	// ErrUnknownConfigKey is returned when an unknown configuration key is encountered.
	// The error includes the unknown key to help with debugging.
	ErrUnknownConfigKey = func(key string) error {
		return fmt.Errorf("unknown configuration key: %s", key)
	}
)

// Wrap wraps an error with additional context.
// This is useful for adding context to errors as they propagate up the call stack.
// If the error is nil, Wrap returns nil.
//
// Example:
//
//	if err := someOperation(); err != nil {
//	    return errors.Wrap(err, "failed to perform operation")
//	}
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// Wrapf wraps an error with additional formatted context.
// This is similar to Wrap but allows for formatting the message with fmt.Sprintf-style formatting.
// If the error is nil, Wrapf returns nil.
//
// Example:
//
//	if err := someOperation(); err != nil {
//	    return errors.Wrapf(err, "failed to process %s", "some value")
//	}
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}
