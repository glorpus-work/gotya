package platform

import (
	"fmt"
	"runtime"
	"strings"
)

// Platform represents a target platform with OS and Architecture
// Both OS and Arch can be "any" to match any platform
// or a specific value like "linux", "windows", "amd64", etc.
type Platform struct {
	OS   string `yaml:"os" json:"os"`
	Arch string `yaml:"arch" json:"arch"`
}

// CurrentPlatform returns the current platform (OS and architecture)
func CurrentPlatform() Platform {
	goos := runtime.GOOS
	if goos == "" {
		goos = "unknown"
	}

	goarch := runtime.GOARCH
	if goarch == "" {
		goarch = "unknown"
	}

	return Platform{
		OS:   NormalizeOS(goos),
		Arch: NormalizeArch(goarch),
	}
}

// Matches checks if this platform matches the target platform
// "any" is a wildcard that matches any value
func (p Platform) Matches(target Platform) bool {
	return (p.OS == "any" || target.OS == "any" || p.OS == target.OS) &&
		(p.Arch == "any" || target.Arch == "any" || p.Arch == target.Arch)
}

// String returns a string representation of the platform
func (p Platform) String() string {
	return fmt.Sprintf("%s/%s", p.OS, p.Arch)
}

// NormalizeOS normalizes OS names to a common format
func NormalizeOS(os string) string {
	os = strings.ToLower(os)
	// Map common variations to standard names
	switch os {
	case "darwin":
		return "macos"
	case "win", "windows":
		return "windows"
	case "linux":
		return "linux"
	case "freebsd", "openbsd", "netbsd":
		return os // Keep as is but normalized to lowercase
	default:
		return os
	}
}

// NormalizeArch normalizes architecture names to a common format
func NormalizeArch(arch string) string {
	arch = strings.ToLower(arch)
	// Map common variations to standard names
	switch arch {
	case "x86_64", "x64":
		return "amd64"
	case "x86", "i386", "i686":
		return "386"
	case "arm64", "aarch64":
		return "arm64"
	case "arm":
		return "arm" // Note: ARM version (v5, v6, v7) would need more context
	default:
		return arch
	}
}

// IsCompatible checks if the current platform is compatible with the target platform
func IsCompatible(targetOS, targetArch string) bool {
	current := CurrentPlatform()
	targetOS = NormalizeOS(targetOS)
	targetArch = NormalizeArch(targetArch)

	// Handle "any" wildcards
	osMatch := targetOS == "any" || current.OS == targetOS
	archMatch := targetArch == "any" || current.Arch == targetArch

	return osMatch && archMatch
}
