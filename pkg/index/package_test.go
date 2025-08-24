package index

import (
	"testing"

	"github.com/cperrin88/gotya/pkg/platform"
	"github.com/stretchr/testify/assert"
)

func TestPackage_MatchOs(t *testing.T) {
	tests := []struct {
		name     string
		pkgOS    string
		testOS   string
		expected bool
	}{
		{
			name:     "empty pkg OS matches any OS",
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
			pkg := &Package{OS: tt.pkgOS}
			result := pkg.MatchOs(tt.testOS)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPackage_MatchArch(t *testing.T) {
	tests := []struct {
		name     string
		pkgArch  string
		testArch string
		expected bool
	}{
		{
			name:     "empty pkg arch matches any arch",
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
			pkg := &Package{Arch: tt.pkgArch}
			result := pkg.MatchArch(tt.testArch)
			assert.Equal(t, tt.expected, result)
		})
	}
}
