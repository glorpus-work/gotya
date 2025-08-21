# Hook System

This package provides an extensible hook system for executing scripts at various stages of package management operations using the Tengo scripting language.

## Features

- Support for pre/post install/remove hooks
- Extensible architecture for adding new hook types
- Thread-safe execution
- Simple API for integration
- Support for Tengo scripting language
- Automatic hook discovery from package directories

## Hook Types

- `pre-install`: Runs before package installation
- `post-install`: Runs after package installation
- `pre-remove`: Runs before package removal
- `post-remove`: Runs after package removal

## Usage

### Basic Usage

```go
import "your-module-path/pkg/hook"

// Create a new hook manager
manager := hook.NewHookManager()

// Add a hook
err := manager.AddHook(hook.Hook{
    Type: hook.PreInstall,
    Content: `
        // This is a pre-install hook
        fmt.println("Installing package:", packageName, "version:", packageVersion)
    `,
})
if err != nil {
    // Handle error
}

// Execute a hook
err = manager.Execute(hook.PreInstall, hook.HookContext{
    PackageName:    "example-package",
    PackageVersion: "1.0.0",
    PackagePath:    "/path/to/package.tar.gz",
    InstallPath:    "/usr/local/example-package",
    Vars: map[string]interface{}{
        "customVar": "custom value",
    },
})

// Check if a hook exists
if manager.HasHook(hook.PostInstall) {
    // Execute post-install hook
}
```

### Loading Hooks from Package Directory

```go
// Load hooks from a package directory
// Looks for hooks in:
// - <packageDir>/.gotya/hooks/<hook-type>.tengo
// - <packageDir>/hooks/<hook-type>.tengo
err := hook.LoadHooksFromPackageDir(manager, "/path/to/package")
if err != nil {
    // Handle error
}
```

## Hook Script Examples

### Pre-install Hook

```tengo
// pre-install.tengo
fmt.println("Running pre-install for:", packageName, "version:", packageVersion)

// Check if a required directory exists
requiredDir := "/path/to/required/dir"
if !os.exists(requiredDir) {
    err := error("Required directory not found: " + requiredDir)
}

// Access custom variables
if vars != undefined {
    customValue := vars.customVar
    fmt.println("Custom variable value:", customValue)
}
```

### Post-install Hook

```tengo
// post-install.tengo
fmt.println("Running post-install for:", packageName)

// Create a symlink
symlink := "/usr/local/bin/myapp"
target := installPath + "/bin/myapp"
if !os.exists(symlink) {
    os.symlink(target, symlink)
}
```

## Available Variables in Hooks

- `packageName`: Name of the package
- `packageVersion`: Version of the package
- `packagePath`: Path to the package file
- `installPath`: Path where the package is being installed
- `vars`: Map of custom variables passed to the hook

## Adding New Hook Types

1. Add the new hook type to the `HookType` constants in `types.go`
2. Update the `HookTemplate` function in `loader.go` to include a template for the new hook type
3. The hook system will automatically support the new hook type

## Thread Safety

The hook manager is safe for concurrent use from multiple goroutines.
