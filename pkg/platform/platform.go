package platform

import (
	"fmt"
	"runtime"
	"strings"
)

// Platform represents a target platform with OS and Architecture.
// Both OS and Arch can be "any" to match any platform.
type Platform struct {
	OS   string `yaml:"os" json:"os"`
	Arch string `yaml:"arch" json:"arch"`
}

// CurrentPlatform returns the current platform (OS and architecture).
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

// Detect returns the current OS and architecture as normalized strings.
// This is a convenience function that returns the same values as CurrentPlatform()
// but as separate return values for backward compatibility.
func Detect() (os, arch string) {
	p := CurrentPlatform()
	return p.OS, p.Arch
}

// "any" is a wildcard that matches any value.
func (p Platform) Matches(target Platform) bool {
	return (p.OS == "any" || target.OS == "any" || p.OS == target.OS) &&
		(p.Arch == "any" || target.Arch == "any" || p.Arch == target.Arch)
}

// String returns a string representation of the platform.
func (p Platform) String() string {
	return fmt.Sprintf("%s/%s", p.OS, p.Arch)
}

// NormalizeOS normalizes OS names to a common format.
func NormalizeOS(osName string) string {
	switch strings.ToLower(strings.TrimSpace(osName)) {
	case "darwin":
		return OSDarwin
	case "win", "windows":
		return OSWindows
	case "linux":
		return OSLinux
	case "freebsd", "openbsd", "netbsd":
		return strings.ToLower(strings.TrimSpace(osName)) // Keep as is but normalized to lowercase
	default:
		return strings.ToLower(strings.TrimSpace(osName))
	}
}

// NormalizeArch normalizes architecture names to a common format.
func NormalizeArch(arch string) string {
	switch strings.ToLower(strings.TrimSpace(arch)) {
	case "x86_64", "x64":
		return ArchAMD64
	case "i386", "i486", "i586", "i686":
		return Arch386
	case "aarch64":
		return ArchARM64
	case "armv6l", "armv7l":
		return ArchARM
	default:
		return strings.ToLower(strings.TrimSpace(arch))
	}
}

// IsCompatible checks if the current platform is compatible with the target platform.
func IsCompatible(targetOS, targetArch string) bool {
	current := CurrentPlatform()
	targetOS = NormalizeOS(targetOS)
	targetArch = NormalizeArch(targetArch)

	// Handle "any" wildcards
	osMatch := targetOS == "any" || current.OS == targetOS
	archMatch := targetArch == "any" || current.Arch == targetArch

	return osMatch && archMatch
}
