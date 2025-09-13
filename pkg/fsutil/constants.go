package fsutil

// File and directory permission constants.
// These follow standard Unix permission conventions and are used consistently
// throughout the application to ensure consistent file and directory permissions.
const (
	// File mode masks.
	FileModeMask = 0o777 // Full permission mask for files
	DirModeMask  = 0o777 // Full permission mask for directories

	// Default file modes.
	FileModeDefault = 0o644 // -rw-r--r--: Default for regular files
	FileModeSecure  = 0o640 // -rw-r----: For sensitive files (owner read/write, group read)
	FileModeExec    = 0o755 // -rwxr-xr-x: For executable files

	// Directory modes.
	DirModeDefault  = 0o755 // drwxr-xr-x: Default for directories
	DirModeSecure   = 0o750 // drwxr-x---: For sensitive directories (owner full, group read/execute)
	DirModePrivate  = 0o700 // drwx------: For private directories (owner only)
	DirModeReadOnly = 0o555 // dr-xr-xr-x: For read-only directories

	// Special file modes.
	Umask = 0o022 // Default umask value (removes group and other write permissions)
)
