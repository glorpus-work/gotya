# gotya

[![Go Reference](https://pkg.go.dev/badge/github.com/cperrin88/gotya.svg)](https://pkg.go.dev/github.com/cperrin88/gotya)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)

A lightweight personal artifact/package manager (think apt, but for personal use). It provides:
- CLI: sync indexes, search, install/uninstall, list, config, cache, version
- Library: reusable packages for index management, artifact handling, hooks, caching

## Overview

- Entry point (CLI): `cli/gotya/gotya.go` (uses spf13/cobra)
- Core domains:
  - `pkg/index`: repository indexes and syncing
  - `pkg/artifact`: artifact metadata, packing, install state, manager
  - `pkg/hooks`: hooks execution (Tengo-based), loader and manager
  - `pkg/cache`: cache manager and operations for indexes/packages
  - `pkg/config`: configuration model, defaults, load/save (YAML)
  - `internal/cli`: cobra commands wiring and helpers
  - `internal/logger`: simple leveled logger

## Stack

- Language: Go (modules)
- Go version: 1.24 (see go.mod)
- CLI framework: github.com/spf13/cobra
- Config format: YAML (gopkg.in/yaml.v3)
- Scripting/DSL for hooks: github.com/d5/tengo/v2
- Testing: go test, github.com/stretchr/testify
- Package manager: Go modules (go, go mod)

## Requirements

- Go 1.24+
- Optional developer tools:
  - task (Taskfile runner) if you want to use Taskfile.yml scripts
  - golangci-lint for `task lint`
  - docker and docker-compose for the local test repo service
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

NOTE: The previous README referenced ./cmd/gotya which does not exist. The correct path is ./cli/gotya.

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

Global flags as wired in CLI:
- `--config <path>` — config file path (default: auto-detect)
- `-v, --verbose` — verbose output (log level debug)
- `--no-color` — disable colored output
- `-o, --output <json|yaml|table>` — output format (default table)

## Configuration

- Default config path: platform-specific user config dir, e.g. `${XDG_CONFIG_HOME:-~/.config}/gotya/config.yaml` (see `pkg/config.GetDefaultConfigPath`).
- Config format: YAML. See `pkg/config.Config` for available fields.
- Key paths derived from config/defaults:
  - Cache dir: detected via `pkg/fsutil.GetCacheDir()`; indexes in `<cache>/indexes`, packages in `<cache>/packages`.
  - Install dir: detected via `pkg/fsutil.GetInstalledDir()`.
  - Installed DB: typically `~/.local/share/gotya/state/gotya/state/installed.json` on Linux (see notes below).

Notes:
- The state path is computed in `pkg/config.getUserDataDir()` using XDG-like rules; review code for exact platform behavior.
- Repositories can be configured in the YAML under `repositories: [{name, url, enabled, priority}]`.
- You can initialize or inspect config via `gotya config init|show|get|set`.

## Environment variables

Currently observed in code/tests:
- `XDG_DATA_HOME` — affects user data directory resolution (used by config/state path logic).
- `NO_COLOR=true` — disables colored output in tests; equivalent to `--no-color` flag for CLI.

TODO:
- Document and implement support for `GOTYA_CONFIG_DIR`, `GOTYA_CACHE_DIR`, `GOTYA_INSTALL_DIR` as runtime overrides if intended. They are used in tests but not currently read by the application code.
- Document any additional env vars once implemented.

## Scripts (Taskfile.yml)

Task aliases are provided for convenience (requires `task`):

- Build/install:
  - `task build` — go build -o ./bin/gotya ./cmd/gotya  [NOTE: path appears stale]
  - `task install` — go install ./cmd/gotya             [NOTE: path appears stale]
- Tests:
  - `task test` — run unit tests (short)
  - `task test-integration` — run integration tests with `-tags=integration`
  - `task test-all` — both unit and integration tests
  - `task test-cover` / `task test-cover-html`
  - `task mutate` / `task mutate-verbose` — mutation testing (go-mutesting)
- Lint/format/modules: `task lint`, `task fmt`, `task tidy`
- Cleanup: `task clean`
- CI helpers (local): `task ci`, and various `act-*` tasks

IMPORTANT: The build/install tasks reference `./cmd/gotya` which is not present in this repo snapshot. The correct module path for the CLI is `./cli/gotya`. TODO: Update Taskfile.yml to use `./cli/gotya`.

## Running a local test repo

An nginx-based static server is defined for serving the test repository:

```bash
docker compose up -d test-repo
# serves ./test/repo on http://localhost:52221
```

This is useful for manual `gotya sync` and `gotya search` experiments against the provided sample repo in `test/repo`.

## Tests

- Unit tests: `go test ./...`
- Integration tests: see `cli/gotya/gotya_integration_test.go`. They start a local HTTP server from `test/testutil/server.go` that serves `test/repo/` on port 52221. You can also run `docker compose up -d test-repo` to replicate the environment manually.

Some tests use the following environment variables solely within the test harness: `GOTYA_CONFIG_DIR`, `GOTYA_CACHE_DIR`, `GOTYA_INSTALL_DIR`, `NO_COLOR`. These are set by tests to isolate state; they are not currently honored by the application runtime unless implemented.

## Project structure

- `cli/gotya/` — CLI entrypoint (main) and integration tests
- `internal/cli/` — cobra commands: sync, install, uninstall, search, list, config, cache, version
- `internal/logger/` — logger and tests
- `pkg/artifact/` — artifact types, manager, packer, installed DB
- `pkg/cache/` — cache manager and operations
- `pkg/config/` — configuration, defaults, helpers
- `pkg/hooks/` — hook interfaces, Tengo executor, loader/manager
- `pkg/http/` — simple HTTP client interface/impl
- `pkg/index/` — index types, repository, manager
- `pkg/platform/` — platform constants and validation
- `pkg/fsutil/` — directories and paths helpers
- `docs/` — design documents
- `test/` — fixtures and test utilities, sample repo under test/repo

## License

This project is licensed under the GNU General Public License v3.0 — see the [LICENSE](LICENSE) file for details.
