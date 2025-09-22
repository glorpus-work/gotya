# Gotya Design Document

Last updated: 2025-09-22

This document captures the overall architecture and inner workings of the gotya project. It is intended for maintainers and contributors who need to understand how the system fits together and how to extend it safely.


1. Purpose and scope
- gotya is a lightweight, personal artifact/package manager. Think of it as a minimal apt-like tool for your own artifacts: it synchronizes indexes, searches, downloads, verifies, and installs versioned artifacts built for specific platforms.
- Two main entry points:
  - CLI application: cli/gotya
  - Library packages: pkg/* (index, artifact, hooks, cache, config, http, platform, fsutil)


2. High-level architecture
- CLI (internal/cli, cli/gotya)
  - Cobra commands: sync, search, install, uninstall, cache, config, version (list and update are planned/partially scaffolded)
  - Uses helpers to load Config, HTTP client, Index manager, Artifact manager
- Core domains
  - pkg/index: repository model, index format, sync and resolution
  - pkg/artifact: artifact model, packer (to build .gotya archives), installer-facing manager, verification
  - pkg/http: thin HTTP client used by index and artifact managers
  - pkg/cache: cache management utilities (index and artifact caches)
  - pkg/config: configuration model, defaults, serialization (YAML)
  - pkg/hooks: Tengo-based hook execution engine (pre/post install/remove)
  - pkg/platform: OS/Arch normalization and matching
  - pkg/fsutil: directories, file modes, and path helpers
  - internal/logger: simple leveled logger used by CLI

Data flow overview (install path):
1) CLI parses user input and loads config
2) Index manager resolves an artifact (name + version range + platform)
3) HTTP client downloads the artifact into cache
4) Artifact manager verifies metadata and file checksums
5) Future work: extract/install, run hooks, record installed DB


3. Configuration (pkg/config)
- File format: YAML
- Suggested default path: GetDefaultConfigPath() uses platform-aware user config dirs, typically $XDG_CONFIG_HOME/gotya/config.yaml on Linux.
- Core structs:
  - Config
    - Repositories []RepositoryConfig
    - Platform PlatformConfig
    - Settings Settings
  - RepositoryConfig { Name, URL, Enabled, Priority }
  - PlatformConfig { OS, Arch } with auto-detection if empty
  - Settings captures cache dir, index dir, install dir, database path, HTTP timeout, etc. Defaults are applied in applyDefaults().
- Helpers: LoadConfig(path), SaveConfig(path), ToYAML().
- Derived paths:
  - Index dir: Config.GetIndexDir()
  - Artifact cache dir: Config.GetArtifactCacheDir()
  - Installed DB path: Config.GetDatabasePath()

Example (minimal) YAML:

```yaml
repositories:
  - name: main
    url: http://localhost:52221
    enabled: true
    priority: 0
platform:
  os: ""   # auto-detect
  arch: "" # auto-detect
settings:
  http_timeout: 30s
```


4. Repository and index management (pkg/index)
- Repository
  - Name, URL (*url.URL), Priority (uint), Enabled (bool)
- Index format (JSON)
  - Index { format_version:string, last_update: RFC3339 time, packages: []Artifact }
  - Artifact fields are defined in pkg/index/artifact.go (Name, Version, Description, URL, OS, Arch, etc.)
- Manager (ManagerImpl in pkg/index/manager.go)
  - NewManager(httpClient, repositories, indexPath, cacheTTL)
  - Sync(ctx, name) / SyncAll(ctx)
    - Downloads index.json with pkg/http client into cache (indexPath/<repo>/index.json)
  - IsCacheStale(name) and GetCacheAge(name)
  - FindArtifacts(name) across repositories (returns map[repo][]*Artifact)
  - ResolveArtifact(name, version, os, arch) chooses the best artifact matching version constraint and platform, taking repository priority into account
  - GetIndex(name) returns parsed Index
  - ListRepositories() for introspection
- Index helpers (pkg/index/index.go)
  - NewIndex(formatVersion)
  - ParseIndex, ParseIndexFromFile/Reader
  - ToJSON (pretty-printed)
  - AddArtifact, RemoveArtifact, FindArtifacts

Index on disk
- Each repository’s index is stored under a configurable index path, e.g. <cache>/indexes/<repo>/index.json. ManagerImpl.getIndexPath builds this path.


5. HTTP client (pkg/http)
- ClientImpl
  - NewHTTPClient(timeout)
  - DownloadIndex(ctx, repoURL, filePath)
    - Builds URL repoURL + "/index.json"
    - GET with User-Agent and Accept headers; writes to file and preserves Last-Modified when provided
  - DownloadArtifact(ctx, packageURL, filePath)
- Errors are wrapped via pkg/errors.
- Retries/backoff/rate-limiting are not yet implemented; TODO items can extend this.


6. Artifact domain (pkg/artifact)
- Metadata (metadata.go)
  - Name, Version, OS, Arch, Maintainer, Description, Dependencies, Hashes (map[path]sha256), Hooks (map[type]script)
  - Helpers: GetVersion() (hashicorp/go-version), GetOS()/GetArch() with platform.Any* fallback
- Packer (packer.go)
  - NewPacker(...).Pack() builds a .gotya archive from an input directory:
    1) Verifies inputs and platform fields
    2) Copies input into a staging area with the required structure
    3) Generates metadata.json with file hashes and optional hooks
    4) Creates the final archive at outputDir/<name>_<version>_<os>_<arch>.gotya
  - Uses github.com/mholt/archives to write archives
- ManagerImpl (manager.go)
  - NewManager(indexManager, httpClient, os, arch, artifactCacheDir)
  - InstallArtifact(ctx, pkgName, version, force)
    - Resolve artifact via index manager
    - DownloadArtifact(ctx)
    - VerifyArtifact(ctx)
    - Note: installation/extraction into install dir is a future step; current code verifies integrity only
  - DownloadArtifact(ctx, artifact)
  - VerifyArtifact(ctx, artifact)
    - Opens the archive as a virtual filesystem
    - Reads metadata file and validates fields against index artifact (name/version/os/arch)
    - Computes sha256 for files under the data directory and compares to recorded hashes
  - getArtifactCacheFilePath: <artifactCacheDir>/<name>_<version>_<os>_<arch>.gotya

Archive structure (logical)
- data/: payload files to be installed
- meta/metadata.json: artifact metadata and checksums
- hooks/: optional Tengo scripts (pre/post-install/remove) referenced in metadata


7. Hooks (pkg/hooks)
- Purpose: allow running user-provided Tengo scripts at lifecycle points
- Types (hooks/types.go): PreInstall, PostInstall, PreRemove, PostRemove
- DefaultHookManager executes scripts via TengoExecutor (tengo v2). The current loader is a stub; future work will scan hooks/ in archives or installed artifacts.
- Execution: manager.Execute(type, HookContext) runs the registered script with a context map.


8. Cache (pkg/cache)
- CacheManager manages directories under a configured cache root (defaults to ~/.cache/gotya):
  - indexes/: repository indexes
  - packages/: downloaded .gotya files
- Operations:
  - Clean(CleanOptions) removes indexes and/or packages and returns sizes freed
  - GetInfo() returns sizes and file counts
  - GetDirectory()/SetDirectory()


9. Platform handling (pkg/platform)
- NormalizeOS/NormalizeArch bring platform strings to canonical form (linux, darwin, windows, amd64, arm64, etc.)
- CurrentPlatform() inspects runtime.GOOS/GOARCH
- IsCompatible(targetOS, targetArch) honors "any" wildcard
- Platform.Matches supports bidirectional wildcard matching


10. Filesystem utilities (pkg/fsutil)
- Directory helpers (paths, directories) plus secure default file/dir modes
- Used by http, cache, and packer to create directories and files safely


11. CLI layout (internal/cli, cli/gotya)
- Cobra root in cli/gotya/gotya.go wires subcommands
- Commands under internal/cli:
  - sync: refresh indexes across repositories
  - search: search by name across indexes
  - install: install one or more artifacts (currently downloads + verifies)
  - uninstall: placeholder for removing installed artifacts
  - cache: info/clean utilities
  - config: show/get/set/init helpers for YAML config
  - version: prints version
- Helpers (internal/cli/*): constructing HTTP client, index and artifact managers from config
- Integration tests in cli/gotya/gotya_integration_test.go spin a local HTTP server with test/repo


12. Data and state on disk
- Config: $XDG_CONFIG_HOME/gotya/config.yaml (or OS equivalent). See pkg/config.GetDefaultConfigPath.
- Cache: defaults to ~/.cache/gotya with subdirs indexes/ and packages/
- Installed DB: JSON written to a platform-appropriate data dir; model type is artifact.InstalledArtifact and an InstalledManager interface exists for future use.
- Test repository: test/repo/index.json and payload files served by the integration test HTTP server.


13. Error handling and logging
- Errors use pkg/errors which wraps underlying errors with context and supports sentinels (e.g., ErrArtifactNotFound, ErrArtifactInvalid, cache errors).
- internal/logger provides leveled logging used by CLI; verbosity is controlled by flags.


14. Versioning and resolution
- Version constraints use hashicorp/go-version. Artifact.Metadata.GetVersion provides parsed versions; index.Manager.ResolveArtifact accounts for name, version constraint string (e.g., ">= 0.0.0"), and platform OS/Arch when choosing candidates across repositories. Repository priority determines tie-breaking.


15. Extensibility and TODOs
- Installer: add extraction to install directory, write installed DB, and run hooks around install/remove operations.
- Hooks loader: implement loading scripts from archives and bind safe stdlib into Tengo runtime.
- HTTP: add retries, ETag/If-Modified-Since caching, and rate limiting.
- CLI: complete list/update commands and improve UX.
- Env var overrides: optionally honor GOTYA_CONFIG_DIR, GOTYA_CACHE_DIR, GOTYA_INSTALL_DIR at runtime.
- Security: verify artifact signatures (future) and tighten archive extraction.


16. Reference: key types and files
- pkg/index
  - types: Index, Artifact, Repository
  - manager: ManagerImpl (Sync, ResolveArtifact, FindArtifacts)
- pkg/artifact
  - metadata: Metadata
  - packer: Packer
  - manager: ManagerImpl (download/verify)
- pkg/http: ClientImpl (DownloadIndex/DownloadArtifact)
- pkg/cache: CacheManager
- pkg/config: Config, RepositoryConfig, PlatformConfig, Settings
- pkg/hooks: DefaultHookManager, TengoExecutor, types
- pkg/platform: Platform helpers and constants
- pkg/fsutil: directory and path helpers
- internal/cli: cobra commands and helpers
- cli/gotya: main entrypoint and integration tests


17. Minimal examples
Index JSON (test/repo/index.json excerpt):
```json
{
  "format_version": "1",
  "last_update": "2024-01-01T00:00:00Z",
  "packages": [
    {
      "name": "hello",
      "version": "1.0.0",
      "description": "hello world artifact",
      "url": "http://localhost:52221/packages/hello_1.0.0_linux_amd64.gotya",
      "os": "linux",
      "arch": "amd64"
    }
  ]
}
```

Config YAML (concise):
```yaml
repositories:
  - { name: main, url: http://localhost:52221, enabled: true, priority: 0 }
platform: { os: "", arch: "" }
settings: { http_timeout: 30s }
```


18. Development and tests
- go test ./... runs unit tests.
- Integration tests: cli/gotya/gotya_integration_test.go starts a local server (see test/testutil/server.go) that serves test/repo.
- Docker compose also provides a static server for manual testing: docker compose up -d test-repo.


19. Glossary
- Artifact: a versioned, platform-targeted bundle (.gotya archive) containing data/, meta/metadata.json, and optional hooks/.
- Index: a JSON document listing artifacts for a repository.
- Repository: a remote or local endpoint serving index.json and artifacts.
- Cache: local storage for downloaded indexes and artifacts.
- Hook: Tengo script executed around install/remove events.

