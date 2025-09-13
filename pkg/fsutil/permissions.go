// Artifact fsutil provides utility functions and constants for file system operations.
package fsutil

// File and directory permission constants.
const (
	// File mode masks.
	FileModeMask = 0o777 // Full permission mask for files
	DirModeMask  = 0o777 // Full permission mask for directories

	// Default file modes.
	FileModeDefault = 0o644 // -rw-r--r--
	FileModeSecure  = 0o640 // -rw-r-----
	FileModeExec    = 0o755 // -rwxr-xr-x

	// Default directory modes.
	DirModeDefault = 0o755 // drwxr-xr-x
	DirModeSecure  = 0o750 // drwxr-x---
	DirModePrivate = 0o700 // drwx------

	// Special file modes.
	Umask = 0o022 // Default umask value
)
