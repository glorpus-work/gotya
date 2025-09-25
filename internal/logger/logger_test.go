package logger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureOutput(t *testing.T, level string, fn func()) string {
	t.Helper()
	buf := &bytes.Buffer{}
	SetTestOutput(buf)
	defer UnsetTestOutput()

	// Reinitialize logger with test output
	logger = nil
	InitLogger(level)

	// Call the function that logs
	fn()

	// Get the output
	output := buf.String()
	return output
}

func TestLogger(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		logFn    func()
		contains []string
		excludes []string
	}{
		{
			name:  "info log",
			level: "info",
			logFn: func() {
				Info("test info message")
			},
			contains: []string{"test info message"},
		},
		{
			name:  "debug log with debug level",
			level: "debug",
			logFn: func() {
				Debug("test debug message")
			},
			contains: []string{"test debug message", "level=DEBUG"},
		},
		{
			name:  "debug log with info level",
			level: "info",
			logFn: func() {
				Debug("test debug message")
			},
			excludes: []string{"test debug message"},
		},
		{
			name:  "error log",
			level: "error",
			logFn: func() {
				Error("test error message")
			},
			contains: []string{"test error message", "level=ERROR"},
		},
		{
			name:  "warn log with fields",
			level: "warn",
			logFn: func() {
				Warn("test warning", Fields{"key1": "value1", "key2": 42})
			},
			contains: []string{"test warning", "level=WARN", "key1=value1", "key2=42"},
		},
		{
			name:  "success log",
			level: "info",
			logFn: func() {
				Success("operation completed")
			},
			contains: []string{"operation completed", "status=success"},
		},
		{
			name:  "formatted info log",
			level: "info",
			logFn: func() {
				Infof("formatted %s", "message")
			},
			contains: []string{"formatted message"},
		},
		{
			name:  "formatted debug with fields",
			level: "debug",
			logFn: func() {
				DebugfWithFields(Fields{"count": 1, "name": "test"}, "processing item %d", 1)
			},
			contains: []string{"processing item 1", "count=1", "name=test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(t, tt.level, tt.logFn)

			// Check for expected substrings
			for _, s := range tt.contains {
				assert.True(t, strings.Contains(output, s), "output should contain %q, got: %s", s, output)
			}

			// Check for excluded substrings
			for _, s := range tt.excludes {
				assert.False(t, strings.Contains(output, s), "output should not contain %q, got: %s", s, output)
			}
		})
	}
}

func TestGetLogger_InitializesIfNil(t *testing.T) {
	// Ensure logger is nil
	logger = nil
	buf := &bytes.Buffer{}
	SetTestOutput(buf)
	defer UnsetTestOutput()

	// This should initialize the logger with default settings
	l := GetLogger()
	require.NotNil(t, l)

	// Log a message
	l.Info("test message")
	output := buf.String()

	// Verify it's using the default level (info)
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "level=INFO")
}

func TestPlainOutput(t *testing.T) {
	output := captureOutput(t, "info", func() {
		Info("info message")
		Error("error message")
	})

	// Should not contain any color codes
	assert.Contains(t, output, "info message")
	assert.Contains(t, output, "error message")
	assert.False(t, strings.Contains(output, "\x1b"), "output should not contain ANSI color codes")
}

func TestMergeFields(t *testing.T) {
	tests := []struct {
		name   string
		fields []Fields
		expect []interface{}
	}{
		{
			name:   "no fields",
			fields: []Fields{},
			expect: []interface{}{},
		},
		{
			name: "single field",
			fields: []Fields{
				{"key1": "value1"},
			},
			expect: []interface{}{"key1", "value1"},
		},
		{
			name: "multiple fields",
			fields: []Fields{
				{"key1": "value1"},
				{"key2": 42, "key3": true},
			},
			expect: []interface{}{"key1", "value1", "key2", 42, "key3", true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeFields(tt.fields...)
			assert.ElementsMatch(t, tt.expect, result)
		})
	}
}
