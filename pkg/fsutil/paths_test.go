package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cperrin88/gotya/pkg/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCacheDir(t *testing.T) {
	expected, err := os.UserCacheDir()
	require.NoError(t, err)
	expected = filepath.Join(expected, "gotya")

	cacheDir, err := GetCacheDir()
	assert.NoError(t, err)
	assert.Equal(t, expected, cacheDir)
}

func TestGetDataDir(t *testing.T) {
	var expectedBase, expected string
	var err error

	switch runtime.GOOS {
	case platform.OSWindows:
		expectedBase = os.Getenv("LOCALAPPDATA")
		require.NotEmpty(t, expectedBase, "LOCALAPPDATA environment variable not set")
	case platform.OSDarwin:
		home, err := os.UserHomeDir()
		require.NoError(t, err)
		expectedBase = filepath.Join(home, "Library", "Application Support")
	default: // Linux, BSD, etc.
		if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
			expectedBase = xdgDataHome
		} else {
			home, err := os.UserHomeDir()
			require.NoError(t, err)
			expectedBase = filepath.Join(home, ".local", "share")
		}
	}

	expected = filepath.Join(expectedBase, "gotya")

	dataDir, err := GetDataDir()
	assert.NoError(t, err)
	assert.Equal(t, expected, dataDir)
}

func TestEnsureDirs(t *testing.T) {
	// Clean up after test
	t.Cleanup(func() {
		cacheDir, _ := GetCacheDir()
		dataDir, _ := GetDataDir()
		os.RemoveAll(cacheDir)
		os.RemoveAll(dataDir)
	})

	err := EnsureDirs()
	assert.NoError(t, err)

	// Verify directories were created
	cacheDir, _ := GetCacheDir()
	dataDir, _ := GetDataDir()

	assert.DirExists(t, filepath.Join(cacheDir, "packages"))
	assert.DirExists(t, filepath.Join(dataDir, "installed"))
	assert.DirExists(t, filepath.Join(dataDir, "meta"))
}
