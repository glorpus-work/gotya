# gotya

[![Go Reference](https://pkg.go.dev/badge/github.com/cperrin88/gotya.svg)](https://pkg.go.dev/github.com/glorpus-work/gotya)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Go](https://github.com/glorpus-work/gotya/actions/workflows/go.yml/badge.svg)](https://github.com/glorpus-work/gotya/actions/workflows/go.yml)
[![codecov](https://codecov.io/github/glorpus-work/gotya/graph/badge.svg?token=PKR5SV44P3)](https://codecov.io/github/glorpus-work/gotya)

A lightweight personal artifact/package manager (think apt, but for personal use). It provides:
- CLI: sync indexes, search, install/uninstall, list, config, cache, version
- Library: reusable packages for index management, artifact handling, hooks, caching

## Requirements

- Go 1.24+
- Optional developer tools:
  - task (Taskfile runner) if you want to use Taskfile.yml scripts
  - golangci-lint for `task lint`

## Installation

From source (latest on your machine):

```bash
# Clone
git clone https://github.com/cperrin88/gotya.git
cd gotya

# Build CLI
go build -o bin/gotya ./cli/gotya

# (Optional) install into your GOPATH/bin
go install ./cli/gotya
```

From module proxy (HEAD version):

```bash
# Installs gotya binary into GOPATH/bin or GOBIN
go install github.com/cperrin88/gotya/cli/gotya@latest
```

## Usage

Common commands (run `gotya <cmd> --help` for full flags):

- `gotya sync` — synchronize repository indexes
- `gotya search <query>` — search packages
- `gotya list` — list installed packages
- `gotya install <package> [<package> ...]` — install one or more packages
- `gotya update` — update installed packages to latest versions
- `gotya uninstall <package> [<package> ...]` — uninstall packages
- `gotya cleanup` — remove orphaned packages and clean up
- `gotya cache clean|info|dir` — manage the cache
- `gotya config show|get|set|init` — view and modify configuration
- `gotya version` — print version info

## Configuration

### Configuration File Format

```yaml
# Repository configurations
repositories:
  - name: "main"
    url: "https://example.com/repo/index.json"
    enabled: true
    priority: 0

# General settings
settings:
  # Cache settings
  cache_dir: "~/.cache/gotya"
  cache_ttl: "24h0m0s"

  # State settings
  state_dir: "~/.local/share/gotya"

  # Installation settings
  install_dir: "~/.local/share/gotya/bin"
  meta_dir: "~/.local/share/gotya/meta"

  # Network settings
  http_timeout: "30s"
  max_concurrent_syncs: 5

  # Platform settings
  platform:
    os: "linux"        # Override target OS (auto-detected if empty)
    arch: "amd64"      # Override target architecture (auto-detected if empty)

  # Output settings
  output_format: "text"  # text, json, yaml
  log_level: "info"      # debug, info, warn, error
```

### Repository Configuration

- **name**: Unique identifier for the repository
- **url**: URL to the repository index file
- **enabled**: Whether the repository is active (default: true)
- **priority**: Repository priority for conflict resolution (lower numbers = higher priority)

### Settings Overview

- **Cache settings**: Control where and how long cached data is stored
- **State settings**: Directory for storing application state
- **Installation settings**: Directories for installed binaries and metadata
- **Network settings**: HTTP timeouts and concurrent operation limits
- **Platform settings**: Target OS/architecture and native package preferences
- **Output settings**: Output format and logging verbosity

### Configuration Commands

- `gotya config init` — Create a default configuration file
- `gotya config show` — Display current configuration
- `gotya config get <key>` — Get a specific configuration value
- `gotya config set <key> <value>` — Set a configuration value

The configuration file is automatically created with sensible defaults when gotya is first run.
