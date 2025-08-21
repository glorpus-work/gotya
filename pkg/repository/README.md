# Repository Package

The `repository` package provides functionality for managing package repositories, including adding, removing, and updating repositories, as well as resolving package dependencies.

## Features

- **Repository Management**: Add, remove, enable, and disable repositories
- **Package Resolution**: Find and resolve packages across multiple repositories
- **Caching**: Built-in caching of repository indexes
- **Priority System**: Control which repositories take precedence
- **Concurrent Operations**: Safe for concurrent use

## Usage

### Creating a New Repository Manager

```go
import "github.com/cperrin88/gotya/pkg/repository"

// Create a new repository manager
manager := repository.NewManager()

// Or with a custom cache directory
manager := repository.NewManagerWithCacheDir("/path/to/cache/dir")
```

### Basic Operations

```go
// Add a repository
err := manager.AddRepository("my-repo", "https://example.com/repo")

// List all repositories
repos := manager.ListRepositories()

// Get a specific repository
repo := manager.GetRepository("my-repo")

// Remove a repository
err = manager.RemoveRepository("my-repo")
```

### Repository Operations

The `RepositoryOperation` provides a higher-level API for common repository operations:

```go
repoOp := repository.NewRepositoryOperation(manager)

// Add a repository with auto-generated name
err := repoOp.Add("", "https://example.com/repo", 0) // Auto-generate name, default priority

// List repositories in a formatted way
output, err := repoOp.List()
fmt.Println(output)

// Update repositories
output, err = repoOp.Update([]string{"my-repo"}) // Update specific repositories
// or
output, err = repoOp.Update(nil) // Update all enabled repositories
```

### Syncing Repositories

```go
// Sync a specific repository
err := manager.SyncRepository(context.Background(), "my-repo")

// Sync all enabled repositories
err = manager.SyncRepositories(context.Background())

// Check if a repository's cache is stale
if manager.IsCacheStale("my-repo", 1*time.Hour) {
    // Cache is older than 1 hour
}
```

## Repository Configuration

Each repository has the following configuration:

- **Name**: Unique identifier for the repository
- **URL**: Base URL of the repository
- **Enabled**: Whether the repository is enabled
- **Priority**: Priority for package resolution (higher numbers take precedence)

## Error Handling

All methods return errors that can be inspected for more details. Common errors include:

- `ErrRepositoryExists`: A repository with the same name already exists
- `ErrRepositoryNotFound`: The specified repository was not found
- `ErrRepositorySyncFailed`: Failed to sync the repository
- `ErrInvalidRepositoryURL`: The repository URL is invalid

## Testing

Run the tests with:

```bash
go test -v ./pkg/repository
```

## License

MIT
