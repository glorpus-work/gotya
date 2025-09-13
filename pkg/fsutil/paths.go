package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/cperrin88/gotya/pkg/platform"
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
	if runtime.GOOS == platform.OSLinux {
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
	return filepath.Join(configDir), nil
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
