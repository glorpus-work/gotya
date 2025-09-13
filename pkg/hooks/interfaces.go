package hooks

// HookManager defines the interface for managing hooks.
type HookManager interface {
	// Execute runs the specified hooks type with the given context
	Execute(hookType HookType, ctx HookContext) error

	// AddHook adds a new hooks
	AddHook(hook Hook) error

	// RemoveHook removes a hooks of the specified type
	RemoveHook(hookType HookType) error

	// HasHook checks if a hooks of the specified type exists
	HasHook(hookType HookType) bool
}
