package repo

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNewIndex(t *testing.T) {
	formatVersion := "1.0"
	index := NewIndex(formatVersion)

	if index.FormatVersion != formatVersion {
		t.Errorf("Expected format version %s, got %s", formatVersion, index.FormatVersion)
	}

	if index.Packages == nil {
		t.Error("Expected packages slice to be initialized")
	}

	if len(index.Packages) != 0 {
		t.Errorf("Expected empty packages slice, got %d packages", len(index.Packages))
	}

	// Check that LastUpdate is recent (within last second)
	if time.Since(index.LastUpdate) > time.Second {
		t.Error("Expected LastUpdate to be recent")
	}
}

func TestParseIndex(t *testing.T) {
	tests := []struct {
		name      string
		jsonData  string
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid index",
			jsonData: `{
				"format_version": "1.0",
				"last_update": "2025-01-01T12:00:00Z",
				"packages": [
					{
						"name": "test-pkg",
						"version": "1.0.0",
						"description": "A test package",
						"url": "https://example.com/test.tar.gz",
						"checksum": "sha256:abc123",
						"size": 1024
					}
				]
			}`,
			expectErr: false,
		},
		{
			name: "missing format version",
			jsonData: `{
				"last_update": "2025-01-01T12:00:00Z",
				"packages": []
			}`,
			expectErr: true,
			errMsg:    "missing format version",
		},
		{
			name: "invalid json",
			jsonData: `{
				"format_version": "1.0"
				"invalid": json
			}`,
			expectErr: true,
			errMsg:    "failed to parse index",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index, err := ParseIndex([]byte(tt.jsonData))

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errMsg, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if index == nil {
				t.Error("Expected index to be non-nil")
			}
		})
	}
}

func TestParseIndexFromReader(t *testing.T) {
	jsonData := `{
		"format_version": "1.0",
		"last_update": "2025-01-01T12:00:00Z",
		"packages": []
	}`

	reader := strings.NewReader(jsonData)
	index, err := ParseIndexFromReader(reader)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if index.FormatVersion != "1.0" {
		t.Errorf("Expected format version 1.0, got %s", index.FormatVersion)
	}
}

func TestIndexToJSON(t *testing.T) {
	index := NewIndex("1.0")
	pkg := Package{
		Name:        "test-pkg",
		Version:     "1.0.0",
		Description: "Test package",
		URL:         "https://example.com/test.tar.gz",
		Checksum:    "sha256:abc123",
		Size:        1024,
	}
	index.AddPackage(pkg)

	data, err := index.ToJSON()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Parse back to verify it's valid JSON
	var parsed Index
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("Generated JSON is invalid: %v", err)
	}

	if parsed.FormatVersion != "1.0" {
		t.Errorf("Expected format version 1.0, got %s", parsed.FormatVersion)
	}

	if len(parsed.Packages) != 1 {
		t.Errorf("Expected 1 package, got %d", len(parsed.Packages))
	}
}

func TestIndexFindPackage(t *testing.T) {
	index := NewIndex("1.0")

	pkg1 := Package{Name: "package1", Version: "1.0.0"}
	pkg2 := Package{Name: "package2", Version: "2.0.0"}

	index.AddPackage(pkg1)
	index.AddPackage(pkg2)

	// Test finding existing package
	found := index.FindPackage("package1")
	if found == nil {
		t.Error("Expected to find package1")
	} else if found.Name != "package1" {
		t.Errorf("Expected package1, got %s", found.Name)
	}

	// Test finding non-existing package
	notFound := index.FindPackage("nonexistent")
	if notFound != nil {
		t.Error("Expected not to find nonexistent package")
	}
}

func TestIndexAddPackage(t *testing.T) {
	index := NewIndex("1.0")

	pkg := Package{
		Name:    "test-pkg",
		Version: "1.0.0",
	}

	// Add new package
	index.AddPackage(pkg)

	if len(index.Packages) != 1 {
		t.Errorf("Expected 1 package, got %d", len(index.Packages))
	}

	if index.Packages[0].Name != "test-pkg" {
		t.Errorf("Expected test-pkg, got %s", index.Packages[0].Name)
	}

	// Update existing package
	updatedPkg := Package{
		Name:    "test-pkg",
		Version: "2.0.0",
	}

	index.AddPackage(updatedPkg)

	// Should still have only 1 package (updated, not added)
	if len(index.Packages) != 1 {
		t.Errorf("Expected 1 package after update, got %d", len(index.Packages))
	}

	if index.Packages[0].Version != "2.0.0" {
		t.Errorf("Expected version 2.0.0, got %s", index.Packages[0].Version)
	}
}

func TestIndexRemovePackage(t *testing.T) {
	index := NewIndex("1.0")

	pkg1 := Package{Name: "package1", Version: "1.0.0"}
	pkg2 := Package{Name: "package2", Version: "2.0.0"}

	index.AddPackage(pkg1)
	index.AddPackage(pkg2)

	// Remove existing package
	removed := index.RemovePackage("package1")
	if !removed {
		t.Error("Expected RemovePackage to return true")
	}

	if len(index.Packages) != 1 {
		t.Errorf("Expected 1 package after removal, got %d", len(index.Packages))
	}

	if index.Packages[0].Name != "package2" {
		t.Errorf("Expected remaining package to be package2, got %s", index.Packages[0].Name)
	}

	// Try to remove non-existing package
	removed = index.RemovePackage("nonexistent")
	if removed {
		t.Error("Expected RemovePackage to return false for nonexistent package")
	}

	if len(index.Packages) != 1 {
		t.Errorf("Expected 1 package after failed removal, got %d", len(index.Packages))
	}
}

func TestPackageWithDependencies(t *testing.T) {
	index := NewIndex("1.0")

	pkg := Package{
		Name:         "complex-pkg",
		Version:      "1.0.0",
		Description:  "A complex package",
		Dependencies: []string{"dep1", "dep2"},
		Metadata: map[string]string{
			"author":  "Test Author",
			"license": "MIT",
		},
	}

	index.AddPackage(pkg)

	// Convert to JSON and back to test serialization
	data, err := index.ToJSON()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	parsed, err := ParseIndex(data)
	if err != nil {
		t.Errorf("Unexpected error parsing JSON: %v", err)
	}

	if len(parsed.Packages) != 1 {
		t.Errorf("Expected 1 package, got %d", len(parsed.Packages))
	}

	parsedPkg := parsed.Packages[0]
	if len(parsedPkg.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(parsedPkg.Dependencies))
	}

	if parsedPkg.Metadata["author"] != "Test Author" {
		t.Errorf("Expected author 'Test Author', got '%s'", parsedPkg.Metadata["author"])
	}
}

func TestIndexLastUpdateChanges(t *testing.T) {
	index := NewIndex("1.0")
	originalTime := index.LastUpdate

	// Wait a tiny bit to ensure time difference
	time.Sleep(time.Millisecond)

	// Adding package should update LastUpdate
	pkg := Package{Name: "test", Version: "1.0.0"}
	index.AddPackage(pkg)

	if !index.LastUpdate.After(originalTime) {
		t.Error("Expected LastUpdate to change after adding package")
	}

	updateTime := index.LastUpdate
	time.Sleep(time.Millisecond)

	// Removing package should update LastUpdate
	index.RemovePackage("test")

	if !index.LastUpdate.After(updateTime) {
		t.Error("Expected LastUpdate to change after removing package")
	}
}
