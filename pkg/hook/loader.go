package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HookFileExtensions lists the supported hook file extensions
var HookFileExtensions = map[string]bool{
	".tengo": true,
	".go":    true, // For Go plugins in the future
}

// LoadHooksFromPackageDir loads hooks from a package directory.
// It looks for hook files in the following locations:
// - <packageDir>/.gotya/hooks/<hook-type>.<ext>
// - <packageDir>/hooks/<hook-type>.<ext>
func LoadHooksFromPackageDir(manager HookManager, packageDir string) error {
	// Try .gotya/hooks directory first
	hooksDir := filepath.Join(packageDir, ".gotya", "hooks")
	if _, err := os.Stat(hooksDir); err == nil {
		if err := loadHooksFromDir(manager, hooksDir); err != nil {
			return fmt.Errorf("error loading hooks from .gotya/hooks: %w", err)
		}
	}

	// Then try the hooks directory
	hooksDir = filepath.Join(packageDir, "hooks")
	if _, err := os.Stat(hooksDir); err == nil {
		if err := loadHooksFromDir(manager, hooksDir); err != nil {
			return fmt.Errorf("error loading hooks from hooks/: %w", err)
		}
	}

	return nil
}

// loadHooksFromDir loads all hook files from a directory
func loadHooksFromDir(manager HookManager, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read hooks directory %s: %w", dir, err)
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

		// Validate hook type
		switch hookType {
		case PreInstall, PostInstall, PreRemove, PostRemove:
			// Valid hook type
		default:
			continue // Skip unknown hook types
		}

		// Read hook content
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return fmt.Errorf("error reading hook file %s: %w", entry.Name(), err)
		}

		// Add the hook
		if err := manager.AddHook(Hook{
			Type:    hookType,
			Content: string(content),
		}); err != nil {
			return fmt.Errorf("error adding hook %s: %w", hookName, err)
		}
	}

	return nil
}

// HookTemplate generates a template for a hook script
func HookTemplate(hookType HookType) string {
	switch hookType {
	case PreInstall:
		return `// Pre-install hook
// This script runs before package installation
// Available variables:
// - packageName: string - name of the package being installed
// - packageVersion: string - version of the package
// - installPath: string - path where the package will be installed
// - packagePath: string - path to the package file
// - vars: map - custom variables passed to the hook

// Example: Check if a required directory exists
/*
requiredDir := "/path/to/required/dir"
if !os.exists(requiredDir) {
    err := error("Required directory not found: " + requiredDir)
}
*/`

	case PostInstall:
		return `// Post-install hook
// This script runs after package installation
// Available variables: same as pre-install hook

// Example: Create a symlink after installation
/*
target := installPath + "/bin/myapp"
link := "/usr/local/bin/myapp"
if !os.exists(link) {
    os.symlink(target, link)
}
*/`

	case PreRemove:
		return `// Pre-remove hook
// This script runs before package removal
// Available variables: same as pre-install hook

// Example: Stop a service before removal
/*
service := "myservice"
os.exec("systemctl stop " + service)
*/`

	case PostRemove:
		return `// Post-remove hook
// This script runs after package removal
// Available variables: same as pre-install hook

// Example: Clean up temporary files
/*
tmpDir := "/tmp/mypkg/" + packageName
os.removeAll(tmpDir)
*/`

	default:
		return "// Unknown hook type: " + string(hookType)
	}
}
