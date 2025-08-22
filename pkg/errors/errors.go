package errors

import "fmt"

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
