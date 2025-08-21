package logger

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

var logger *logrus.Logger

// InitLogger initializes the global logger for CLI operations
func InitLogger(logLevel string, noColor bool) {
	logger = logrus.New()
	logger.SetOutput(os.Stdout)

	// Parse log level
	level, err := logrus.ParseLevel(strings.ToLower(logLevel))
	if err != nil {
		level = logrus.InfoLevel // fallback to info level
	}
	logger.SetLevel(level)

	// Configure formatter
	if noColor {
		logger.SetFormatter(&logrus.TextFormatter{
			DisableColors: true,
			FullTimestamp: false,
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			ForceColors:   true,
			FullTimestamp: false,
		})
	}
}

// GetLogger returns the configured logger instance
func GetLogger() *logrus.Logger {
	if logger == nil {
		// Initialize with default settings if not already initialized
		InitLogger("info", false)
	}
	return logger
}

// Info logs an info message
func Info(msg string, fields ...logrus.Fields) {
	entry := GetLogger().WithFields(mergeFields(fields...))
	entry.Info(msg)
}

// Debug logs a debug message (only shown when debug level is enabled)
func Debug(msg string, fields ...logrus.Fields) {
	entry := GetLogger().WithFields(mergeFields(fields...))
	entry.Debug(msg)
}

// Error logs an error message
func Error(msg string, fields ...logrus.Fields) {
	entry := GetLogger().WithFields(mergeFields(fields...))
	entry.Error(msg)
}

// Warn logs a warning message
func Warn(msg string, fields ...logrus.Fields) {
	entry := GetLogger().WithFields(mergeFields(fields...))
	entry.Warn(msg)
}

// Success logs a success message as info with success indicator
func Success(msg string, fields ...logrus.Fields) {
	mergedFields := mergeFields(fields...)
	mergedFields["status"] = "success"
	GetLogger().WithFields(mergedFields).Info(msg)
}

// mergeFields merges multiple logrus.Fields into one
func mergeFields(fields ...logrus.Fields) logrus.Fields {
	result := make(logrus.Fields)
	for _, field := range fields {
		for k, v := range field {
			result[k] = v
		}
	}
	return result
}
