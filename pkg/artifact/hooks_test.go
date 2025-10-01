package artifact

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookExecutor_ExecuteHook_ScriptNotFound(t *testing.T) {
	// Test that executing a non-existent hook script doesn't cause an error
	tempDir := t.TempDir()
	hookPath := filepath.Join(tempDir, "nonexistent-hook.tengo")

	hookExecutor := &HookExecutorImpl{}
	context := &HookContext{
		ArtifactName:    "test-artifact",
		ArtifactVersion: "1.0.0",
		Operation:       "test",
	}

	// Should not return an error for non-existent hook
	err := hookExecutor.ExecuteHook(hookPath, context)
	assert.Error(t, err)
}

func TestHookExecutor_ExecuteHook_ScriptExists(t *testing.T) {
	// Test that executing an existing hook script works
	tempDir := t.TempDir()
	hookPath := filepath.Join(tempDir, "test-hook.tengo")

	// Create a simple Tengo script that just returns true
	scriptContent := `
true
`
	err := os.WriteFile(hookPath, []byte(scriptContent), 0o644)
	require.NoError(t, err)

	hookExecutor := &HookExecutorImpl{}
	context := &HookContext{
		ArtifactName:    "test-artifact",
		ArtifactVersion: "1.0.0",
		Operation:       "test",
	}

	// Should execute successfully
	err = hookExecutor.ExecuteHook(hookPath, context)
	assert.NoError(t, err)
}

func TestHookExecutor_ExecuteHook_InvalidScript(t *testing.T) {
	// Test that executing an invalid hook script returns an error
	tempDir := t.TempDir()
	hookPath := filepath.Join(tempDir, "invalid-hook.tengo")

	// Create an invalid Tengo script
	scriptContent := `
invalid tengo syntax !!!
`
	err := os.WriteFile(hookPath, []byte(scriptContent), 0o644)
	require.NoError(t, err)

	hookExecutor := &HookExecutorImpl{}
	context := &HookContext{
		ArtifactName:    "test-artifact",
		ArtifactVersion: "1.0.0",
		Operation:       "test",
	}

	// Should return an error for invalid script
	err = hookExecutor.ExecuteHook(hookPath, context)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hook script execution failed")
}
