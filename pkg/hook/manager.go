package hook

import (
	"sync"

	"github.com/cperrin88/gotya/pkg/errors"
)

// DefaultHookManager is the default implementation of HookManager.
type DefaultHookManager struct {
	executor *TengoExecutor
	mutex    sync.RWMutex
}

// NewHookManager creates a new hook manager.
func NewHookManager() *DefaultHookManager {
	return &DefaultHookManager{
		executor: NewTengoExecutor(),
	}
}

// Execute runs the specified hook type with the given context.
func (m *DefaultHookManager) Execute(hookType HookType, ctx HookContext) error {
	if !m.HasHook(hookType) {
		return nil // No hook registered for this type
	}

	// Copy the context to prevent modifications
	ctxCopy := ctx
	if ctxCopy.Vars == nil {
		ctxCopy.Vars = make(map[string]interface{})
	}

	return m.executor.Execute(hookType, ctxCopy)
}

// AddHook adds a new hook.
func (m *DefaultHookManager) AddHook(hook Hook) error {
	if hook.Type == "" {
		return ErrHookTypeEmpty
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.executor.AddScript(hook.Type, hook.Content)
	return nil
}

// RemoveHook removes a hook of the specified type.
func (m *DefaultHookManager) RemoveHook(hookType HookType) error {
	if hookType == "" {
		return ErrHookTypeEmpty
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.executor.RemoveScript(hookType)
	return nil
}

// HasHook checks if a hook of the specified type exists.
func (m *DefaultHookManager) HasHook(hookType HookType) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.executor.HasScript(hookType)
}

// LoadHooksFromPackage loads hooks from a package directory.
func (m *DefaultHookManager) LoadHooksFromPackage(packagePath string) error {
	// Implementation for loading hooks from package directory
	// This would typically look for hook scripts in a specific directory structure
	// For example: <packagePath>/hooks/pre-install.tengo
	return nil
}

// ExecuteAll executes all registered hooks in the order they were added.
func (m *DefaultHookManager) ExecuteAll(ctx HookContext) error {
	hooks := []HookType{PreInstall, PostInstall, PreRemove, PostRemove}

	for _, hookType := range hooks {
		if m.HasHook(hookType) {
			if err := m.Execute(hookType, ctx); err != nil {
				return errors.Wrapf(err, "error executing hook %s", hookType)
			}
		}
	}

	return nil
}
