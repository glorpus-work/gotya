package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

var (
	// testOutput is used to capture log output during tests
	testOutput   io.Writer
	testOutputMu sync.Mutex
)

// Fields is a type alias for log fields to make the API cleaner
type Fields map[string]interface{}

var logger *slog.Logger

// InitLogger initializes the global logger for CLI operations.
// SetTestOutput sets the output writer for testing purposes
func SetTestOutput(w io.Writer) {
	testOutputMu.Lock()
	defer testOutputMu.Unlock()
	testOutput = w
}

// UnsetTestOutput resets the test output to nil
func UnsetTestOutput() {
	testOutputMu.Lock()
	defer testOutputMu.Unlock()
	testOutput = nil
}

func getOutput() io.Writer {
	testOutputMu.Lock()
	defer testOutputMu.Unlock()
	if testOutput != nil {
		return testOutput
	}
	return os.Stdout
}

func InitLogger(logLevel string) {
	var level slog.Level
	switch strings.ToLower(logLevel) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo // fallback to info level
	}

	// Configure handler
	handler := slog.NewTextHandler(getOutput(), &slog.HandlerOptions{
		Level: level,
	})

	logger = slog.New(handler)
}

// GetLogger returns the configured logger instance.
func GetLogger() *slog.Logger {
	if logger == nil {
		// Initialize with default settings if not already initialized
		InitLogger("info")
	}
	return logger
}

// Info logs an info message.
func Info(msg string, fields ...Fields) {
	attrs := mergeFields(fields...)
	GetLogger().Info(msg, attrs...)
}

// Infof logs a formatted info message.
func Infof(format string, args ...interface{}) {
	GetLogger().Info(fmt.Sprintf(format, args...))
}

// InfofWithFields logs a formatted info message with fields.
func InfofWithFields(fields Fields, format string, args ...interface{}) {
	attrs := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		attrs = append(attrs, k, v)
	}
	GetLogger().Info(fmt.Sprintf(format, args...), attrs...)
}

// Debug logs a debug message (only shown when debug level is enabled).
func Debug(msg string, fields ...Fields) {
	attrs := mergeFields(fields...)
	GetLogger().Debug(msg, attrs...)
}

// Debugf logs a formatted debug message.
func Debugf(format string, args ...interface{}) {
	GetLogger().Debug(fmt.Sprintf(format, args...))
}

// DebugfWithFields logs a formatted debug message with fields.
func DebugfWithFields(fields Fields, format string, args ...interface{}) {
	attrs := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		attrs = append(attrs, k, v)
	}
	GetLogger().Debug(fmt.Sprintf(format, args...), attrs...)
}

// Error logs an error message.
func Error(msg string, fields ...Fields) {
	attrs := mergeFields(fields...)
	GetLogger().Error(msg, attrs...)
}

// Errorf logs a formatted error message.
func Errorf(format string, args ...interface{}) {
	GetLogger().Error(fmt.Sprintf(format, args...))
}

// ErrorfWithFields logs a formatted error message with fields.
func ErrorfWithFields(fields Fields, format string, args ...interface{}) {
	attrs := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		attrs = append(attrs, k, v)
	}
	GetLogger().Error(fmt.Sprintf(format, args...), attrs...)
}

// Warn logs a warning message.
func Warn(msg string, fields ...Fields) {
	attrs := mergeFields(fields...)
	GetLogger().Warn(msg, attrs...)
}

// Warnf logs a formatted warning message.
func Warnf(format string, args ...interface{}) {
	GetLogger().Warn(fmt.Sprintf(format, args...))
}

// WarnfWithFields logs a formatted warning message with fields.
func WarnfWithFields(fields Fields, format string, args ...interface{}) {
	attrs := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		attrs = append(attrs, k, v)
	}

	GetLogger().Warn(fmt.Sprintf(format, args...), attrs...)
}

// Success logs a success message as info with success indicator.
func Success(msg string, fields ...Fields) {
	allFields := mergeFields(fields...)
	allFields = append(allFields, "status", "success")
	GetLogger().Info(msg, allFields...)
}

// Successf logs a formatted success message.
func Successf(format string, args ...interface{}) {
	GetLogger().Info(fmt.Sprintf("SUCCESS: "+format, args...))
}

// SuccessfWithFields logs a formatted success message with additional fields.
func SuccessfWithFields(fields Fields, format string, args ...interface{}) {
	attrs := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		attrs = append(attrs, k, v)
	}
	GetLogger().Info(fmt.Sprintf("SUCCESS: "+format, args...), attrs...)
}

// mergeFields merges multiple field maps into one slice of key-value pairs for slog.
func mergeFields(fields ...Fields) []interface{} {
	result := []interface{}{}
	for _, field := range fields {
		for k, v := range field {
			result = append(result, k, v)
		}
	}
	return result
}
