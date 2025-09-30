package platform

import (
	"runtime"
	"testing"
)

func TestCurrentPlatform(t *testing.T) {
	platform := CurrentPlatform()

	// Test that we get a valid platform
	if platform.OS == "" {
		t.Error("Expected OS to be non-empty")
	}
	if platform.Arch == "" {
		t.Error("Expected Arch to be non-empty")
	}

	// Test that OS matches runtime.GOOS (no normalization now)
	if platform.OS != runtime.GOOS {
		t.Errorf("Expected OS %q, got %q", runtime.GOOS, platform.OS)
	}

	// Test that Arch matches runtime.GOARCH (no normalization now)
	if platform.Arch != runtime.GOARCH {
		t.Errorf("Expected Arch %q, got %q", runtime.GOARCH, platform.Arch)
	}
}

func TestDetect(t *testing.T) {
	os, arch := Detect()

	// Test that we get non-empty values
	if os == "" {
		t.Error("Expected OS to be non-empty")
	}
	if arch == "" {
		t.Error("Expected Arch to be non-empty")
	}

	// Test that values match CurrentPlatform
	platform := CurrentPlatform()
	if os != platform.OS {
		t.Errorf("Expected OS %q, got %q", platform.OS, os)
	}
	if arch != platform.Arch {
		t.Errorf("Expected Arch %q, got %q", platform.Arch, arch)
	}
}

func TestPlatformMatches(t *testing.T) {
	tests := []struct {
		name     string
		platform Platform
		target   Platform
		expected bool
	}{
		{
			name:     "exact match",
			platform: Platform{OS: "linux", Arch: "amd64"},
			target:   Platform{OS: "linux", Arch: "amd64"},
			expected: true,
		},
		{
			name:     "OS wildcard match",
			platform: Platform{OS: "linux", Arch: "amd64"},
			target:   Platform{OS: "any", Arch: "amd64"},
			expected: true,
		},
		{
			name:     "Arch wildcard match",
			platform: Platform{OS: "linux", Arch: "amd64"},
			target:   Platform{OS: "linux", Arch: "any"},
			expected: true,
		},
		{
			name:     "both wildcards match",
			platform: Platform{OS: "linux", Arch: "amd64"},
			target:   Platform{OS: "any", Arch: "any"},
			expected: true,
		},
		{
			name:     "OS mismatch",
			platform: Platform{OS: "linux", Arch: "amd64"},
			target:   Platform{OS: "darwin", Arch: "amd64"},
			expected: false,
		},
		{
			name:     "Arch mismatch",
			platform: Platform{OS: "linux", Arch: "amd64"},
			target:   Platform{OS: "linux", Arch: "386"},
			expected: false,
		},
		{
			name:     "both mismatch",
			platform: Platform{OS: "linux", Arch: "amd64"},
			target:   Platform{OS: "darwin", Arch: "386"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.platform.Matches(tt.target)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestPlatformString(t *testing.T) {
	tests := []struct {
		name     string
		platform Platform
		expected string
	}{
		{
			name:     "linux amd64",
			platform: Platform{OS: "linux", Arch: "amd64"},
			expected: "linux/amd64",
		},
		{
			name:     "darwin arm64",
			platform: Platform{OS: "darwin", Arch: "arm64"},
			expected: "darwin/arm64",
		},
		{
			name:     "windows 386",
			platform: Platform{OS: "windows", Arch: "386"},
			expected: "windows/386",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.platform.String()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
