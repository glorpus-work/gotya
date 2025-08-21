# Package Installer

The `installer` package provides functionality for installing, updating, and managing packages in the gotya package manager.

## Features

- **Package Installation**: Install packages with dependency resolution
- **Package Updates**: Update installed packages to their latest versions
- **Dependency Management**: Automatic handling of package dependencies
- **Transaction Support**: Atomic operations for package management
- **Hooks**: Support for pre- and post-installation hooks

## Usage

### Creating a New Installer

```go
import "github.com/cperrin88/gotya/pkg/installer"

// Create a new installer instance
cfg := &config.Config{...}  // Your configuration
repoManager := repository.NewManager()  // Repository manager
hookManager := hook.NewManager(cfg, repoManager)  // Hook manager

installer := installer.New(cfg, repoManager, hookManager)
```

### Installing a Package

```go
err := installer.InstallPackage("example-package", false, false)
if err != nil {
    log.Fatalf("Failed to install package: %v", err)
}
```

### Updating a Package

```go
updated, err := installer.UpdatePackage("example-package")
if err != nil {
    log.Fatalf("Failed to update package: %v", err)
}
if updated {
    log.Println("Package was updated")
} else {
    log.Println("Package is already up to date")
}
```

## Configuration

The installer can be configured using the following options:

- `Force`: Force installation/upgrade even if the package is already installed
- `SkipDeps`: Skip dependency resolution (not recommended)
- `DryRun`: Perform a dry run without making any changes

## Hooks

The installer supports the following hooks:

- `pre-install`: Runs before package installation
- `post-install`: Runs after successful package installation
- `pre-update`: Runs before package update
- `post-update`: Runs after successful package update

## Error Handling

All methods return errors that can be inspected for more details. Common errors include:

- `ErrPackageNotFound`: The requested package was not found
- `ErrDependencyConflict`: A dependency conflict was detected
- `ErrInstallationFailed`: The installation failed

## Testing

Run the tests with:

```bash
go test -v ./pkg/installer
```

## License

MIT
