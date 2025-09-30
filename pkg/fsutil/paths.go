// Package fsutil provides utility functions for file system operations.
package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	// AppName is the name of the application used in paths
	AppName = "gotya"
)

// GetCacheDir returns the platform-specific cache directory for the application
// On Linux: ~/.cache/gotya/
// On macOS: ~/Library/Caches/gotya/
// On Windows: %LOCALAPPDATA%\gotya\cache\
func GetCacheDir() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, AppName), nil
}

// getAppDataDir returns the platform-specific base data directory
// On Linux: ~/.local/share
// On macOS: ~/Library/Application Support
// On Windows: %LOCALAPPDATA%
func getAppDataDir() (string, error) {
	// Check for XDG_STATE_HOME environment variable - if set, always use it
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir, nil
	}

	// Special case for Linux: follow XDG Base Directory Specification
	if runtime.GOOS == "linux" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		return filepath.Join(homeDir, ".local", "share"), nil
	}

	// For all other platforms (Windows, macOS, etc.), use UserConfigDir + gotya/state
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}
	return configDir, nil
}

// GetDataDir returns the platform-specific data directory for the application
// On Linux: ~/.local/share/gotya/
// On macOS: ~/Library/Application Support/gotya/
// On Windows: %LOCALAPPDATA%\gotya\
// Other platforms follow the XDG Base Directory Specification
func GetDataDir() (string, error) {
	baseDir, err := getAppDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, AppName), nil
}

// GetArtifactCacheDir returns the directory for storing downloaded artifact archives
// Format: <cache_dir>/packages/
func GetArtifactCacheDir() (string, error) {
	cacheDir, err := GetCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "packages"), nil
}

// GetInstalledDir returns the directory for installed packages
// Format: <data_dir>/installed/
func GetInstalledDir() (string, error) {
	dataDir, err := GetDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "installed"), nil
}

// GetMetaDir returns the directory for artifact metadata
// Format: <data_dir>/meta/
func GetMetaDir() (string, error) {
	dataDir, err := GetDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "meta"), nil
}

// EnsureDir creates a directory and all necessary parent directories with default permissions if they don't exist.
// It uses DirModeDefault (0755) permissions for the created directories.
// Returns an error if the directory cannot be created or if the path exists but is not a directory.
func EnsureDir(path string) error {
	return os.MkdirAll(path, DirModeDefault)
}

// EnsureFileDir creates the parent directory of a file path if it doesn't exist.
// This is useful when you want to ensure a directory exists before creating a file.
// It uses EnsureDir internally to create the parent directory with default permissions.
// Returns an error if the parent directory cannot be created.
func EnsureFileDir(filePath string) error {
	dir := filepath.Dir(filePath)
	return EnsureDir(dir)
}

// EnsureDirs creates all necessary directories if they don't exist
func EnsureDirs() error {
	dirs := []func() (string, error){
		GetArtifactCacheDir,
		GetInstalledDir,
		GetMetaDir,
	}

	for _, dirFn := range dirs {
		dir, err := dirFn()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(dir, DirModeDefault); err != nil {
			return err
		}
	}

	return nil
}
