# Installer Artifact Design

## Overview
The `installer` package is responsible for managing the installation, update, and removal of packages in the Gotya package manager. It serves as the core component that coordinates between package repositories, file systems, and hook systems to ensure proper package management.

## Artifact Structure

```
pkg/installer/
├── installer.go         # Main implementation
└── installer_test.go    # Unit tests
```

## Core Components

### 1. Installer Struct

```go
type Installer struct {
    config      *config.Config
    repoManager repository.RepositoryManager
    hookManager hook.HookManager
}
```

- **config**: Application configuration including installation directories and settings
- **repoManager**: Handles package repository operations
- **hookManager**: Manages pre/post installation hooks

## Key Functions

### New(cfg *config.Config, repoManager repository.RepositoryManager, hookManager hook.HookManager) *Installer
- **Purpose**: Creates a new Installer instance
- **Parameters**:
  - `cfg`: Application configuration
  - `repoManager`: Repository manager instance
  - `hookManager`: Hook manager instance
- **Returns**: New Installer instance

### InstallArtifact(packageName string, force, skipDeps bool) error
- **Purpose**: Installs a package with the given name
- **Parameters**:
  - `packageName`: Name of the package to install
  - `force`: Force installation even if already installed
  - `skipDeps`: Skip dependency resolution if true
- **Returns**: Error if installation fails
- **Workflow**:
  1. Find package in repositories
  2. Check if already installed
  3. Resolve and install dependencies (if not skipped)
  4. Run pre-install hooks
  5. Install package files
  6. Update installed packages database
  7. Run post-install hooks

### UpdateArtifact(packageName string) (bool, error)
- **Purpose**: Updates a package to the latest version
- **Parameters**:
  - `packageName`: Name of the package to update
- **Returns**: (updated bool, error)
- **Workflow**:
  1. Find latest version in repositories
  2. Check if update is needed
  3. Run pre-update hooks
  4. Install new version
  5. Update installed packages database
  6. Run post-update hooks

### Platform-Specific Directory Structure

#### Linux/Unix
```
~/.cache/gotya/
└── packages/                # Downloaded package archives
    └── <package_name>_<version>.tar.gz

~/.local/share/gotya/
├── installed/              # Installed package files
│   └── <package_name>/     # Artifact installation directory
│       └── ...             # Files from package's 'files' directory
└── meta/                   # Artifact metadata
    └── <package_name>/     # Per-package metadata
        ├── manifest.json   # Artifact manifest
        └── checksums.sha256
```

#### macOS
```
~/Library/Caches/gotya/
└── packages/                # Downloaded package archives
    └── <package_name>_<version>.tar.gz

~/Library/Application Support/gotya/
├── installed/              # Installed package files
│   └── <package_name>/
└── meta/                   # Artifact metadata
    └── <package_name>/
```

#### Windows
```
%LOCALAPPDATA%\gotya\
├── cache\                 # Downloaded package archives
│   └── packages\
│       └── <package_name>_<version>.tar.gz
├── installed\             # Installed package files
│   └── <package_name>\
└── meta\                  # Artifact metadata
    └── <package_name>\
```

#### Environment Variables
- **Cache Directory**:
  - Linux/Unix: `$XDG_CACHE_HOME/gotya/` or `~/.cache/gotya/`
  - macOS: `$HOME/Library/Caches/gotya/`
  - Windows: `%LOCALAPPDATA%\gotya\cache\`

- **Data Directory**:
  - Linux/Unix: `$XDG_DATA_HOME/gotya/` or `~/.local/share/gotya/`
  - macOS: `$HOME/Library/Application Support/gotya/`
  - Windows: `%LOCALAPPDATA%\gotya\`

### installArtifactFiles(pkg *repository.Artifact) error
- **Purpose**: Internal method to handle the actual file installation
- **Parameters**:
  - `pkg`: Artifact to install
- **Returns**: Error if installation fails
- **Workflow**:
  1. **Create target directories**:
     - `<install_dir>/installed/<package_name>` - Main package directory
     - `<install_dir>/cache` - For downloaded archives (if not exists)
     - `<install_dir>/state` - For state files (if not exists)
  2. **Download package archive**:
     - Download URL: Retrieved from `pkg.URL`
     - Cache location: `<install_dir>/cache/<package_name>_<version>.tar.gz`
     - If file exists in cache and checksum matches, skip download
  3. **Verify checksum**:
     - Compare downloaded file's SHA256 with `pkg.Checksum`
     - Delete and retry download if checksum doesn't match (up to 3 times)
  4. **Extract files**:
     - Extract archive to temporary directory
     - Apply file permissions from package manifest
     - Move files to their final locations
     - Create necessary symlinks in standard locations (e.g., `/usr/local/bin`)
  5. **Update package metadata**:
     - Create/update `<install_dir>/installed/<package_name>/.gotya/manifest.json`
     - Generate and store file checksums in `checksums.sha256`
  6. **Handle file conflicts**:
     - Check for existing files before installation
     - If files exist and are identical, skip
     - If files differ, either:
       - Backup and replace (with `--force` flag)
       - Keep both versions (with `--keep` flag)
       - Abort installation (default)

### runHooks(event, packageName string, pkg *repository.Artifact) error
- **Purpose**: Executes hooks for a specific event
- **Parameters**:
  - `event`: Hook event type (e.g., "pre-install", "post-update")
  - `packageName`: Name of the package
  - `pkg`: Artifact details
- **Returns**: Error if hook execution fails

## Hook System

The installer supports the following hook events:

### Installation Hooks
- `pre-install`: Runs before package installation
  - **Context**: Artifact is about to be installed
  - **Use Case**: Check system requirements, validate environment
- `post-install`: Runs after successful installation
  - **Context**: Artifact files are in place
  - **Use Case**: Set up configurations, start services

### Update Hooks
- `pre-update`: Runs before package update
  - **Context**: New version is about to be installed
  - **Use Case**: Backup configurations, stop running services
- `post-update`: Runs after successful update
  - **Context**: New version is installed
  - **Use Case**: Migrate configurations, restart services

### Uninstallation Hooks
- `pre-uninstall`: Runs before package removal
  - **Context**: Artifact is about to be removed
  - **Use Case**: Stop services, backup user data
- `post-uninstall`: Runs after package removal
  - **Context**: Artifact files have been removed
  - **Use Case**: Clean up temporary files, remove user data (if confirmed)

## Error Handling

All errors are wrapped with context using `github.com/pkg/errors` to provide better error tracking and debugging information.

## File Management Details

### Download Process
1. **Temporary Storage**:
   - Downloads are saved to a temporary file in the system's temp directory
   - On successful download and verification, moved to cache directory
   - Temporary files are cleaned up on process exit

2. **Cache Management**:
   - Cache location follows platform conventions:
     - Linux/Unix: `~/.cache/gotya/packages/`
     - macOS: `~/Library/Caches/gotya/packages/`
     - Windows: `%LOCALAPPDATA%\gotya\cache\packages\`
   - Filename format: `<name>_<version>_<arch>_<checksum>.tar.gz`
   - Implements LRU (Least Recently Used) eviction policy
   - Maximum cache size is configurable (default: 1GB)
   - Cache can be cleared using `gotya cache clean`

3. **Installation Layout**:
   - **Artifact Files**:
     - Copied to platform-specific data directory under `installed/<package_name>/`
     - Original directory structure from package's `files/` directory is preserved
   - **Metadata**:
     - Stored in platform-specific data directory under `meta/<package_name>/`
     - Includes manifest, checksums, and installation logs
   - **Symlinks**:
     - Created in standard locations (e.g., `~/.local/bin/` on Linux) for executables
     - Managed by the package manager to avoid conflicts

4. **State Management**:
   - `installed.db`: SQLite database tracking installed packages
     - Artifact name, version, installation time, files
   - Lock files prevent concurrent modifications
     - File-based locks in `<install_dir>/state/locks/`
     - Automatically released on process exit

## Dependencies

- `github.com/cperrin88/gotya/pkg/config`: Configuration management
- `github.com/cperrin88/gotya/pkg/errors`: Error handling utilities
- `github.com/cperrin88/gotya/pkg/fsutil`: Filesystem utilities
- `github.com/cperrin88/gotya/pkg/hook`: Hook management
- `github.com/cperrin88/gotya/pkg/package`: Artifact management
- `github.com/cperrin88/gotya/pkg/repository`: Repository management

## Concurrency Considerations

The package is not currently designed for concurrent access. If multiple processes might install/update packages simultaneously, additional synchronization would be required.

## Future Improvements

1. **Rollback Mechanism**: Implement transaction-like behavior to rollback failed installations
2. **Concurrent Downloads**: Support parallel downloads of package files
3. **Resume Downloads**: Add support for resuming interrupted downloads
4. **Signature Verification**: Add package signature verification
5. **Delta Updates**: Support downloading only changed files for updates
6. **Atomic Installs**: Ensure atomic package installations to prevent partial installs

## Example Usage

```go
// Create required managers
config := config.New()
repoManager := repository.NewManager()
hookManager := hook.NewManager()

// Create orchestrator
installer := installer.New(config, repoManager, hookManager)

// Install a artifact
err := installer.InstallArtifact("example-artifact", false, false)
if err != nil {
    log.Fatalf("Failed to install artifact: %v", err)
}

// Update a artifact
updated, err := installer.UpdateArtifact("example-artifact")
if err != nil {
    log.Fatalf("Failed to update artifact: %v", err)
}
if updated {
    log.Println("Artifact was updated")
} else {
    log.Println("Artifact was already up to date")
}
```
