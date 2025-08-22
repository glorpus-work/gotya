package cache

import "fmt"

// Common cache errors.
var (
	// ErrCacheClean is returned when there's an error cleaning the cache.
	ErrCacheClean = fmt.Errorf("failed to clean cache")

	// ErrCacheInfo is returned when there's an error getting cache information.
	ErrCacheInfo = fmt.Errorf("failed to get cache info")

	// ErrCacheDirectory is returned when there's an error with the cache directory.
	ErrCacheDirectory = fmt.Errorf("invalid cache directory")
)
