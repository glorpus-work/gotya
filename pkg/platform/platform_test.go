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

	// Test that OS is normalized
	expectedOS := NormalizeOS(runtime.GOOS)
	if platform.OS != expectedOS {
		t.Errorf("Expected OS %q, got %q", expectedOS, platform.OS)
	}

	// Test that Arch is normalized
	expectedArch := NormalizeArch(runtime.GOARCH)
	if platform.Arch != expectedArch {
		t.Errorf("Expected Arch %q, got %q", expectedArch, platform.Arch)
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

func TestNormalizeOS(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"darwin lowercase", "darwin", "darwin"},
		{"darwin uppercase", "DARWIN", "darwin"},
		{"darwin with spaces", " darwin ", "darwin"},
		{"linux lowercase", "linux", "linux"},
		{"linux uppercase", "LINUX", "linux"},
		{"win to windows", "win", "windows"},
		{"windows", "windows", "windows"},
		{"freebsd", "freebsd", "freebsd"},
		{"openbsd", "openbsd", "openbsd"},
		{"netbsd", "netbsd", "netbsd"},
		{"unknown OS", "unknownos", "unknownos"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeOS(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeOS(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeArch(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"amd64", "amd64", "amd64"},
		{"x86_64 to amd64", "x86_64", "amd64"},
		{"x64 to amd64", "x64", "amd64"},
		{"386", "386", "386"},
		{"i386 to 386", "i386", "386"},
		{"i486 to 386", "i486", "386"},
		{"i586 to 386", "i586", "386"},
		{"i686 to 386", "i686", "386"},
		{"aarch64 to arm64", "aarch64", "arm64"},
		{"armv6l to arm", "armv6l", "arm"},
		{"armv7l to arm", "armv7l", "arm"},
		{"arm64 uppercase", "ARM64", "arm64"},
		{"unknown arch", "unknownarch", "unknownarch"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeArch(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeArch(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsCompatible(t *testing.T) {
	tests := []struct {
		name        string
		targetOS    string
		targetArch  string
		currentOS   string
		currentArch string
		expected    bool
	}{
		{
			name:        "exact match",
			targetOS:    "linux",
			targetArch:  "amd64",
			currentOS:   "linux",
			currentArch: "amd64",
			expected:    true,
		},
		{
			name:        "OS wildcard",
			targetOS:    "any",
			targetArch:  "amd64",
			currentOS:   "linux",
			currentArch: "amd64",
			expected:    true,
		},
		{
			name:        "Arch wildcard",
			targetOS:    "linux",
			targetArch:  "any",
			currentOS:   "linux",
			currentArch: "amd64",
			expected:    true,
		},
		{
			name:        "OS mismatch",
			targetOS:    "darwin",
			targetArch:  "amd64",
			currentOS:   "linux",
			currentArch: "amd64",
			expected:    false,
		},
		{
			name:        "Arch mismatch",
			targetOS:    "linux",
			targetArch:  "386",
			currentOS:   "linux",
			currentArch: "amd64",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the current platform for testing
			originalCurrentPlatform := currentPlatformFunc
			currentPlatformFunc = func() Platform {
				return Platform{OS: tt.currentOS, Arch: tt.currentArch}
			}
			defer func() { currentPlatformFunc = originalCurrentPlatform }()

			result := IsCompatible(tt.targetOS, tt.targetArch)
			if result != tt.expected {
				t.Errorf("IsCompatible(%q, %q) = %v, expected %v", tt.targetOS, tt.targetArch, result, tt.expected)
			}
		})
	}
}

// currentPlatformFunc allows us to mock CurrentPlatform for testing
var currentPlatformFunc = CurrentPlatform
