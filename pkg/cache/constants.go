package cache

import "github.com/cperrin88/gotya/pkg/fsutil"

// CacheDirPerm is the default permission mode for cache directories (rwx------).
var CacheDirPerm = fsutil.DirModePrivate
