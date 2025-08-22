package cache_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cperrin88/gotya/pkg/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDefaultManager(t *testing.T) {
	mgr, err := cache.NewDefaultManager()
	require.NoError(t, err)
	require.NotNil(t, mgr)

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	expectedDir := filepath.Join(homeDir, ".cache", "gotya")
	assert.Equal(t, expectedDir, mgr.GetDirectory())
}

func TestSetDirectory(t *testing.T) {
	tests := []struct {
		name        string
		directory   string
		expectError bool
	}{
		{
			name:        "valid directory",
			directory:   t.TempDir(),
			expectError: false,
		},
		{
			name:        "empty directory",
			directory:   "",
			expectError: true,
		},
		{
			name:        "non-existent directory",
			directory:   filepath.Join(t.TempDir(), "nonexistent"),
			expectError: false, // Should not error for non-existent dirs
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mgr := cache.NewManager(t.TempDir())

			err := mgr.SetDirectory(testCase.directory)

			if testCase.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, testCase.directory, mgr.GetDirectory())
			}
		})
	}
}

func TestNewManagerWithDifferentDirectories(t *testing.T) {
	tests := []struct {
		name      string
		directory string
	}{
		{"empty directory", ""},
		{"valid directory", t.TempDir()},
		{"non-existent directory", filepath.Join(t.TempDir(), "nonexistent")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := cache.NewManager(tt.directory)
			assert.NotNil(t, mgr)
			assert.Equal(t, tt.directory, mgr.GetDirectory())
		})
	}
}

func TestCleanAll(t *testing.T) {
	tempDir := t.TempDir()
	setupTestCache(t, tempDir)

	mgr := cache.NewManager(tempDir)

	// Verify files exist before cleaning
	_, err := os.Stat(filepath.Join(tempDir, "indexes", "test.index"))
	require.NoError(t, err, "index file should exist before cleaning")

	_, err = os.Stat(filepath.Join(tempDir, "packages", "test.pkg"))
	require.NoError(t, err, "package file should exist before cleaning")

	// Clean all
	result, err := mgr.Clean(cache.CleanOptions{All: true})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify files were deleted
	_, err = os.Stat(filepath.Join(tempDir, "indexes", "test.index"))
	assert.True(t, os.IsNotExist(err), "index file should be deleted")

	_, err = os.Stat(filepath.Join(tempDir, "packages", "test.pkg"))
	assert.True(t, os.IsNotExist(err), "package file should be deleted")

	// Verify the result contains the expected sizes
	assert.Greater(t, result.IndexFreed, int64(0), "should have freed some index data")
	assert.Greater(t, result.PackageFreed, int64(0), "should have freed some package data")
	assert.Equal(t, result.IndexFreed+result.PackageFreed, result.TotalFreed, "total should be sum of index and package")
}

func TestCleanIndexesOnly(t *testing.T) {
	tempDir := t.TempDir()
	setupTestCache(t, tempDir)

	mgr := cache.NewManager(tempDir)

	// Clean only indexes
	result, err := mgr.Clean(cache.CleanOptions{Indexes: true})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify index was deleted but package still exists
	_, err = os.Stat(filepath.Join(tempDir, "indexes", "test.index"))
	assert.True(t, os.IsNotExist(err), "index file should be deleted")

	_, err = os.Stat(filepath.Join(tempDir, "packages", "test.pkg"))
	assert.NoError(t, err, "package file should still exist")

	// Verify the result
	assert.Greater(t, result.IndexFreed, int64(0), "should have freed some index data")
	assert.Equal(t, int64(0), result.PackageFreed, "no package data should be freed")
	assert.Equal(t, result.IndexFreed, result.TotalFreed, "total should equal index freed")
}

func TestCleanNone(t *testing.T) {
	tempDir := t.TempDir()
	setupTestCache(t, tempDir)

	mgr := cache.NewManager(tempDir)

	// Clean nothing (should default to cleaning all)
	result, err := mgr.Clean(cache.CleanOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify both were deleted (default is to clean all)
	_, err = os.Stat(filepath.Join(tempDir, "indexes", "test.index"))
	assert.True(t, os.IsNotExist(err), "index file should be deleted")

	_, err = os.Stat(filepath.Join(tempDir, "packages", "test.pkg"))
	assert.True(t, os.IsNotExist(err), "package file should be deleted")
}

func TestCleanNonExistentDirectories(t *testing.T) {
	tempDir := t.TempDir()
	mgr := cache.NewManager(tempDir)

	// Clean when directories don't exist (should not error)
	result, err := mgr.Clean(cache.CleanOptions{All: true})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(0), result.TotalFreed, "no data should be freed from non-existent directories")
}

func TestCleanPackagesOnly(t *testing.T) {
	tempDir := t.TempDir()
	setupTestCache(t, tempDir)

	mgr := cache.NewManager(tempDir)

	// Test cleaning package cache only
	result, err := mgr.Clean(cache.CleanOptions{Packages: true})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Greater(t, result.PackageFreed, int64(0), "should have freed some package data")
	assert.Equal(t, int64(0), result.IndexFreed, "no index data should be freed")

	// Verify package file was deleted
	_, err = os.Stat(filepath.Join(tempDir, "packages", "test.pkg"))
	assert.True(t, os.IsNotExist(err), "package file should be deleted")

	// Verify index file still exists
	_, err = os.Stat(filepath.Join(tempDir, "indexes", "test.index"))
	assert.NoError(t, err, "index file should still exist")
}

func TestGetInfo(t *testing.T) {
	tempDir := t.TempDir()
	setupTestCache(t, tempDir)

	mgr := cache.NewManager(tempDir)

	info, err := mgr.GetInfo()
	require.NoError(t, err)
	require.NotNil(t, info)

	// Verify all fields are set correctly
	assert.Equal(t, tempDir, info.Directory)
	assert.Greater(t, info.TotalSize, int64(0))
	assert.Greater(t, info.IndexSize, int64(0))
	assert.Greater(t, info.PackageSize, int64(0))
	assert.Equal(t, 1, info.IndexFiles)
	assert.Equal(t, 1, info.PackageFiles)
	assert.False(t, info.LastCleaned.IsZero(), "LastCleaned should be set")
}

func TestGetInfoEmptyCache(t *testing.T) {
	tempDir := t.TempDir()
	mgr := cache.NewManager(tempDir)

	info, err := mgr.GetInfo()
	require.NoError(t, err)
	require.NotNil(t, info)

	// Verify all sizes are 0 for empty cache
	assert.Equal(t, tempDir, info.Directory)
	assert.Equal(t, int64(0), info.TotalSize)
	assert.Equal(t, int64(0), info.IndexSize)
	assert.Equal(t, int64(0), info.PackageSize)
	assert.Equal(t, 0, info.IndexFiles)
	assert.Equal(t, 0, info.PackageFiles)
}

func TestGetInfoErrorCases(t *testing.T) {
	// Test with a non-existent directory (should not error)
	tempDir := t.TempDir()
	nonExistentDir := filepath.Join(tempDir, "nonexistent")
	mgr := cache.NewManager(nonExistentDir)

	// GetInfo should not error on non-existent directories
	info, err := mgr.GetInfo()
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, nonExistentDir, info.Directory)
	assert.Equal(t, int64(0), info.TotalSize)
}

func setupTestCache(t *testing.T, baseDir string) {
	// Create test directories with secure permissions
	dirs := []string{"indexes", "packages"}
	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(baseDir, dir), 0o750)
		require.NoError(t, err, "Failed to create test directory %s", dir)
	}

	// Create test files
	err := os.WriteFile(
		filepath.Join(baseDir, "indexes", "test.index"),
		[]byte("test index data"),
		0o644,
	)
	require.NoError(t, err)

	err = os.WriteFile(
		filepath.Join(baseDir, "packages", "test.pkg"),
		[]byte("test package data"),
		0o644,
	)
	require.NoError(t, err)
}
