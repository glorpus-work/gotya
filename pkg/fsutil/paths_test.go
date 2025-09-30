package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureDirs(t *testing.T) {
	// Clean up after test
	t.Cleanup(func() {
		cacheDir, err := GetCacheDir()
		if err != nil {
			t.Errorf("failed to get cache dir: %v", err)
			return
		}
		dataDir, err := GetDataDir()
		if err != nil {
			t.Errorf("failed to get data dir: %v", err)
			return
		}
		_ = os.RemoveAll(cacheDir) // Best effort cleanup
		_ = os.RemoveAll(dataDir)  // Best effort cleanup
	})

	err := EnsureDirs()
	require.NoError(t, err)

	// Verify directories were created
	cacheDir, err := GetCacheDir()
	require.NoError(t, err, "should get cache directory")

	dataDir, err := GetDataDir()
	require.NoError(t, err, "should get data directory")

	assert.DirExists(t, filepath.Join(cacheDir, "packages"))
	assert.DirExists(t, filepath.Join(dataDir, "installed"))
	assert.DirExists(t, filepath.Join(dataDir, "meta"))
}

func TestEnsureDir(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) string
		checkPerms  bool
		expectError bool
	}{
		{
			name: "creates directory successfully",
			setup: func(t *testing.T) string {
				dir := filepath.Join(t.TempDir(), "newdir")
				return dir
			},
			checkPerms:  true,
			expectError: false,
		},
		{
			name: "creates nested directories",
			setup: func(t *testing.T) string {
				dir := filepath.Join(t.TempDir(), "parent", "child", "nested")
				return dir
			},
			checkPerms:  true,
			expectError: false,
		},
		{
			name: "succeeds when directory already exists",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				return dir
			},
			checkPerms:  false,
			expectError: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			path := testCase.setup(t)
			err := EnsureDir(path)

			if testCase.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.DirExists(t, path)

				// Verify permissions (only check on Unix-like systems)
				if testCase.checkPerms && runtime.GOOS != "windows" {
					info, err := os.Stat(path)
					require.NoError(t, err)
					assert.Equal(t, os.FileMode(DirModeDefault), info.Mode().Perm())
				}
			}
		})
	}
}

func TestEnsureFileDir(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) string
		checkPerms  bool
		expectError bool
	}{
		{
			name: "creates parent directory for file",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "parent", "file.txt")
			},
			checkPerms:  true,
			expectError: false,
		},
		{
			name: "creates nested parent directories for file",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nested", "parent", "file.txt")
			},
			checkPerms:  true,
			expectError: false,
		},
		{
			name: "succeeds when parent directory exists",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				return filepath.Join(dir, "file.txt")
			},
			checkPerms:  false,
			expectError: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			filePath := testCase.setup(t)
			dir := filepath.Dir(filePath)

			err := EnsureFileDir(filePath)

			if testCase.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.DirExists(t, dir)

				// Verify permissions (only check on Unix-like systems)
				if testCase.checkPerms && runtime.GOOS != "windows" {
					info, err := os.Stat(dir)
					require.NoError(t, err)
					assert.Equal(t, os.FileMode(DirModeDefault), info.Mode().Perm())
				}
			}
		})
	}
}

func TestEnsureDir_PermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission test on Windows")
	}

	// Create a directory we can't write to
	tempDir := t.TempDir()
	readonlyDir := filepath.Join(tempDir, "readonly")
	err := os.Mkdir(readonlyDir, DirModeReadOnly)
	require.NoError(t, err)

	// Try to create a subdirectory in the read-only directory
	targetDir := filepath.Join(readonlyDir, "shouldfail")
	err = EnsureDir(targetDir)

	// Verify we got a permission error
	require.Error(t, err)
	assert.False(t, os.IsExist(err), "Should not be an 'already exists' error")
}

func TestEnsureFileDir_EmptyPath(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		expectError bool
	}{
		{
			name:        "empty path",
			filePath:    "",
			expectError: false, // Empty path is handled gracefully
		},
		{
			name:        "root path",
			filePath:    "/",
			expectError: false, // Root path is valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EnsureFileDir(tt.filePath)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
