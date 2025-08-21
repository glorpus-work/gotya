// Package util provides utility functions for common operations.
package util

import (
	"os"
	"path/filepath"
)

const (
	// SecureDirPerm represents secure directory permissions (read/write/execute for owner, read/execute for group)
	SecureDirPerm = 0750
	// SecureFilePerm represents secure file permissions (read/write for owner, read for group)
	SecureFilePerm = 0640
)

// EnsureDir creates a directory with secure permissions if it doesn't already exist.
// It also creates any necessary parent directories with the same permissions.
func EnsureDir(path string) error {
	return os.MkdirAll(path, SecureDirPerm)
}

// EnsureFileDir ensures that the directory containing the specified file exists.
// It creates any necessary parent directories with secure permissions.
func EnsureFileDir(filePath string) error {
	dir := filepath.Dir(filePath)
	return EnsureDir(dir)
}
