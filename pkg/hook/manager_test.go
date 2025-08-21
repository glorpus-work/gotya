package hook_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cperrin88/gotya/pkg/hook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHookManager(t *testing.T) {
	manager := hook.NewHookManager()
	assert.NotNil(t, manager, "NewHookManager should return a non-nil manager")
}

func TestAddAndExecuteHook(t *testing.T) {
	manager := hook.NewHookManager()
	ctx := hook.HookContext{
		PackageName:    "test-pkg",
		PackageVersion: "1.0.0",
		Vars: map[string]interface{}{
			"testVar": "testValue",
		},
	}

	// Test adding a valid hook
	err := manager.AddHook(hook.Hook{
		Type:    hook.PreInstall,
		Content: `// Simple hook that doesn't return anything`,
	})
	require.NoError(t, err, "AddHook should not return an error for valid hook")

	// Test executing the hook
	err = manager.Execute(hook.PreInstall, ctx)
	require.NoError(t, err, "Execute should not return an error for valid hook")
}

func TestHasHook(t *testing.T) {
	manager := hook.NewHookManager()

	// Initially should not have the hook
	assert.False(t, manager.HasHook(hook.PreInstall), "Should not have hook before adding")

	// Add the hook
	err := manager.AddHook(hook.Hook{
		Type:    hook.PreInstall,
		Content: `// Test hook`,
	})
	require.NoError(t, err)

	// Now should have the hook
	assert.True(t, manager.HasHook(hook.PreInstall), "Should have hook after adding")
}

func TestRemoveHook(t *testing.T) {
	manager := hook.NewHookManager()

	// Add a hook
	err := manager.AddHook(hook.Hook{
		Type:    hook.PreInstall,
		Content: `// Test hook`,
	})
	require.NoError(t, err)

	// Remove the hook
	err = manager.RemoveHook(hook.PreInstall)
	require.NoError(t, err, "RemoveHook should not return an error for existing hook")

	// Should not have the hook anymore
	assert.False(t, manager.HasHook(hook.PreInstall), "Should not have hook after removal")
}

func TestLoadHooksFromPackageDir(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	hooksDir := filepath.Join(tempDir, ".gotya", "hooks")
	err := os.MkdirAll(hooksDir, 0755)
	require.NoError(t, err, "Failed to create hooks directory")

	// Create a test hook file
	hookFile := filepath.Join(hooksDir, "pre-install.tengo")
	err = os.WriteFile(hookFile, []byte(`result = "Test hook executed"`), 0644)
	require.NoError(t, err, "Failed to create test hook file")

	// Test loading hooks
	manager := hook.NewHookManager()
	err = hook.LoadHooksFromPackageDir(manager, tempDir)
	require.NoError(t, err, "LoadHooksFromPackageDir should not return an error")

	// Verify the hook was loaded
	assert.True(t, manager.HasHook(hook.PreInstall), "Should have loaded the pre-install hook")
}

func TestHookTemplate(t *testing.T) {
	tests := []struct {
		name     string
		hookType hook.HookType
		expected string
	}{
		{"PreInstall", hook.PreInstall, "Pre-install hook"},
		{"PostInstall", hook.PostInstall, "Post-install hook"},
		{"PreRemove", hook.PreRemove, "Pre-remove hook"},
		{"PostRemove", hook.PostRemove, "Post-remove hook"},
		{"Unknown", hook.HookType("unknown"), "Unknown hook type"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			template := hook.HookTemplate(tc.hookType)
			assert.Contains(t, template, tc.expected, "Template should contain expected content")
		})
	}
}
