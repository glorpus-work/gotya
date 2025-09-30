// Package errors provides a comprehensive error handling system for the gotya package manager.
// It defines common error types, provides error wrapping functionality, and offers utilities
// for creating and working with domain-specific errors. The package follows Go's error handling
// best practices and provides consistent error messages across the application.
//
// The package includes:
// - Predefined error variables for common error cases
// - Error wrapping utilities for adding context
// - Helper functions for creating specific error types
// - Support for error categorization and comparison
// - Utilities for file and JSON operation errors
package errors

import (
	"fmt"
	"os"
)

// Common error types used throughout the application.
// Errors are grouped by their domain or functionality.
var (
	// Index errors are related to package index operations.
	ErrAlreadyExists = fmt.Errorf("resource already exists")
	ErrValidation    = fmt.Errorf("validation failed")
	ErrIndexConflict = fmt.Errorf("index conflict")
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
	// Use ErrConfigFileExistsWithPath to include the actual path in the error message.
	ErrConfigFileExists = fmt.Errorf("configuration file already exists (use --force to overwrite)")

	// ErrConfigFileRename is returned when renaming the temporary config file fails.
	ErrConfigFileRename = fmt.Errorf("failed to rename temporary config file")

	// ErrConfigFileChmod is returned when changing file permissions for the config file fails.
	ErrConfigFileChmod = fmt.Errorf("failed to set config file permissions")

	// ErrConfigMarshal is returned when marshaling the config to YAML fails.
	ErrConfigMarshal = fmt.Errorf("failed to marshal config to YAML")

	// Cache errors are related to cache management operations.
	// These errors occur during cache cleanup or access operations.
	ErrCacheCleanIndex = fmt.Errorf(
		"failed to clean index cache") // When index cache cleanup fails

	ErrCacheCleanArtifact = fmt.Errorf(
		"failed to clean artifact cache") // When artifact cache cleanup fails

	// Platform and configuration validation errors.
	// These errors are used to validate system-specific configuration values.

	// ErrInvalidOSValue is returned when an invalid operating system value is provided.
	ErrInvalidOSValue = fmt.Errorf("invalid OS value")

	// ErrInvalidArchValue is returned when an invalid architecture value is provided.
	ErrInvalidArchValue = fmt.Errorf("invalid architecture value")

	// CLI errors are returned during command-line interface operations.
	// These errors help users understand and correct their command usage.

	// ErrNoArtifactsSpecified is returned when a command requires artifact arguments
	// but none were provided and the --all flag was not used.
	ErrNoArtifactsSpecified = fmt.Errorf("no packages specified and --all flag not used")

	// ErrNoRepositories is returned when no repositories are configured
	// and an operation requires at least one index.
	ErrNoRepositories = fmt.Errorf("no repositories configured")

	// ErrArtifactNotFound is returned when an operation is attempted on a artifact
	// that doesn't exist in the database.
	ErrArtifactNotFound = fmt.Errorf("artifact not found")

	// Repository errors are related to index management operations.

	// ErrEmptyRepositoryName is returned when a index configuration is missing a name.
	ErrEmptyRepositoryName = fmt.Errorf("index name cannot be empty")

	// ErrRepositoryURLEmpty is returned when a index configuration is missing a URL.
	ErrRepositoryURLEmpty = fmt.Errorf("index URL cannot be empty")

	// ErrEmptyRepositoryURL is an alias for ErrRepositoryURLEmpty for backward compatibility.
	// Use ErrRepositoryURLEmpty instead.
	ErrEmptyRepositoryURL = ErrRepositoryURLEmpty

	// ErrConfigFileClose is returned when closing the config file fails.
	ErrConfigFileClose = fmt.Errorf("failed to close config file")

	// ErrConfigFileRead is returned when reading the config file fails.
	ErrConfigFileRead = fmt.Errorf("failed to read config file")

	// ErrUnsupportedOS is returned when an unsupported operating system is detected.
	ErrUnsupportedOS = fmt.Errorf("unsupported operating system")

	// ErrRepositoryExists is returned when attempting to add a index that already exists.
	ErrRepositoryExists = fmt.Errorf("index already exists")

	// ErrDuplicateRepository is returned when a index with the same name already exists.
	// The parameter 'name' is the duplicate index name.
	// This is an alias for ErrRepositoryExists for backward compatibility.
	ErrDuplicateRepository = ErrRepositoryExists

	// Artifact errors are related to artifact management operations.

	// ErrFileNotFound is returned when a required file cannot be found.
	ErrFileNotFound = fmt.Errorf("file not found")

	// ErrInvalidPath is returned when a file or directory path is invalid.
	ErrInvalidPath = fmt.Errorf("invalid path")

	// ErrInvalidFile is returned when a file exists but is invalid or corrupted.
	ErrInvalidFile = fmt.Errorf("invalid file")

	// Artifact errors are related to artifact management operations.
	// ErrArtifactInvalid is returned when a artifact is invalid or contains invalid data.
	ErrArtifactInvalid = fmt.Errorf("invalid artifact")

	// ErrValidationFailed is returned when artifact validation fails.
	// This is the preferred error for validation failures.
	ErrValidationFailed = fmt.Errorf("validation failed") // Alias for backward compatibility

	// ErrNameRequired is returned when a artifact name is required but not provided.
	ErrNameRequired = fmt.Errorf("name is required")

	// Example: fmt.Errorf("invalid artifact name: %s - must match %s", name, pattern).
	ErrInvalidArtifactName = fmt.Errorf("invalid artifact name: %%s - must match %%s")

	// ErrVersionRequired is returned when a artifact version is required but not provided.
	ErrVersionRequired = fmt.Errorf("version is required")

	// ErrInvalidVersionString is returned when a artifact version string is invalid.
	// The format string is used to include the version string and regex pattern.
	// Example: fmt.Errorf(ErrInvalidVersionString.Error(), "1.0", "^[0-9]+\\.[0-9]+\\.[0-9]+$")
	// The %%s placeholders will be replaced with the actual values when the error is created.
	ErrInvalidVersionString = fmt.Errorf("invalid artifact version: %%s - must match %%s")

	// ErrHTTPTimeoutNegative is returned when HTTP timeout is set to a negative value.
	ErrHTTPTimeoutNegative = fmt.Errorf("http_timeout cannot be negative")

	// ErrCacheTTLNegative is returned when cache TTL is set to a negative value.
	ErrCacheTTLNegative = fmt.Errorf("cache_ttl cannot be negative")

	// ErrMaxConcurrentInvalid is returned when max_concurrent_syncs is less than 1.
	ErrMaxConcurrentInvalid = fmt.Errorf("max_concurrent_syncs must be at least 1")

	// ErrInvalidOutputFormat is returned when an invalid output format is specified.
	ErrInvalidOutputFormat = fmt.Errorf("invalid output format")

	// ErrInvalidLogLevel is returned when an invalid log level is specified.
	ErrInvalidLogLevel = fmt.Errorf("invalid log level")

	// ErrTargetOSEmpty is returned when a target operating system is required but not provided.
	ErrTargetOSEmpty = fmt.Errorf("target OS cannot be empty")

	// ErrTargetArchEmpty is returned when a target architecture is required but not provided.
	ErrTargetArchEmpty = fmt.Errorf("target architecture cannot be empty")

	// Validation errors are used to validate various configuration values and inputs.

	// ErrInvalidOS is returned when an invalid operating system is specified.
	ErrInvalidOS = fmt.Errorf("invalid OS")

	// ErrInvalidArch is returned when an invalid architecture is specified.
	ErrInvalidArch = fmt.Errorf("invalid architecture")

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

	// Configuration errors are related to configuration value validation and processing.

	// ErrInvalidBoolValue is returned when an invalid boolean value is provided in the configuration.
	ErrInvalidBoolValue = fmt.Errorf("invalid boolean value")

	// ErrUnknownConfigKey is returned when an unknown configuration key is encountered.
	ErrUnknownConfigKey = fmt.Errorf("unknown configuration key")

	// ErrRepositoryURLInvalid is returned when a index URL is invalid.
	ErrRepositoryURLInvalid = fmt.Errorf("index URL is invalid")

	// ErrRepositoryNotFound is returned when a repository with the given name is not found.
	ErrRepositoryNotFound = fmt.Errorf("repository not found")

	// ErrFileOperationFailed is returned when a file operation fails.
	ErrFileOperationFailed = fmt.Errorf("file operation failed")

	// ErrJSONOperationFailed is returned when a JSON operation fails.
	ErrJSONOperationFailed = fmt.Errorf("JSON operation failed")

	// ErrFileHashMismatch is returned when a file's hash doesn't match the expected value.
	ErrFileHashMismatch = fmt.Errorf("file hash mismatch")

	// ErrFileSizeMismatch is returned when a file's size doesn't match the expected value.
	ErrFileSizeMismatch = fmt.Errorf("file size mismatch")

	// ErrFilePermissionMismatch is returned when a file's permissions don't match the expected value.
	ErrFilePermissionMismatch = fmt.Errorf("file permission mismatch")

	// ErrFileModeMismatch is returned when a file's mode doesn't match the expected value.
	ErrFileModeMismatch = fmt.Errorf("file mode mismatch")

	// ErrUnexpectedFile is returned when an unexpected file is found.
	ErrUnexpectedFile = fmt.Errorf("unexpected file")

	// ErrMissingFile is returned when an expected file is missing.
	ErrMissingFile = fmt.Errorf("missing file")

	// ErrDownloadFailed is returned when a download operation fails.
	ErrDownloadFailed = fmt.Errorf("download failed")
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

// ErrConfigFileExistsWithPath is a helper to create a wrapped error with the config path.
func ErrConfigFileExistsWithPath(configPath string) error {
	return fmt.Errorf("%w: %s", ErrConfigFileExists, configPath)
}

// ErrEmptyRepositoryNameWithIndex is a helper to create a wrapped error with the index index.
func ErrEmptyRepositoryNameWithIndex(i int) error {
	return fmt.Errorf("index %d: %w", i, ErrEmptyRepositoryName)
}

// ErrRepositoryURLEmptyWithName is a helper to create a wrapped error with the index name.
func ErrRepositoryURLEmptyWithName(name string) error {
	return fmt.Errorf("index '%s': %w", name, ErrRepositoryURLEmpty)
}

// ErrRepositoryExistsWithName is a helper to create a wrapped error with the index name.
func ErrRepositoryExistsWithName(name string) error {
	return fmt.Errorf("index '%s': %w", name, ErrRepositoryExists)
}

// ErrInvalidOSValueWithDetails is a helper to create a wrapped error with the invalid value and valid options.
func ErrInvalidOSValueWithDetails(value string, validOS []string) error {
	return fmt.Errorf("%w: %s. Valid values are: %v", ErrInvalidOSValue, value, validOS)
}

// ErrInvalidArchValueWithDetails is a helper to create a wrapped error with the invalid value and valid options.
func ErrInvalidArchValueWithDetails(value string, validArch []string) error {
	return fmt.Errorf("%w: %s. Valid values are: %v", ErrInvalidArchValue, value, validArch)
}

// ErrInvalidOutputFormatWithDetails is a helper to create a wrapped error with the invalid format and valid options.
func ErrInvalidOutputFormatWithDetails(format string) error {
	return fmt.Errorf("%w: '%s', must be one of: json, table, yaml", ErrInvalidOutputFormat, format)
}

// ErrInvalidLogLevelWithDetails is a helper to create a wrapped error with the invalid level and valid options.
func ErrInvalidLogLevelWithDetails(level string) error {
	return fmt.Errorf("%w: '%s', must be one of: panic, fatal, error, warn, info, debug, trace", ErrInvalidLogLevel, level)
}

// ErrInvalidOSWithDetails is a helper to create a wrapped error with the invalid OS and valid options.
func ErrInvalidOSWithDetails(operatingSystem string) error {
	return fmt.Errorf("%w: %s, must be one of: windows, linux, darwin, freebsd, openbsd, netbsd", ErrInvalidOS, operatingSystem)
}

// ErrInvalidArchWithDetails is a helper to create a wrapped error with the invalid architecture and valid options.
func ErrInvalidArchWithDetails(arch string) error {
	return fmt.Errorf("%w: %s, must be one of: amd64, 386, arm, arm64", ErrInvalidArch, arch)
}

// ErrInvalidBoolValueWithDetails is a helper to create a wrapped error with the key and invalid value.
func ErrInvalidBoolValueWithDetails(key, value string) error {
	return fmt.Errorf("%w for %s: %s", ErrInvalidBoolValue, key, value)
}

// ErrRepositoryNotFoundWithName creates an error for when a repository with the given name is not found.
func ErrRepositoryNotFoundWithName(name string) error {
	return fmt.Errorf("%w: %s", ErrRepositoryNotFound, name)
}

// ErrUnknownConfigKeyWithName is a helper to create a wrapped error with the unknown key.
func ErrUnknownConfigKeyWithName(key string) error {
	return fmt.Errorf("%w: %s", ErrUnknownConfigKey, key)
}

// Helper functions for artifact errors

// WrapFileError wraps a file-related error with additional context.
func WrapFileError(err error, operation, path string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s %s: %w: %w", operation, path, ErrFileOperationFailed, err)
}

// WrapJSONError wraps a JSON-related error with additional context.
func WrapJSONError(err error, operation string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w: %w", operation, ErrJSONOperationFailed, err)
}

// NewFileHashMismatchError creates a new error for file hash mismatches.
func NewFileHashMismatchError(filename, expected, actual string) error {
	return fmt.Errorf("%w: %s (expected: %s, got: %s)",
		ErrFileHashMismatch, filename, expected, actual)
}

// NewFileSizeMismatchError creates a new error for file size mismatches.
func NewFileSizeMismatchError(filename string, expected, actual int64) error {
	return fmt.Errorf("%w: %s (expected: %d, got: %d)",
		ErrFileSizeMismatch, filename, expected, actual)
}

// NewFilePermissionMismatchError creates a new error for permission mismatches.
func NewFilePermissionMismatchError(filename string, expected, actual os.FileMode) error {
	return fmt.Errorf("%w: %s (expected: %o, got: %o)",
		ErrFilePermissionMismatch, filename, expected, actual)
}

// NewFileModeMismatchError creates a new error for file mode mismatches.
func NewFileModeMismatchError(filename string, expected, actual uint32) error {
	return fmt.Errorf("%w: %s (expected: %o, got: %o)",
		ErrFileModeMismatch, filename, expected, actual)
}

// NewUnexpectedFileError creates a new error for unexpected files.
func NewUnexpectedFileError(filename string) error {
	return fmt.Errorf("%w: %s", ErrUnexpectedFile, filename)
}

// NewMissingFileError creates a new error for missing files.
func NewMissingFileError(filename string) error {
	return fmt.Errorf("%w: %s", ErrMissingFile, filename)
}
