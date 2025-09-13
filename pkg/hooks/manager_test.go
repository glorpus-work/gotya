package hooks_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cperrin88/gotya/pkg/hooks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHookManager(t *testing.T) {
	manager := hooks.NewHookManager()
	assert.NotNil(t, manager, "NewHookManager should return a non-nil manager")
}

func TestAddAndExecuteHook(t *testing.T) {
	manager := hooks.NewHookManager()
	ctx := hooks.HookContext{
		PackageName:    "test-pkg",
		PackageVersion: "1.0.0",
		Vars: map[string]interface{}{
			"testVar": "testValue",
		},
	}

	tests := []struct {
		name          string
		hook          hooks.Hook
		expectedError string
	}{
		{
			name: "valid hooks",
			hook: hooks.Hook{
				Type:    hooks.PreInstall,
				Content: `// Simple hooks that doesn't return anything`,
			},
		},
		{
			name: "empty hooks type",
			hook: hooks.Hook{
				Type:    "",
				Content: "test content",
			},
			expectedError: hooks.ErrHookTypeEmpty.Error(),
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := manager.AddHook(testCase.hook)
			if testCase.expectedError != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", testCase.expectedError)
				}
				if !strings.Contains(err.Error(), testCase.expectedError) {
					t.Fatalf("expected error to contain %q, got %v", testCase.expectedError, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}

	// Test executing the hooks
	err := manager.Execute(hooks.PreInstall, ctx)
	require.NoError(t, err, "Execute should not return an error for valid hooks")
}

func TestHasHook(t *testing.T) {
	manager := hooks.NewHookManager()

	// Initially should not have the hooks
	assert.False(t, manager.HasHook(hooks.PreInstall), "Should not have hooks before adding")

	// Add the hooks
	err := manager.AddHook(hooks.Hook{
		Type:    hooks.PreInstall,
		Content: `// Test hooks`,
	})
	require.NoError(t, err)

	// Now should have the hooks
	assert.True(t, manager.HasHook(hooks.PreInstall), "Should have hooks after adding")
}

func TestRemoveHook(t *testing.T) {
	manager := hooks.NewHookManager()

	// Add a hooks
	err := manager.AddHook(hooks.Hook{
		Type:    hooks.PreInstall,
		Content: `// Test hooks`,
	})
	require.NoError(t, err)

	// Remove the hooks
	err = manager.RemoveHook(hooks.PreInstall)
	require.NoError(t, err, "RemoveHook should not return an error for existing hooks")

	// Should not have the hooks anymore
	assert.False(t, manager.HasHook(hooks.PreInstall), "Should not have hooks after removal")
}

func TestLoadHooksFromPackageDir(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	hooksDir := filepath.Join(tempDir, ".gotya", "hooks")
	err := os.MkdirAll(hooksDir, 0o750)
	require.NoError(t, err, "Failed to create hooks directory")

	// Create a test hooks file
	hookFile := filepath.Join(hooksDir, "pre-install.tengo")
	err = os.WriteFile(hookFile, []byte(`result = "Test hooks executed"`), 0o644)
	require.NoError(t, err, "Failed to create test hooks file")

	// Test loading hooks
	manager := hooks.NewHookManager()
	err = hooks.LoadHooksFromPackageDir(manager, tempDir)
	require.NoError(t, err, "LoadHooksFromPackageDir should not return an error")

	// Verify the hooks was loaded
	assert.True(t, manager.HasHook(hooks.PreInstall), "Should have loaded the pre-install hooks")
}

func TestHookTemplate(t *testing.T) {
	tests := []struct {
		name     string
		hookType hooks.HookType
		expected string
	}{
		{"PreInstall", hooks.PreInstall, "Pre-install hooks"},
		{"PostInstall", hooks.PostInstall, "Post-install hooks"},
		{"PreRemove", hooks.PreRemove, "Pre-remove hooks"},
		{"PostRemove", hooks.PostRemove, "Post-remove hooks"},
		{"Unknown", hooks.HookType("unknown"), "Unknown hooks type"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			template := hooks.HookTemplate(tc.hookType)
			assert.Contains(t, template, tc.expected, "Template should contain expected content")
		})
	}
}
