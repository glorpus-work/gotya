package hooks

import (
	"fmt"
	"sync"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/stdlib"
)

// TengoExecutor handles the execution of Tengo scripts.
type TengoExecutor struct {
	scripts map[HookType]string
	mutex   sync.RWMutex
}

// NewTengoExecutor creates a new Tengo script executor.
func NewTengoExecutor() *TengoExecutor {
	return &TengoExecutor{
		scripts: make(map[HookType]string),
	}
}

// Execute runs the specified hooks type with the given context.
func (e *TengoExecutor) Execute(hookType HookType, ctx HookContext) error {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	script, exists := e.scripts[hookType]
	if !exists {
		return nil // No script for this hooks type
	}

	// Create a new Tengo script
	scriptInstance := tengo.NewScript([]byte(script))

	// Add standard library modules
	modules := stdlib.GetModuleMap("fmt", "os", "strings", "time")
	scriptInstance.SetImports(modules)

	// Add context variables
	if err := scriptInstance.Add("packageName", ctx.PackageName); err != nil {
		return fmt.Errorf("failed to add packageName to script: %w", err)
	}
	if err := scriptInstance.Add("packageVersion", ctx.PackageVersion); err != nil {
		return fmt.Errorf("failed to add packageVersion to script: %w", err)
	}
	if err := scriptInstance.Add("packagePath", ctx.PackagePath); err != nil {
		return fmt.Errorf("failed to add packagePath to script: %w", err)
	}
	if err := scriptInstance.Add("installPath", ctx.InstallPath); err != nil {
		return fmt.Errorf("failed to add installPath to script: %w", err)
	}

	// Add custom variables
	for k, v := range ctx.Vars {
		if err := scriptInstance.Add(k, v); err != nil {
			return fmt.Errorf("failed to add variable '%s' to script: %w", k, err)
		}
	}

	// Run the script
	compiled, err := scriptInstance.Run()
	if err != nil {
		return fmt.Errorf("%s: %w: %w", hookType, ErrHookExecution, err)
	}

	// Check for any returned error
	errVar := compiled.Get("err")
	if errVar != nil {
		switch v := errVar.Value().(type) {
		case error:
			return fmt.Errorf("%w: %w", ErrHookScript, v)
		case string:
			if v != "" {
				return fmt.Errorf("%w: %s", ErrHookScript, v)
			}
		}
	}

	return nil
}

// AddScript adds or updates a script for the specified hooks type.
func (e *TengoExecutor) AddScript(hookType HookType, script string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.scripts[hookType] = script
}

// RemoveScript removes the script for the specified hooks type.
func (e *TengoExecutor) RemoveScript(hookType HookType) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	delete(e.scripts, hookType)
}

// HasScript checks if a script exists for the specified hooks type.
func (e *TengoExecutor) HasScript(hookType HookType) bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	_, exists := e.scripts[hookType]
	return exists
}
