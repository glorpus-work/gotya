# gotya

[![Go Reference](https://pkg.go.dev/badge/github.com/cperrin88/gotya.svg)](https://pkg.go.dev/github.com/cperrin88/gotya)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Go](https://github.com/cperrin88/gotya/actions/workflows/go.yml/badge.svg)](https://github.com/cperrin88/gotya/actions/workflows/go.yml)

A lightweight personal artifact/package manager (think apt, but for personal use). It provides:
- CLI: sync indexes, search, install/uninstall, list, config, cache, version
- Library: reusable packages for index management, artifact handling, hooks, caching

## Requirements

- Go 1.24+
- Optional developer tools:
  - task (Taskfile runner) if you want to use Taskfile.yml scripts
  - golangci-lint for `task lint`
  - act (optional) for running GitHub Actions locally via Taskfile

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
- `gotya list` — list available packages; `gotya list --installed` lists installed
- `gotya install <package> [<package> ...]` — install one or more packages
- `gotya uninstall <package> [<package> ...]` — uninstall packages
- `gotya cache [clean|info|dir]` — manage the cache
- `gotya config [show|get|set|init]` — view and modify configuration
- `gotya version` — print version info
