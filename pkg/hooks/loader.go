package hooks

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cperrin88/gotya/pkg/errors"
)

// HookFileExtensions lists the supported hooks file extensions.
var HookFileExtensions = map[string]bool{
	".tengo": true,
	".go":    true, // For Go plugins in the future
}

// - <packageDir>/hooks/<hooks-type>.<ext>.
func LoadHooksFromArtifactDir(manager HookManager, packageDir string) error {
	// Try .gotya/hooks directory first
	hooksDir := filepath.Join(packageDir, ".gotya", "hooks")
	if _, err := os.Stat(hooksDir); err == nil {
		if err := loadHooksFromDir(manager, hooksDir); err != nil {
			return errors.Wrapf(err, "error loading hooks from %s", hooksDir)
		}
	}

	// Then try the hooks directory
	hooksDir = filepath.Join(packageDir, "hooks")
	if _, err := os.Stat(hooksDir); err == nil {
		if err := loadHooksFromDir(manager, hooksDir); err != nil {
			return errors.Wrapf(err, "error loading hooks from %s", hooksDir)
		}
	}

	return nil
}

// loadHooksFromDir loads all hooks files from a directory.
func loadHooksFromDir(manager HookManager, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errors.Wrapf(err, "failed to read hooks directory %s", dir)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if _, ok := HookFileExtensions[ext]; !ok {
			continue // Skip unsupported file types
		}

		hookName := strings.TrimSuffix(entry.Name(), ext)
		hookType := HookType(hookName)

		// Validate hooks type
		switch hookType {
		case PreInstall, PostInstall, PreRemove, PostRemove:
			// Valid hooks type
		default:
			continue // Skip unknown hooks types
		}

		// Read hooks content
		hookPath := filepath.Join(dir, entry.Name())
		content, err := os.ReadFile(hookPath)
		if err != nil {
			return errors.Wrapf(err, "error reading hooks file %s", hookPath)
		}

		// Add the hooks
		if err := manager.AddHook(Hook{
			Type:    hookType,
			Content: string(content),
		}); err != nil {
			return errors.Wrapf(err, "error adding hooks %s", hookName)
		}
	}

	return nil
}

// HookTemplate generates a template for a hooks script.
func HookTemplate(hookType HookType) string {
	switch hookType {
	case PreInstall:
		return `// Pre-install hooks
// This script runs before artifact installation
// Available variables:
// - packageName: string - name of the artifact being installed
// - packageVersion: string - version of the artifact
// - installPath: string - path where the artifact will be installed
// - packagePath: string - path to the artifact file
// - vars: map - custom variables passed to the hooks

// Example: Check if a required directory exists
/*
requiredDir := "/path/to/required/dir"
if !os.exists(requiredDir) {
    err := error("Required directory not found: " + requiredDir)
}
*/`

	case PostInstall:
		return `// Post-install hooks
// This script runs after artifact installation
// Available variables: same as pre-install hooks

// Example: Create a symlink after installation
/*
target := installPath + "/bin/myapp"
link := "/usr/local/bin/myapp"
if !os.exists(link) {
    os.symlink(target, link)
}
*/`

	case PreRemove:
		return `// Pre-remove hooks
// This script runs before artifact removal
// Available variables: same as pre-install hooks

// Example: Stop a service before removal
/*
service := "myservice"
os.exec("systemctl stop " + service)
*/`

	case PostRemove:
		return `// Post-remove hooks
// This script runs after artifact removal
// Available variables: same as pre-install hooks

// Example: Clean up temporary files
/*
tmpDir := "/tmp/mypkg/" + packageName
os.removeAll(tmpDir)
*/`

	default:
		return "// Unknown hooks type: " + string(hookType)
	}
}
