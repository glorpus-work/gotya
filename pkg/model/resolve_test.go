package model

import (
	"testing"
)

func TestResolvedArtifact_GetID(t *testing.T) {
	tests := []struct {
		name     string
		artifact ResolvedArtifact
		expected string
	}{
		{
			name: "basic artifact",
			artifact: ResolvedArtifact{
				Name:    "test-package",
				Version: "1.0.0",
			},
			expected: "test-package@1.0.0",
		},
		{
			name: "artifact with special characters",
			artifact: ResolvedArtifact{
				Name:    "test_package.with-dashes",
				Version: "2.1.0-alpha",
			},
			expected: "test_package.with-dashes@2.1.0-alpha",
		},
		{
			name: "empty name and version",
			artifact: ResolvedArtifact{
				Name:    "",
				Version: "",
			},
			expected: "@",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.artifact.GetID()
			if result != tt.expected {
				t.Errorf("GetID() = %v, want %v", result, tt.expected)
			}
		})
	}
}
