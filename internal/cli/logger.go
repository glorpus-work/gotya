package cli

import (
	"io"
	"log/slog"
	"os"
)

var logger *slog.Logger

// InitLogger initializes the global logger for CLI operations
func InitLogger(verbose bool, noColor bool) {
	var level slog.Level
	var output io.Writer = os.Stdout

	if verbose {
		level = slog.LevelDebug
	} else {
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if noColor {
		handler = slog.NewTextHandler(output, opts)
	} else {
		handler = slog.NewJSONHandler(output, opts)
	}

	logger = slog.New(handler)
}

// GetLogger returns the configured logger instance
func GetLogger() *slog.Logger {
	if logger == nil {
		// Initialize with default settings if not already initialized
		InitLogger(false, false)
	}
	return logger
}

// Info logs an info message
func Info(msg string, args ...any) {
	GetLogger().Info(msg, args...)
}

// Debug logs a debug message (only shown when verbose is enabled)
func Debug(msg string, args ...any) {
	GetLogger().Debug(msg, args...)
}

// Error logs an error message
func Error(msg string, args ...any) {
	GetLogger().Error(msg, args...)
}

// Success logs a success message as info
func Success(msg string, args ...any) {
	GetLogger().Info(msg, args...)
}
