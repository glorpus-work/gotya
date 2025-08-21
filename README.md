# gotya

[![Go Reference](https://pkg.go.dev/badge/github.com/cperrin88/gotya.svg)](https://pkg.go.dev/github.com/cperrin88/gotya)

gotya is a lightweight personal package manager (like apt) with:
- **CLI**: Install, update, and manage packages
- **Library**: Programmatic package management
- **Extensible**: Plugin system for custom repositories and hooks

## Installation

```bash
# Install the latest version
go install github.com/cperrin88/gotya@latest
```

## Features

- **Package Management**: Install, update, and remove packages
- **Repository Support**: Multiple repository sources with priority support
- **Dependency Resolution**: Automatic handling of package dependencies
- **Hooks**: Pre- and post-installation hooks for custom actions
- **Cache Management**: Efficient package and index caching

## Architecture

gotya is organized into several packages, each with a specific responsibility:

### Core Packages

- `pkg/config`: Configuration management with support for multiple backends
- `pkg/cache`: Package and index caching system
- `pkg/repository`: Repository management and package resolution
- `pkg/installer`: Package installation and update logic
- `pkg/hook`: Hooks system for custom actions

### CLI Commands

- `gotya install <package>`: Install a package
- `gotya update [package]`: Update packages
- `gotya repo add <url>`: Add a package repository
- `gotya cache clean`: Clean the package cache
- `gotya config`: Manage configuration

## Development

### Requirements

- Go 1.24+

### Building from Source

```bash
git clone https://github.com/cperrin88/gotya.git
cd gotya
go build -o gotya ./cmd/gotya
```

### Running Tests

```bash
go test ./...
```

## License

MIT
