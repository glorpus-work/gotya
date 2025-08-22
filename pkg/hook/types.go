package hook

// HookType represents the type of hook.
type HookType string

// Supported hook types.
const (
	PreInstall  HookType = "pre-install"
	PostInstall HookType = "post-install"
	PreRemove   HookType = "pre-remove"
	PostRemove  HookType = "post-remove"
)

// Hook represents a hook script with its type and content.
type Hook struct {
	Type    HookType
	Content string
}

// HookContext contains information passed to hooks.
type HookContext struct {
	PackageName    string
	PackageVersion string
	PackagePath    string
	InstallPath    string
	Vars           map[string]interface{}
}

// HookManager defines the interface for managing hooks.
type HookManager interface {
	// Execute runs the specified hook type with the given context
	Execute(hookType HookType, ctx HookContext) error

	// AddHook adds a new hook
	AddHook(hook Hook) error

	// RemoveHook removes a hook of the specified type
	RemoveHook(hookType HookType) error

	// HasHook checks if a hook of the specified type exists
	HasHook(hookType HookType) bool
}
