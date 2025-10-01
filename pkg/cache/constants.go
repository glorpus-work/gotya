// Package cache provides a caching mechanism for the gotya package manager.
// It includes functionality for managing cached package artifacts, metadata, and other
// temporary data to improve performance and reduce redundant downloads. The package
// supports concurrent access and provides a clean API for cache operations.
package cache

import (
	"github.com/glorpus-work/gotya/pkg/fsutil"
)

// CacheDirPerm is the default permission mode for cache directories (rwx------).
var CacheDirPerm = fsutil.DirModePrivate
