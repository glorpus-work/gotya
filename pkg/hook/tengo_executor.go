package hook

import (
	"fmt"
	"sync"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/stdlib"
)

// tengoExecutor handles the execution of Tengo scripts
type tengoExecutor struct {
	scripts map[HookType]string
	mutex   sync.RWMutex
}

// newTengoExecutor creates a new Tengo script executor
func newTengoExecutor() *tengoExecutor {
	return &tengoExecutor{
		scripts: make(map[HookType]string),
	}
}

// Execute runs the specified hook type with the given context
func (e *tengoExecutor) Execute(hookType HookType, ctx HookContext) error {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	script, exists := e.scripts[hookType]
	if !exists {
		return nil // No script for this hook type
	}

	// Create a new Tengo script
	s := tengo.NewScript([]byte(script))

	// Add standard library modules
	modules := stdlib.GetModuleMap("fmt", "os", "strings", "time")
	s.SetImports(modules)

	// Add context variables
	_ = s.Add("packageName", ctx.PackageName)
	_ = s.Add("packageVersion", ctx.PackageVersion)
	_ = s.Add("packagePath", ctx.PackagePath)
	_ = s.Add("installPath", ctx.InstallPath)

	// Add custom variables
	for k, v := range ctx.Vars {
		_ = s.Add(k, v)
	}

	// Run the script
	compiled, err := s.Run()
	if err != nil {
		return fmt.Errorf("failed to execute %s hook: %w", hookType, err)
	}

	// Check for any returned error
	if compiled.Get("err") != tengo.UndefinedValue {
		if errVal, ok := compiled.Get("err").(*tengo.Error); ok {
			return fmt.Errorf("hook script error: %s", errVal.String())
		}
	}

	return nil
}

// AddScript adds or updates a script for the specified hook type
func (e *tengoExecutor) AddScript(hookType HookType, script string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.scripts[hookType] = script
}

// RemoveScript removes the script for the specified hook type
func (e *tengoExecutor) RemoveScript(hookType HookType) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	delete(e.scripts, hookType)
}

// HasScript checks if a script exists for the specified hook type
func (e *tengoExecutor) HasScript(hookType HookType) bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	_, exists := e.scripts[hookType]
	return exists
}
