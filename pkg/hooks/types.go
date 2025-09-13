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
