package errors

import (
	"errors"
	"testing"
)

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
