package hook

import (
	"sync"

	"github.com/cperrin88/gotya/pkg/errors"
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

// Execute runs the specified hook type with the given context.
func (e *TengoExecutor) Execute(hookType HookType, ctx HookContext) error {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	script, exists := e.scripts[hookType]
	if !exists {
		return nil // No script for this hook type
	}

	// Create a new Tengo script
	scriptInstance := tengo.NewScript([]byte(script))

	// Add standard library modules
	modules := stdlib.GetModuleMap("fmt", "os", "strings", "time")
	scriptInstance.SetImports(modules)

	// Add context variables
	_ = scriptInstance.Add("packageName", ctx.PackageName)
	_ = scriptInstance.Add("packageVersion", ctx.PackageVersion)
	_ = scriptInstance.Add("packagePath", ctx.PackagePath)
	_ = scriptInstance.Add("installPath", ctx.InstallPath)

	// Add custom variables
	for k, v := range ctx.Vars {
		_ = scriptInstance.Add(k, v)
	}

	// Run the script
	compiled, err := scriptInstance.Run()
	if err != nil {
		return errors.Wrapf(errors.ErrHookExecution, "%s: %v", hookType, err)
	}

	// Check for any returned error
	errVar := compiled.Get("err")
	if errVar != nil {
		switch v := errVar.Value().(type) {
		case error:
			return errors.Wrap(errors.ErrHookScript, v.Error())
		case string:
			if v != "" {
				return errors.Wrap(errors.ErrHookScript, v)
			}
		}
	}

	return nil
}

// AddScript adds or updates a script for the specified hook type.
func (e *TengoExecutor) AddScript(hookType HookType, script string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.scripts[hookType] = script
}

// RemoveScript removes the script for the specified hook type.
func (e *TengoExecutor) RemoveScript(hookType HookType) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	delete(e.scripts, hookType)
}

// HasScript checks if a script exists for the specified hook type.
func (e *TengoExecutor) HasScript(hookType HookType) bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	_, exists := e.scripts[hookType]
	return exists
}
