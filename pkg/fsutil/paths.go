package fsutil

import (
	"errors"
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
	switch runtime.GOOS {
	case platform.OSWindows:
		// Windows: %LOCALAPPDATA%
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			return "", errors.New("LOCALAPPDATA environment variable not set")
		}
		return localAppData, nil

	case platform.OSDarwin:
		// macOS: ~/Library/Application Support
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support"), nil

	default: // Linux, BSD, etc.
		// Use XDG_DATA_HOME with fallback to ~/.local/share
		if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
			return xdgDataHome, nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share"), nil
	}
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

// GetPackageCacheDir returns the directory for storing downloaded package archives
// Format: <cache_dir>/packages/
func GetPackageCacheDir() (string, error) {
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

// GetMetaDir returns the directory for package metadata
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
		GetPackageCacheDir,
		GetInstalledDir,
		GetMetaDir,
	}

	for _, dirFn := range dirs {
		dir, err := dirFn()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}
