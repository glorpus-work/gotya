// Package errutils provides a comprehensive error handling system for the gotya package manager.
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
package errutils

import (
	"fmt"
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

	// ErrRepositoryExists is returned when attempting to add a index that already exists.
	ErrRepositoryExists = fmt.Errorf("index already exists")

	// Artifact errors are related to artifact management operations.

	// ErrFileNotFound is returned when a required file cannot be found.
	ErrFileNotFound = fmt.Errorf("file not found")

	// ErrInvalidPath is returned when a file or directory path is invalid.
	ErrInvalidPath = fmt.Errorf("invalid path")

	// ErrEmptyPaths is returned when source or destination paths are empty in file operations.
	ErrEmptyPaths = fmt.Errorf("source and destination paths cannot be empty")

	// Artifact errors are related to artifact management operations.
	// ErrArtifactInvalid is returned when a artifact is invalid or contains invalid data.
	ErrArtifactInvalid = fmt.Errorf("invalid artifact")

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

	// Configuration errors are related to configuration value validation and processing.

	// ErrInvalidBoolValue is returned when an invalid boolean value is provided in the configuration.
	ErrInvalidBoolValue = fmt.Errorf("invalid boolean value")

	// ErrUnknownConfigKey is returned when an unknown configuration key is encountered.
	ErrUnknownConfigKey = fmt.Errorf("unknown configuration key")

	// ErrRepositoryNotFound is returned when a repository with the given name is not found.
	ErrRepositoryNotFound = fmt.Errorf("repository not found")

	// ErrFileHashMismatch is returned when a file's hash doesn't match the expected value.
	ErrFileHashMismatch = fmt.Errorf("file hash mismatch")

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

// ErrRepositoryNotFoundWithName creates an error for when a repository with the given name is not found.
func ErrRepositoryNotFoundWithName(name string) error {
	return fmt.Errorf("%w: %s", ErrRepositoryNotFound, name)
}
