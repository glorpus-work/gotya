// Artifact fsutil provides utility functions and constants for file system operations.
package fsutil

import (
	"os"
	"path/filepath"

	"github.com/cperrin88/gotya/pkg/permissions"
)

// EnsureDir creates a directory and all necessary parent directories with default permissions if they don't exist.
// It uses DirModeDefault (0755) permissions for the created directories.
// Returns an error if the directory cannot be created or if the path exists but is not a directory.
func EnsureDir(path string) error {
	return os.MkdirAll(path, permissions.DirModeDefault)
}

// EnsureFileDir creates the parent directory of a file path if it doesn't exist.
// This is useful when you want to ensure a directory exists before creating a file.
// It uses EnsureDir internally to create the parent directory with default permissions.
// Returns an error if the parent directory cannot be created.
func EnsureFileDir(filePath string) error {
	dir := filepath.Dir(filePath)
	return EnsureDir(dir)
}
