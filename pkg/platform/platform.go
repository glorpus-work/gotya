package platform

import (
	"fmt"
	"runtime"
)

// Platform represents a target platform with OS and Architecture.
// Both OS and Arch can be "any" to match any platform.
type Platform struct {
	OS   string `yaml:"os"`
	Arch string `yaml:"arch"`
}

const (
	// AnyOS represents any possible OS
	AnyOS = "any"

	// AnyArch represents any possible architecture
	AnyArch = "any"
)

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
		OS:   goos,
		Arch: goarch,
	}
}

// Detect returns the current OS and architecture as strings.
// This is a convenience function that returns the same values as CurrentPlatform()
// but as separate return values for backward compatibility.
func Detect() (os, arch string) {
	p := CurrentPlatform()
	return p.OS, p.Arch
}

// Matches checks if this platform matches the target platform, considering "any" as a wildcard.
func (p Platform) Matches(target Platform) bool {
	return (p.OS == AnyOS || target.OS == AnyOS || p.OS == target.OS) &&
		(p.Arch == AnyArch || target.Arch == AnyArch || p.Arch == target.Arch)
}

// String returns a string representation of the platform.
func (p Platform) String() string {
	return fmt.Sprintf("%s/%s", p.OS, p.Arch)
}
