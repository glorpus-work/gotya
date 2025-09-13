package cache

import "github.com/cperrin88/gotya/pkg/permissions"

// CacheDirPerm is the default permission mode for cache directories (rwx------).
var CacheDirPerm = permissions.DirModePrivate
