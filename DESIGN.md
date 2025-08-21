# Gotya Package Manager - Design Document

## Overview
Gotya is a lightweight, cross-platform package manager designed for managing software packages in a simple and efficient manner. It supports multiple repositories, platform-specific packages, and efficient dependency resolution.

## Core Components

### Repository System

#### Repository Management
- **Repository**: A collection of packages with metadata
- **Index**: Contains package metadata and download information
- **Sync**: Handles repository synchronization with support for caching and conditional requests

#### Repository Structure
```
repositories/
  <repo-name>/
    index.json       # Package index
    packages/        # Cached package files
```

### Index Management

#### Index Structure
```go
type IndexImpl struct {
    FormatVersion string    `json:"format_version"`
    LastUpdate    time.Time `json:"last_update"`
    Packages      []Package `json:"packages"`
}
```

#### Index Package Structure
```go
type Package struct {
    Name         string            `json:"name"`
    Version      string            `json:"version"`
    Description  string            `json:"description"`
    URL          string            `json:"url"`
    Checksum     string            `json:"checksum"`
    Size         int64             `json:"size"`
    OS           string            `json:"os,omitempty"`
    Arch         string            `json:"arch,omitempty"`
    Dependencies []string          `json:"dependencies,omitempty"`
    Metadata     map[string]string `json:"metadata,omitempty"`
}
```

## Package Management



### Configuration

#### Configuration Structure
```yaml
repositories:
  - name: main
    url: https://example.com/repo
    enabled: true
    priority: 0

http:
  timeout: 30s
  max_retries: 3
  max_redirects: 5

platform:
  os: ""        # Auto-detect if empty
  arch: ""      # Auto-detect if empty
  prefer_native: true

cache:
  directory: ~/.cache/gotya
  ttl: 1h

logging:
  level: info
  format: text
  output: stderr
```

### HTTP Client

#### Features
- Conditional GET requests with `If-Modified-Since`
- Timeout and retry mechanisms
- Concurrent downloads with rate limiting
- Support for HTTP/HTTPS with proper redirect handling

#### Caching
- Local file system cache
- ETag and Last-Modified header support
- Configurable TTL for cached items

### Platform Support

#### Platform Detection
- Auto-detection of OS and architecture
- Support for platform-specific packages
- Cross-platform package resolution

#### Platform-Specific Packages

## Package Format

Gotya packages are distributed as `.tar.gz` archives with a specific internal structure. This section defines the package format and structure.

### Package Structure

```
<package-name>_<version>_<os>_<arch>/
├── meta/
│   ├── package.json    # Package metadata
│   ├── pre-install     # Optional pre-installation script
│   └── post-install    # Optional post-installation script
└── files/              # Package contents
    └── ...             # Files to be installed
```

### Package Metadata (package.json)

```json
{
  "name": "string",           // Package name (required)
  "version": "string",        // Package version (required)
  "maintainer": "string",     // Package maintainer (optional)
  "description": "string",    // Package description (required)
  "dependencies": [           // List of package dependencies (optional)
    "package1>=1.0.0",
    "package2<2.0.0"
  ],
  "files": [                  // List of files to install (auto-generated)
    {
      "path": "string",       // Relative path of the file
      "size": 0,              // File size in bytes
      "mode": 0,              // File mode/permissions
      "digest": "string"      // SHA256 checksum of the file
    }
  ],
  "hooks": {                  // Hook scripts (optional)
    "pre_install": "string",  // Path to pre-install script (relative to meta/)
    "post_install": "string"  // Path to post-install script (relative to meta/)
  }
}
```

### Installation Directories

Packages are installed in the following locations by default:

- **Linux/Unix**: `~/.local/share/gotya/packages/`
- **macOS**: `~/Library/Application Support/gotya/packages/`
- **Windows**: `%APPDATA%\gotya\packages\`

### Hook Scripts

- **Pre-install**: Executed before files are extracted
- **Post-install**: Executed after files are extracted

## Package Creation Tool

The `gotya package create` command is used to create packages from a source directory.

### Package Source Directory Structure

```
package-src/
├── meta/
│   ├── package.json    # Package metadata
│   ├── pre-install     # Optional pre-installation script
│   └── post-install    # Optional post-installation script
└── files/              # Files to include in the package
    └── ...             # Package contents
```

### Creating a Package

```bash
# Basic usage
gotya package create --source ./package-src --output ./output-dir

# With version and platform overrides
gotya package create --source ./package-src --version 1.0.0 --os linux --arch amd64
```

### Package Creation Process

1. Validate package metadata
2. Calculate checksums for all files
3. Generate final package metadata
4. Create tarball with proper directory structure
5. Compress with bzip2
6. Generate package checksum

### Package Installation

```bash
# Install a package
gotya install <package-file>.tar.gz

# Install with custom installation directory
gotya install --prefix ~/my-packages <package-file>.tar.gz
```
- Packages can specify target OS/architecture
- Fallback to platform-agnostic packages
- Support for platform-specific dependencies

### CLI Interface

#### Core Commands
- `install`: Install packages
- `remove`: Remove packages
- `update`: Update package index
- `upgrade`: Upgrade installed packages
- `search`: Search for packages
- `list`: List installed packages
- `info`: Show package information
- `config`: Manage configuration
  - `repo`: Add/Remove/Update repository

#### Advanced Commands
- `package`: Create packages
- `repository`: Create and update remote repositories

### Installation Process

1. Resolve package dependencies
2. Download package files with checksum verification
3. Extract and install files to the appropriate locations
4. Update package database
5. Run post-install scripts (if any)

#### Install locations
- Installed into the system directory for user installed software
  - Linux: $HOME/.local/share
  - macOS: $HOME/Library/Application Support
  - Windows: $LOCALAPPDATA

### Security

#### Package Verification
- Checksum verification for downloaded packages
- Optional package signing
- Repository signature verification

## Future Enhancements

### Short-term
- [ ] Support for package signing and verification
- [ ] Parallel package downloads
- [ ] Transaction support for atomic operations
- [ ] Plugin system for custom package sources

### Long-term
- [ ] Distributed package caching
- [ ] Support for virtual environments
- [ ] Integration with system package managers
- [ ] GUI interface

## Dependencies

### Core Dependencies
- Go standard library
- [spf13/cobra](https://github.com/spf13/cobra) - CLI framework
- [stretchr/testify](https://github.com/stretchr/testify) - Testing utilities

## Development

### Building
```bash
go build -o gotya ./cmd/gotya
```

### Testing
```bash
go test ./...
```

### Code Style
- Follow standard Go code review comments
- 100% test coverage for critical paths
- Document all exported functions and types

## License
[Specify License]

## Contributing
[Contribution guidelines]

## Maintainers
[Maintainer information]
