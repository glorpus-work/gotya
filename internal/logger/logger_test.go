package logger

import (
	"bytes"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
)

func captureOutput(t *testing.T, level string, format OutputFormat, fn func()) string {
	t.Helper()
	buf := &bytes.Buffer{}
	SetTestOutput(buf)
	defer UnsetTestOutput()

	// Reinitialize logger with test output
	logger = nil
	InitLogger(level, format)

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
			// Test text format
			testOutput := captureOutput(t, tt.level, FormatText, tt.logFn)
			for _, want := range tt.contains {
				assert.Contains(t, testOutput, want, "text log output should contain expected message")
			}
			for _, notWant := range tt.excludes {
				assert.NotContains(t, testOutput, notWant, "text log output should not contain excluded message")
			}

			// Test JSON format if this is not a format-specific test
			if !strings.HasPrefix(tt.name, "json format") {
				jsonOutput := captureOutput(t, tt.level, FormatJSON, tt.logFn)
				// For JSON output, we need to adjust our expectations
				// as the format is different from text output
				for _, want := range tt.contains {
					// For key=value pairs in the text output, we need to look for "key":value in JSON
					if strings.Contains(want, "=") {
						parts := strings.SplitN(want, "=", 2)
						key := parts[0]
						value := parts[1]
						// Handle different value types in JSON
						if value == "true" || value == "false" || unicode.IsDigit(rune(value[0])) {
							// For boolean or numeric values, don't add quotes
							assert.Contains(t, jsonOutput, `"`+key+`":`+value, "JSON log output should contain expected field")
						} else {
							// For string values, add quotes
							assert.Contains(t, jsonOutput, `"`+key+`":"`+value+`"`, "JSON log output should contain expected field")
						}
					} else {
						// For non-key=value strings, just check if they're in the message
						assert.Contains(t, jsonOutput, want, "JSON log output should contain expected message")
					}
				}
				for _, notWant := range tt.excludes {
					if strings.Contains(notWant, "=") {
						parts := strings.SplitN(notWant, "=", 2)
						key := parts[0]
						value := parts[1]
						// For key=value pairs, check both key and value separately
						assert.NotContains(t, jsonOutput, `"`+key+`":"`+value+`"`, "JSON log output should not contain excluded field")
					} else {
						assert.NotContains(t, jsonOutput, notWant, "JSON log output should not contain excluded message")
					}
				}
			}
		})
	}
}

func TestGetLogger_InitializesIfNil(t *testing.T) {
	logger = nil
	assert.NotPanics(t, func() {
		lg := GetLogger()
		assert.NotNil(t, lg)
		lg.Info("test message")
	})
}

func TestSetOutputFormat(t *testing.T) {
	// Test switching from text to JSON
	buf := &bytes.Buffer{}
	SetTestOutput(buf)
	defer UnsetTestOutput()

	// Initialize with text format
	logger = nil
	InitLogger("debug", FormatText)
	Info("test message 1")
	output := buf.String()
	assert.Contains(t, output, "test message 1")
	assert.Contains(t, output, "INFO")

	// Clear buffer and switch to JSON
	buf.Reset()
	SetOutputFormat(FormatJSON)
	Info("test message 2")
	jsonOutput := buf.String()
	assert.Contains(t, jsonOutput, `"msg":"test message 2"`)
	assert.Contains(t, jsonOutput, `"level":"INFO"`)
}

func TestPlainOutput(t *testing.T) {
	// Test text format
	output := captureOutput(t, "info", FormatText, func() {
		Info("test message")
	})
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "INFO")

	// Test JSON format
	jsonOutput := captureOutput(t, "info", FormatJSON, func() {
		Info("test message")
	})
	assert.Contains(t, jsonOutput, `"msg":"test message"`)
	assert.Contains(t, jsonOutput, `"level":"INFO"`)
}

func TestMergeFields(t *testing.T) {
	tests := []struct {
		name   string
		fields []Fields
		expect map[string]interface{}
	}{
		{
			name:   "single field",
			fields: []Fields{{"key1": "value1"}},
			expect: map[string]interface{}{"key1": "value1"},
		},
		{
			name:   "multiple fields",
			fields: []Fields{{"key1": "value1"}, {"key2": 123, "key3": true}},
			expect: map[string]interface{}{"key1": "value1", "key2": 123, "key3": true},
		},
		{
			name:   "overwrite fields",
			fields: []Fields{{"key1": "value1"}, {"key1": "new value", "key2": 123}},
			expect: map[string]interface{}{"key1": "new value", "key2": 123},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := mergeFields(tt.fields...)
			result := make(map[string]interface{})
			for i := 0; i < len(attrs); i += 2 {
				key := attrs[i].(string)
				result[key] = attrs[i+1]
			}
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestJSONFormat(t *testing.T) {
	// Test with fields to ensure they're properly serialized to JSON
	output := captureOutput(t, "info", FormatJSON, func() {
		Info("test json message", Fields{
			"key1":   "value1",
			"number": 42,
			"bool":   true,
		})
	})

	// Check for JSON structure
	assert.Contains(t, output, `"msg":"test json message"`)
	assert.Contains(t, output, `"level":"INFO"`)
	assert.Contains(t, output, `"key1":"value1"`)
	assert.Contains(t, output, `"number":42`)
	assert.Contains(t, output, `"bool":true`)
}
