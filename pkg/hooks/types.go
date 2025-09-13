package hooks

// HookType represents the type of hooks.
type HookType string

// Supported hooks types.
const (
	PreInstall  HookType = "pre-install"
	PostInstall HookType = "post-install"
	PreRemove   HookType = "pre-remove"
	PostRemove  HookType = "post-remove"
)

// Hook represents a hooks script with its type and content.
type Hook struct {
	Type    HookType
	Content string
}

// HookContext contains information passed to hooks.
type HookContext struct {
	ArtifactName    string
	ArtifactVersion string
	ArtifactPath    string
	InstallPath     string
	Vars            map[string]interface{}
}

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
