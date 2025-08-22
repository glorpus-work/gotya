package repository

import (
	"fmt"
	"sort"

	"github.com/cperrin88/gotya/pkg/platform"
)

// PlatformMatch represents how well a package matches the target platform.
type PlatformMatch int

const (
	// NoMatch means the package doesn't match the platform at all.
	NoMatch PlatformMatch = iota
	// AnyMatch means the package matches but has "any" for either OS or Arch.
	AnyMatch
	// ExactMatch means the package exactly matches both OS and Arch.
	ExactMatch
)

// packageMatch represents a package with its platform match score.
type packageMatch struct {
	pkg   Package
	score PlatformMatch
}

// ResolvePackage finds the best matching package for the target platform.
func ResolvePackage(pkgs []Package, targetOS, targetArch string) (*Package, error) {
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages available")
	}

	// If no target platform is specified, use the first package
	if targetOS == "" && targetArch == "" && len(pkgs) == 1 {
		return &pkgs[0], nil
	}

	// Find all matching packages
	var matches []packageMatch
	for _, pkg := range pkgs {
		match := getPlatformMatch(pkg, targetOS, targetArch)
		if match > NoMatch {
			matches = append(matches, packageMatch{pkg: pkg, score: match})
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no matching package found for platform %s/%s", targetOS, targetArch)
	}

	// Sort by match score (best match first)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})

	// Return the best match
	return &matches[0].pkg, nil
}

// getPlatformMatch determines how well a package matches the target platform.
func getPlatformMatch(pkg Package, targetOS, targetArch string) PlatformMatch {
	// If package has no platform specified, it matches any platform
	if pkg.OS == "" && pkg.Arch == "" {
		return AnyMatch
	}

	// Normalize platform strings
	pkgOS := platform.NormalizeOS(pkg.OS)
	pkgArch := platform.NormalizeArch(pkg.Arch)
	targetOS = platform.NormalizeOS(targetOS)
	targetArch = platform.NormalizeArch(targetArch)

	// Check for exact matches first
	osMatch := pkgOS == "any" || targetOS == "any" || pkgOS == targetOS
	archMatch := pkgArch == "any" || targetArch == "any" || pkgArch == targetArch

	if !osMatch || !archMatch {
		return NoMatch
	}

	// Determine match quality
	if (pkgOS == "any" || pkgArch == "any") && (pkgOS != "any" || pkgArch != "any") {
		return AnyMatch
	}

	return ExactMatch
}

// FilterPackages returns only packages that match the target platform.
func FilterPackages(pkgs []Package, targetOS, targetArch string) []Package {
	var result []Package
	for _, pkg := range pkgs {
		if getPlatformMatch(pkg, targetOS, targetArch) > NoMatch {
			result = append(result, pkg)
		}
	}
	return result
}
