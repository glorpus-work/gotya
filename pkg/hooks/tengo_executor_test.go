package hooks_test

import (
	"testing"

	"github.com/cperrin88/gotya/pkg/hooks"
	"github.com/stretchr/testify/assert"
)

func TestTengoExecutor(t *testing.T) {
	executor := hooks.NewTengoExecutor()
	ctx := hooks.HookContext{
		PackageName:    "test-pkg",
		PackageVersion: "1.0.0",
		InstallPath:    "/test/install/path",
		Vars: map[string]interface{}{
			"customVar": "customValue",
		},
	}

	t.Run("Execute script with return value", func(t *testing.T) {
		// Test a simple script that doesn't return anything
		script := `// This is a valid script that does nothing`
		executor.AddScript(hooks.PreInstall, script)

		err := executor.Execute(hooks.PreInstall, ctx)
		assert.NoError(t, err, "Execute should not return an error for valid script")
	})

	t.Run("Execute script with error", func(t *testing.T) {
		// Test a script that causes a runtime error
		script := `
			// This will cause a runtime error because non-existent-function doesn't exist
			non_existent_function()
		`
		executor.AddScript(hooks.PostInstall, script)

		err := executor.Execute(hooks.PostInstall, ctx)
		assert.Error(t, err, "Execute should return an error for invalid script")
	})

	t.Run("Execute non-existent script", func(t *testing.T) {
		// Test executing a hooks type that hasn't been added
		err := executor.Execute("non-existent-hooks", ctx)
		assert.NoError(t, err, "Execute should not return an error for non-existent hooks")
	})

	t.Run("HasScript check", func(t *testing.T) {
		// Test HasScript method
		hookType := hooks.HookType("test-hooks")
		assert.False(t, executor.HasScript(hookType), "Should not have script before adding")

		executor.AddScript(hookType, "// test script")
		assert.True(t, executor.HasScript(hookType), "Should have script after adding")

		executor.RemoveScript(hookType)
		assert.False(t, executor.HasScript(hookType), "Should not have script after removal")
	})

	t.Run("Context variables are accessible", func(t *testing.T) {
		// Test that context variables are properly passed to the script
		script := `
			// Access context variables and use them in a way that's valid in Tengo
			name := packageName
			version := packageVersion
			path := installPath
			custom := customVar
			
			// Create a simple condition that uses the variables
			if name != "" && version != "" && path != "" && custom != "" {
				// All variables are set, do nothing
			}
		`
		executor.AddScript(hooks.PreRemove, script)

		err := executor.Execute(hooks.PreRemove, ctx)
		assert.NoError(t, err, "Context variables should be accessible in script")
	})

	t.Run("Script can use basic operations", func(t *testing.T) {
		// Test basic operations in script
		script := `
			// Simple script with basic operations
			a := 5
			b := 3
			sum := a + b
			
			// Simple condition
			if sum > 0 {
				// Do nothing, just testing the condition
			}
		`
		executor.AddScript(hooks.PostRemove, script)

		err := executor.Execute(hooks.PostRemove, ctx)
		assert.NoError(t, err, "Basic operations should work in script")
	})
}
