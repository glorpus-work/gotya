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
	ErrEmptyConfigPath = fmt.Errorf(
		"config file path cannot be empty") // When config file path is empty

	ErrInvalidConfigPath = fmt.Errorf(
		"invalid config file path") // When provided config file path is invalid

	ErrConfigParse = fmt.Errorf(
		"failed to parse config") // When config file cannot be parsed

	// ErrConfigValidation is returned when configuration values fail validation.
	ErrConfigValidation = fmt.Errorf(
		"invalid configuration") // When config values fail validation

	ErrConfigEncode = fmt.Errorf(
		"failed to encode config") // When config cannot be encoded

	ErrConfigDirectory = fmt.Errorf(
		"failed to create config directory") // When config dir cannot be created

	ErrConfigFileCreate = fmt.Errorf(
		"failed to create config file") // When config file cannot be created

	ErrConfigFileNotExists = fmt.Errorf(
		"configuration file does not exist") // When config file does not exist

	ErrConfigInvalidFormat = fmt.Errorf(
		"invalid configuration format") // When config format is invalid

	ErrConfigInvalidValue = fmt.Errorf(
		"invalid configuration value") // When config value is invalid

	// ErrConfigFileExists is returned when attempting to create a configuration file that already exists.
	// The error includes the path to the existing file and suggests using --force to overwrite.
	ErrConfigFileExists = func(configPath string) error {
		return fmt.Errorf("configuration file already exists at %s (use --force to overwrite)", configPath)
	}

	// Cache errors are related to cache management operations.
	// These errors occur during cache cleanup or access operations.
	ErrCacheCleanIndex = fmt.Errorf(
		"failed to clean index cache") // When index cache cleanup fails

	ErrCacheCleanPackage = fmt.Errorf(
		"failed to clean package cache") // When package cache cleanup fails

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
	ErrPackageNotFound = fmt.Errorf("package not found")

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

	// Package errors are related to package management operations.
	// ErrPackageInvalid is returned when a package is invalid or contains invalid data.
	ErrPackageInvalid = fmt.Errorf("invalid package")

	// ErrValidationFailed is returned when package validation fails.
	// This is the preferred error for validation failures.
	ErrValidationFailed = fmt.Errorf("validation failed") // Alias for backward compatibility

	// ErrNameRequired is returned when a package name is required but not provided.
	ErrNameRequired = fmt.Errorf("name is required")

	// ErrInvalidPackageName is returned when a package name contains invalid characters.
	// The format string is used to include the package name and regex pattern.
	// Example: fmt.Errorf("invalid package name: %s - must match %s", name, pattern)
	ErrInvalidPackageName = fmt.Errorf("invalid package name: %%s - must match %%s")

	// ErrVersionRequired is returned when a package version is required but not provided.
	ErrVersionRequired = fmt.Errorf("version is required")

	// ErrInvalidVersionString is returned when a package version string is invalid.
	// The format string is used to include the version string and regex pattern.
	// Example: fmt.Errorf(ErrInvalidVersionString.Error(), "1.0", "^[0-9]+\\.[0-9]+\\.[0-9]+$")
	// The %%s placeholders will be replaced with the actual values when the error is created.
	ErrInvalidVersionString = fmt.Errorf("invalid package version: %%s - must match %%s")

	// ErrHTTPTimeoutNegative is returned when HTTP timeout is set to a negative value.
	ErrHTTPTimeoutNegative = fmt.Errorf("http_timeout cannot be negative")

	// ErrCacheTTLNegative is returned when cache TTL is set to a negative value.
	ErrCacheTTLNegative = fmt.Errorf("cache_ttl cannot be negative")

	// ErrMaxConcurrentInvalid is returned when max_concurrent_syncs is less than 1.
	ErrMaxConcurrentInvalid = fmt.Errorf("max_concurrent_syncs must be at least 1")

	// ErrInvalidOutputFormat is returned when an invalid output format is specified.
	// The error includes the invalid format and a list of valid formats.
	ErrInvalidOutputFormat = func(format string) error {
		return fmt.Errorf("invalid output format '%s', must be one of: json, table, yaml", format)
	}

	// ErrInvalidLogLevel is returned when an invalid log level is specified.
	// The error includes the invalid level and a list of valid log levels.
	ErrInvalidLogLevel = func(level string) error {
		return fmt.Errorf("invalid log level '%s', must be one of: panic, fatal, error, warn, info, debug, trace", level)
	}

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
	// This is an alias for ErrHTTPTimeoutNegative for backward compatibility.
	ErrNegativeHTTPTimeout = ErrHTTPTimeoutNegative

	// ErrNegativeCacheTTL is returned when a negative cache TTL value is provided.
	// This is an alias for ErrCacheTTLNegative for backward compatibility.
	ErrNegativeCacheTTL = ErrCacheTTLNegative

	// ErrInvalidConcurrency is returned when an invalid concurrency value is provided.
	// The value must be at least 1.
	// This is an alias for ErrMaxConcurrentInvalid for backward compatibility.
	ErrInvalidConcurrency = ErrMaxConcurrentInvalid

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
