package logger

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

var logger *logrus.Logger

// InitLogger initializes the global logger for CLI operations.
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

// GetLogger returns the configured logger instance.
func GetLogger() *logrus.Logger {
	if logger == nil {
		// Initialize with default settings if not already initialized
		InitLogger("info", false)
	}
	return logger
}

// Info logs an info message.
func Info(msg string, fields ...logrus.Fields) {
	entry := GetLogger().WithFields(mergeFields(fields...))
	entry.Info(msg)
}

// Infof logs a formatted info message.
func Infof(format string, args ...interface{}) {
	GetLogger().Infof(format, args...)
}

// InfofWithFields logs a formatted info message with fields.
func InfofWithFieldsf(fields logrus.Fields, format string, args ...interface{}) {
	GetLogger().WithFields(fields).Infof(format, args...)
}

// Debug logs a debug message (only shown when debug level is enabled).
func Debug(msg string, fields ...logrus.Fields) {
	entry := GetLogger().WithFields(mergeFields(fields...))
	entry.Debug(msg)
}

// Debugf logs a formatted debug message.
func Debugf(format string, args ...interface{}) {
	GetLogger().Debugf(format, args...)
}

// DebugfWithFields logs a formatted debug message with fields.
func DebugfWithFieldsf(fields logrus.Fields, format string, args ...interface{}) {
	GetLogger().WithFields(fields).Debugf(format, args...)
}

// Error logs an error message.
func Error(msg string, fields ...logrus.Fields) {
	entry := GetLogger().WithFields(mergeFields(fields...))
	entry.Error(msg)
}

// Errorf logs a formatted error message.
func Errorf(format string, args ...interface{}) {
	GetLogger().Errorf(format, args...)
}

// ErrorfWithFields logs a formatted error message with fields.
func ErrorfWithFieldsf(fields logrus.Fields, format string, args ...interface{}) {
	GetLogger().WithFields(fields).Errorf(format, args...)
}

// Warn logs a warning message.
func Warn(msg string, fields ...logrus.Fields) {
	entry := GetLogger().WithFields(mergeFields(fields...))
	entry.Warn(msg)
}

// Warnf logs a formatted warning message.
func Warnf(format string, args ...interface{}) {
	GetLogger().Warnf(format, args...)
}

// WarnfWithFields logs a formatted warning message with fields.
func WarnfWithFieldsf(fields logrus.Fields, format string, args ...interface{}) {
	GetLogger().WithFields(fields).Warnf(format, args...)
}

// Success logs a success message as info with success indicator.
func Success(msg string, fields ...logrus.Fields) {
	fields = append(fields, logrus.Fields{"status": "success"})
	entry := GetLogger().WithFields(mergeFields(fields...))
	entry.Info(msg)
}

// Successf logs a formatted success message.
func Successf(format string, args ...interface{}) {
	GetLogger().WithField("status", "success").Infof(format, args...)
}

// SuccessfWithFields logs a formatted success message with additional fields.
func SuccessfWithFieldsf(fields logrus.Fields, format string, args ...interface{}) {
	fields["status"] = "success"
	GetLogger().WithFields(fields).Infof(format, args...)
}

// mergeFields merges multiple logrus.Fields into one.
func mergeFields(fields ...logrus.Fields) logrus.Fields {
	result := make(logrus.Fields)
	for _, field := range fields {
		for k, v := range field {
			result[k] = v
		}
	}
	return result
}
