# Cache Package

The `cache` package provides a flexible and efficient caching system for gotya, handling both package and repository index caching.

## Features

- **Dual Cache Types**: Separate caches for package indexes and downloaded packages
- **Automatic Cleanup**: Configurable cleanup of stale cache entries
- **Size Tracking**: Track cache usage and file counts
- **Thread-Safe**: Safe for concurrent use

## Usage

### Creating a New Cache Manager

```go
import "github.com/cperrin88/gotya/pkg/cache"

// Create a new cache manager with default settings
cacheManager := cache.NewManager("/path/to/cache/directory")

// Or with custom options
cacheManager := cache.NewManagerWithOptions(cache.Options{
    BaseDir: "/custom/cache/dir",
    IndexTTL:  24 * time.Hour,  // How long to keep index files
    PackageTTL: 7 * 24 * time.Hour,  // How long to keep downloaded packages
})
```

### Basic Operations

```go
// Clean the cache
result, err := cacheManager.Clean(cache.CleanOptions{
    All:      true,  // Clean everything
    Indexes:  true,  // Clean only indexes
    Packages: false, // Don't clean packages
})

// Get cache information
info, err := cacheManager.GetInfo()
fmt.Printf("Cache size: %d bytes\n", info.TotalSize)
```

### Cache Operations

The `CacheOperation` provides a higher-level API for common cache operations:

```go
cacheOp := cache.NewCacheOperation(cacheManager)

// Clean cache with human-readable output
result, err := cacheOp.Clean(true, false, false) // Clean all
if err != nil {
    log.Fatal(err)
}
fmt.Println(result) // Human-readable result

// Get formatted cache info
info, err := cacheOp.GetInfo()
if err != nil {
    log.Fatal(err)
}
fmt.Println(info) // Formatted cache information
```

## Cache Structure

The cache directory has the following structure:

```
cache/
├── indexes/       # Repository index files
│   └── <repo-name>.json
└── packages/      # Downloaded packages
    └── <package-name>_<version>.pkg
```

## Configuration Options

- **BaseDir**: Base directory for cache storage
- **IndexTTL**: Time-to-live for index files (default: 1 hour)
- **PackageTTL**: Time-to-live for downloaded packages (default: 7 days)
- **MaxSize**: Maximum total cache size in bytes (0 for unlimited)

## Error Handling

All methods return errors that can be inspected for more details. Common errors include:

- `ErrCacheNotInitialized`: The cache manager was not properly initialized
- `ErrInvalidCacheDir`: The specified cache directory is invalid
- `ErrCleanInProgress`: A cleanup operation is already in progress

## Testing

Run the tests with:

```bash
go test -v ./pkg/cache
```

## License

MIT
