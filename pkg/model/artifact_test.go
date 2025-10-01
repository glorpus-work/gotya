package model

import (
	"testing"

	"github.com/glorpus-work/gotya/pkg/platform"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArtifact_MatchOs(t *testing.T) {
	tests := []struct {
		name     string
		pkgOS    string
		testOS   string
		expected bool
	}{
		{
			name:     "empty artifact OS matches any OS",
			pkgOS:    "",
			testOS:   "linux",
			expected: true,
		},
		{
			name:     "any OS matches any OS",
			pkgOS:    platform.AnyOS,
			testOS:   "windows",
			expected: true,
		},
		{
			name:     "matching OS returns true",
			pkgOS:    "linux",
			testOS:   "linux",
			expected: true,
		},
		{
			name:     "non-matching OS returns false",
			pkgOS:    "windows",
			testOS:   "linux",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &IndexArtifactDescriptor{OS: tt.pkgOS}
			result := pkg.MatchOs(tt.testOS)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestArtifact_MatchArch(t *testing.T) {
	tests := []struct {
		name     string
		pkgArch  string
		testArch string
		expected bool
	}{
		{
			name:     "empty artifact arch matches any arch",
			pkgArch:  "",
			testArch: "amd64",
			expected: true,
		},
		{
			name:     "any arch matches any arch",
			pkgArch:  platform.AnyArch,
			testArch: "arm64",
			expected: true,
		},
		{
			name:     "matching arch returns true",
			pkgArch:  "amd64",
			testArch: "amd64",
			expected: true,
		},
		{
			name:     "non-matching arch returns false",
			pkgArch:  "amd64",
			testArch: "arm64",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &IndexArtifactDescriptor{Arch: tt.pkgArch}
			result := pkg.MatchArch(tt.testArch)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestArtifact_MatchVersion(t *testing.T) {
	tests := []struct {
		name       string
		pkgVersion string
		constraint string
		expected   bool
	}{
		{
			name:       "exact version match",
			pkgVersion: "1.2.3",
			constraint: "= 1.2.3",
			expected:   true,
		},
		{
			name:       "exact version match without operator",
			pkgVersion: "1.2.3",
			constraint: "1.2.3",
			expected:   true,
		},
		{
			name:       "exact version match not matching",
			pkgVersion: "1.2.2",
			constraint: "1.2.3",
			expected:   false,
		},
		{
			name:       "exact version match without operator not matching",
			pkgVersion: "1.2.2",
			constraint: "1.2.3",
			expected:   false,
		},
		{
			name:       "version does not match constraint",
			pkgVersion: "1.2.3",
			constraint: ">= 2.0.0",
			expected:   false,
		},
		{
			name:       "version matches range constraint",
			pkgVersion: "1.2.3",
			constraint: ">= 1.0.0, < 2.0.0",
			expected:   true,
		},
		{
			name:       "invalid version string",
			pkgVersion: "not-a-version",
			constraint: ">= 1.0.0",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &IndexArtifactDescriptor{Version: tt.pkgVersion}
			result := pkg.MatchVersion(tt.constraint)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestArtifact_GetVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected *version.Version
	}{
		{
			name:     "valid version string",
			version:  "1.2.3",
			expected: version.Must(version.NewVersion("1.2.3")),
		},
		{
			name:     "empty version",
			version:  "",
			expected: nil,
		},
		{
			name:     "invalid version",
			version:  "not-a-version",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &IndexArtifactDescriptor{Version: tt.version}
			result := pkg.GetVersion()
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, 0, result.Compare(tt.expected))
			}
		})
	}
}

func TestArtifact_GetOS(t *testing.T) {
	tests := []struct {
		name     string
		pkgOS    string
		expected string
	}{
		{
			name:     "empty OS returns any",
			pkgOS:    "",
			expected: platform.AnyOS,
		},
		{
			name:     "specific OS returns as is",
			pkgOS:    "linux",
			expected: "linux",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &IndexArtifactDescriptor{OS: tt.pkgOS}
			result := pkg.GetOS()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestArtifact_GetArch(t *testing.T) {
	tests := []struct {
		name     string
		pkgArch  string
		expected string
	}{
		{
			name:     "empty arch returns any",
			pkgArch:  "",
			expected: platform.AnyArch,
		},
		{
			name:     "specific arch returns as is",
			pkgArch:  "amd64",
			expected: "amd64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg := &IndexArtifactDescriptor{Arch: tt.pkgArch}
			result := pkg.GetArch()
			assert.Equal(t, tt.expected, result)
		})
	}
}
