package errors

import (
	"errors"
	"strings"
	"testing"
)

func TestErrorVariables(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"ErrAlreadyExists", ErrAlreadyExists, "resource already exists"},
		{"ErrValidation", ErrValidation, "validation failed"},
		{"ErrIndexConflict", ErrIndexConflict, "index conflict"},
		{"ErrEmptyConfigPath", ErrEmptyConfigPath, "config file path cannot be empty"},
		{"ErrInvalidConfigPath", ErrInvalidConfigPath, "invalid config file path"},
		{"ErrConfigParse", ErrConfigParse, "failed to parse config"},
		{"ErrConfigValidation", ErrConfigValidation, "invalid configuration"},
		{"ErrConfigEncode", ErrConfigEncode, "failed to encode config"},
		{"ErrConfigDirectory", ErrConfigDirectory, "failed to create config directory"},
		{"ErrConfigFileCreate", ErrConfigFileCreate, "failed to create config file"},
		{"ErrConfigFileExists", ErrConfigFileExists, "configuration file already exists (use --force to overwrite)"},
		{"ErrConfigFileRename", ErrConfigFileRename, "failed to rename temporary config file"},
		{"ErrConfigFileChmod", ErrConfigFileChmod, "failed to set config file permissions"},
		{"ErrConfigMarshal", ErrConfigMarshal, "failed to marshal config to YAML"},
		{"ErrInvalidOSValue", ErrInvalidOSValue, "invalid OS value"},
		{"ErrInvalidArchValue", ErrInvalidArchValue, "invalid architecture value"},
		{"ErrNoArtifactsSpecified", ErrNoArtifactsSpecified, "no packages specified and --all flag not used"},
		{"ErrNoRepositories", ErrNoRepositories, "no repositories configured"},
		{"ErrArtifactNotFound", ErrArtifactNotFound, "artifact not found"},
		{"ErrEmptyRepositoryName", ErrEmptyRepositoryName, "index name cannot be empty"},
		{"ErrRepositoryURLEmpty", ErrRepositoryURLEmpty, "index URL cannot be empty"},
		{"ErrRepositoryExists", ErrRepositoryExists, "index already exists"},
		{"ErrFileNotFound", ErrFileNotFound, "file not found"},
		{"ErrInvalidPath", ErrInvalidPath, "invalid path"},
		{"ErrArtifactInvalid", ErrArtifactInvalid, "invalid artifact"},
		{"ErrHTTPTimeoutNegative", ErrHTTPTimeoutNegative, "http_timeout cannot be negative"},
		{"ErrCacheTTLNegative", ErrCacheTTLNegative, "cache_ttl cannot be negative"},
		{"ErrMaxConcurrentInvalid", ErrMaxConcurrentInvalid, "max_concurrent_syncs must be at least 1"},
		{"ErrInvalidOutputFormat", ErrInvalidOutputFormat, "invalid output format"},
		{"ErrInvalidLogLevel", ErrInvalidLogLevel, "invalid log level"},
		{"ErrInvalidBoolValue", ErrInvalidBoolValue, "invalid boolean value"},
		{"ErrUnknownConfigKey", ErrUnknownConfigKey, "unknown configuration key"},
		{"ErrRepositoryNotFound", ErrRepositoryNotFound, "repository not found"},
		{"ErrFileHashMismatch", ErrFileHashMismatch, "file hash mismatch"},
		{"ErrDownloadFailed", ErrDownloadFailed, "download failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("Expected error message %q, got %q", tt.expected, tt.err.Error())
			}
		})
	}
}

func TestWrap(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		msg      string
		expected string
	}{
		{
			name:     "wrap nil error",
			err:      nil,
			msg:      "additional context",
			expected: "",
		},
		{
			name:     "wrap standard error",
			err:      errors.New("original error"),
			msg:      "additional context",
			expected: "additional context: original error",
		},
		{
			name:     "wrap with empty message",
			err:      errors.New("original error"),
			msg:      "",
			expected: ": original error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Wrap(tt.err, tt.msg)
			if tt.err == nil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
				return
			}
			if result.Error() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result.Error())
			}
			// Test that the original error is wrapped
			if !errors.Is(result, tt.err) {
				t.Errorf("Expected wrapped error to contain original error")
			}
		})
	}
}

func TestWrapf(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		format   string
		args     []interface{}
		expected string
	}{
		{
			name:     "wrapf nil error",
			err:      nil,
			format:   "formatted: %s",
			args:     []interface{}{"test"},
			expected: "",
		},
		{
			name:     "wrapf standard error",
			err:      errors.New("original error"),
			format:   "failed to process %s",
			args:     []interface{}{"file.txt"},
			expected: "failed to process file.txt: original error",
		},
		{
			name:     "wrapf with multiple args",
			err:      errors.New("original error"),
			format:   "failed to process %s in %d attempts",
			args:     []interface{}{"file.txt", 3},
			expected: "failed to process file.txt in 3 attempts: original error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Wrapf(tt.err, tt.format, tt.args...)
			if tt.err == nil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
				return
			}
			if result.Error() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result.Error())
			}
			// Test that the original error is wrapped
			if !errors.Is(result, tt.err) {
				t.Errorf("Expected wrapped error to contain original error")
			}
		})
	}
}

func TestErrEmptyRepositoryNameWithIndex(t *testing.T) {
	tests := []struct {
		name     string
		index    int
		expected string
	}{
		{
			name:     "index 0",
			index:    0,
			expected: "index 0: index name cannot be empty",
		},
		{
			name:     "index 5",
			index:    5,
			expected: "index 5: index name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ErrEmptyRepositoryNameWithIndex(tt.index)
			if result.Error() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result.Error())
			}
			// Test that the result wraps ErrEmptyRepositoryName
			if !errors.Is(result, ErrEmptyRepositoryName) {
				t.Errorf("Expected error to wrap ErrEmptyRepositoryName")
			}
		})
	}
}

func TestErrRepositoryURLEmptyWithName(t *testing.T) {
	tests := []struct {
		name     string
		repoName string
		expected string
	}{
		{
			name:     "empty name",
			repoName: "",
			expected: "index '': index URL cannot be empty",
		},
		{
			name:     "main repo",
			repoName: "main",
			expected: "index 'main': index URL cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ErrRepositoryURLEmptyWithName(tt.repoName)
			if result.Error() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result.Error())
			}
			// Test that the result wraps ErrRepositoryURLEmpty
			if !errors.Is(result, ErrRepositoryURLEmpty) {
				t.Errorf("Expected error to wrap ErrRepositoryURLEmpty")
			}
		})
	}
}

func TestErrRepositoryExistsWithName(t *testing.T) {
	tests := []struct {
		name     string
		repoName string
		expected string
	}{
		{
			name:     "main repo",
			repoName: "main",
			expected: "index 'main': index already exists",
		},
		{
			name:     "custom repo",
			repoName: "custom",
			expected: "index 'custom': index already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ErrRepositoryExistsWithName(tt.repoName)
			if result.Error() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result.Error())
			}
			// Test that the result wraps ErrRepositoryExists
			if !errors.Is(result, ErrRepositoryExists) {
				t.Errorf("Expected error to wrap ErrRepositoryExists")
			}
		})
	}
}

func TestErrInvalidOSValueWithDetails(t *testing.T) {
	validOS := []string{"linux", "darwin", "windows"}
	tests := []struct {
		name     string
		value    string
		validOS  []string
		contains []string // parts that should be in the error message
	}{
		{
			name:     "invalid OS",
			value:    "invalidos",
			validOS:  validOS,
			contains: []string{"invalid OS value", "invalidos", "linux", "darwin", "windows"},
		},
		{
			name:     "empty value",
			value:    "",
			validOS:  validOS,
			contains: []string{"invalid OS value", "linux", "darwin", "windows"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ErrInvalidOSValueWithDetails(tt.value, tt.validOS)
			errorMsg := result.Error()

			// Test that the result wraps ErrInvalidOSValue
			if !errors.Is(result, ErrInvalidOSValue) {
				t.Errorf("Expected error to wrap ErrInvalidOSValue")
			}

			// Test that all expected parts are in the error message
			for _, part := range tt.contains {
				if !strings.Contains(errorMsg, part) {
					t.Errorf("Expected error message to contain %q, got %q", part, errorMsg)
				}
			}
		})
	}
}

func TestErrInvalidArchValueWithDetails(t *testing.T) {
	validArch := []string{"amd64", "386", "arm", "arm64"}
	tests := []struct {
		name      string
		value     string
		validArch []string
		contains  []string // parts that should be in the error message
	}{
		{
			name:      "invalid arch",
			value:     "invalidarch",
			validArch: validArch,
			contains:  []string{"invalid architecture value", "invalidarch", "amd64", "386", "arm", "arm64"},
		},
		{
			name:      "empty value",
			value:     "",
			validArch: validArch,
			contains:  []string{"invalid architecture value", "amd64", "386", "arm", "arm64"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ErrInvalidArchValueWithDetails(tt.value, tt.validArch)
			errorMsg := result.Error()

			// Test that the result wraps ErrInvalidArchValue
			if !errors.Is(result, ErrInvalidArchValue) {
				t.Errorf("Expected error to wrap ErrInvalidArchValue")
			}

			// Test that all expected parts are in the error message
			for _, part := range tt.contains {
				if !strings.Contains(errorMsg, part) {
					t.Errorf("Expected error message to contain %q, got %q", part, errorMsg)
				}
			}
		})
	}
}

func TestErrInvalidOutputFormatWithDetails(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		expected string
	}{
		{
			name:     "invalid format",
			format:   "xml",
			expected: "invalid output format: 'xml', must be one of: json, table, yaml",
		},
		{
			name:     "empty format",
			format:   "",
			expected: "invalid output format: '', must be one of: json, table, yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ErrInvalidOutputFormatWithDetails(tt.format)
			if result.Error() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result.Error())
			}
			// Test that the result wraps ErrInvalidOutputFormat
			if !errors.Is(result, ErrInvalidOutputFormat) {
				t.Errorf("Expected error to wrap ErrInvalidOutputFormat")
			}
		})
	}
}

func TestErrInvalidLogLevelWithDetails(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected string
	}{
		{
			name:     "invalid level",
			level:    "invalid",
			expected: "invalid log level: 'invalid', must be one of: panic, fatal, error, warn, info, debug, trace",
		},
		{
			name:     "empty level",
			level:    "",
			expected: "invalid log level: '', must be one of: panic, fatal, error, warn, info, debug, trace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ErrInvalidLogLevelWithDetails(tt.level)
			if result.Error() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result.Error())
			}
			// Test that the result wraps ErrInvalidLogLevel
			if !errors.Is(result, ErrInvalidLogLevel) {
				t.Errorf("Expected error to wrap ErrInvalidLogLevel")
			}
		})
	}
}

func TestErrRepositoryNotFoundWithName(t *testing.T) {
	tests := []struct {
		name     string
		repoName string
		expected string
	}{
		{
			name:     "main repo",
			repoName: "main",
			expected: "repository not found: main",
		},
		{
			name:     "custom repo",
			repoName: "custom",
			expected: "repository not found: custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ErrRepositoryNotFoundWithName(tt.repoName)
			if result.Error() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result.Error())
			}
			// Test that the result wraps ErrRepositoryNotFound
			if !errors.Is(result, ErrRepositoryNotFound) {
				t.Errorf("Expected error to wrap ErrRepositoryNotFound")
			}
		})
	}
}
