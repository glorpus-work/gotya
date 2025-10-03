package cache_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glorpus-work/gotya/pkg/cache"
	"github.com/glorpus-work/gotya/pkg/fsutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDefaultManager(t *testing.T) {
	mgr, err := cache.NewDefaultManager()
	require.NoError(t, err)
	require.NotNil(t, mgr)

	// Use OS-aware user cache directory for expectation
	userCacheDir, err := os.UserCacheDir()
	require.NoError(t, err)

	expectedDir := filepath.Join(userCacheDir, "gotya")
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

	_, err = os.Stat(filepath.Join(tempDir, "packages", "test.artifact"))
	require.NoError(t, err, "artifact file should exist before cleaning")

	// Clean all
	result, err := mgr.Clean(cache.CleanOptions{All: true})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify files were deleted
	_, err = os.Stat(filepath.Join(tempDir, "indexes", "test.index"))
	assert.True(t, os.IsNotExist(err), "index file should be deleted")

	_, err = os.Stat(filepath.Join(tempDir, "packages", "test.artifact"))
	assert.True(t, os.IsNotExist(err), "artifact file should be deleted")

	// Verify the result contains the expected sizes
	assert.Positive(t, result.IndexFreed, "should have freed some index data")
	assert.Positive(t, result.ArtifactFreed, "should have freed some artifact data")
	assert.Equal(t, result.IndexFreed+result.ArtifactFreed, result.TotalFreed, "total should be sum of index and artifact")
}

func TestCleanIndexesOnly(t *testing.T) {
	tempDir := t.TempDir()
	setupTestCache(t, tempDir)

	mgr := cache.NewManager(tempDir)

	// Clean only indexes
	result, err := mgr.Clean(cache.CleanOptions{Indexes: true})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify index was deleted but artifact still exists
	_, err = os.Stat(filepath.Join(tempDir, "indexes", "test.index"))
	assert.True(t, os.IsNotExist(err), "index file should be deleted")

	_, err = os.Stat(filepath.Join(tempDir, "packages", "test.artifact"))
	require.NoError(t, err, "artifact file should still exist")

	// Verify the result
	assert.Positive(t, result.IndexFreed, "should have freed some index data")
	assert.Equal(t, int64(0), result.ArtifactFreed, "no artifact data should be freed")
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

	_, err = os.Stat(filepath.Join(tempDir, "packages", "test.artifact"))
	assert.True(t, os.IsNotExist(err), "artifact file should be deleted")
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

func TestCleanArtifactsOnly(t *testing.T) {
	tempDir := t.TempDir()
	setupTestCache(t, tempDir)

	mgr := cache.NewManager(tempDir)

	// Test cleaning artifact cache only
	result, err := mgr.Clean(cache.CleanOptions{Artifacts: true})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Positive(t, result.ArtifactFreed, "should have freed some artifact data")
	assert.Equal(t, int64(0), result.IndexFreed, "no index data should be freed")

	// Verify artifact file was deleted
	_, err = os.Stat(filepath.Join(tempDir, "packages", "test.artifact"))
	assert.True(t, os.IsNotExist(err), "artifact file should be deleted")

	// Verify index file still exists
	_, err = os.Stat(filepath.Join(tempDir, "indexes", "test.index"))
	require.NoError(t, err, "index file should still exist")
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
	assert.Positive(t, info.TotalSize)
	assert.Positive(t, info.IndexSize)
	assert.Positive(t, info.ArtifactSize)
	assert.Equal(t, 1, info.IndexFiles)
	assert.Equal(t, 1, info.ArtifactFiles)
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
	assert.Equal(t, int64(0), info.ArtifactSize)
	assert.Equal(t, 0, info.IndexFiles)
	assert.Equal(t, 0, info.ArtifactFiles)
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
		err := os.MkdirAll(filepath.Join(baseDir, dir), fsutil.DirModeSecure)
		require.NoError(t, err, "Failed to create test directory %s", dir)
	}

	// Create test files
	err := os.WriteFile(
		filepath.Join(baseDir, "indexes", "test.index"),
		[]byte("test index data"),
		fsutil.FileModeDefault,
	)
	require.NoError(t, err)

	err = os.WriteFile(
		filepath.Join(baseDir, "packages", "test.artifact"),
		[]byte("test artifact data"),
		fsutil.FileModeDefault,
	)
	require.NoError(t, err)
}

// Tests for cache.Operation

func TestNewOperation(t *testing.T) {
	tempDir := t.TempDir()
	mgr := cache.NewManager(tempDir)

	op := cache.NewOperation(mgr)
	require.NotNil(t, op)
	assert.Equal(t, tempDir, op.GetDirectory())
}

func TestOperation_Clean_All(t *testing.T) {
	tempDir := t.TempDir()
	setupTestCache(t, tempDir)

	mgr := cache.NewManager(tempDir)
	op := cache.NewOperation(mgr)

	msg, err := op.Clean(true, false, false)
	require.NoError(t, err)
	assert.Contains(t, msg, "Successfully cleaned cache")
	assert.Contains(t, msg, "Freed")
}

func TestOperation_Clean_IndexesOnly(t *testing.T) {
	tempDir := t.TempDir()
	setupTestCache(t, tempDir)

	mgr := cache.NewManager(tempDir)
	op := cache.NewOperation(mgr)

	msg, err := op.Clean(false, true, false)
	require.NoError(t, err)
	assert.Contains(t, msg, "Successfully cleaned cache")
	assert.Contains(t, msg, "Indexes:")
}

func TestOperation_Clean_PackagesOnly(t *testing.T) {
	tempDir := t.TempDir()
	setupTestCache(t, tempDir)

	mgr := cache.NewManager(tempDir)
	op := cache.NewOperation(mgr)

	msg, err := op.Clean(false, false, true)
	require.NoError(t, err)
	assert.Contains(t, msg, "Successfully cleaned cache")
	assert.Contains(t, msg, "Artifacts:")
}

func TestOperation_Clean_DefaultBehavior(t *testing.T) {
	tempDir := t.TempDir()
	setupTestCache(t, tempDir)

	mgr := cache.NewManager(tempDir)
	op := cache.NewOperation(mgr)

	// When no flags are set, should clean both indexes and packages
	msg, err := op.Clean(false, false, false)
	require.NoError(t, err)
	assert.Contains(t, msg, "Successfully cleaned cache")
}

func TestOperation_Clean_EmptyCache(t *testing.T) {
	tempDir := t.TempDir()
	mgr := cache.NewManager(tempDir)
	op := cache.NewOperation(mgr)

	msg, err := op.Clean(true, false, false)
	require.NoError(t, err)
	assert.Contains(t, msg, "No files were removed from the cache")
}

func TestOperation_GetInfo(t *testing.T) {
	tempDir := t.TempDir()
	setupTestCache(t, tempDir)

	mgr := cache.NewManager(tempDir)
	op := cache.NewOperation(mgr)

	info, err := op.GetInfo()
	require.NoError(t, err)
	assert.Contains(t, info, "Cache Information:")
	assert.Contains(t, info, "Directory:")
	assert.Contains(t, info, "Total Size:")
	assert.Contains(t, info, "Indexes:")
	assert.Contains(t, info, "Artifacts:")
	assert.Contains(t, info, "Last Cleaned:")
	assert.Contains(t, info, tempDir)
}

func TestOperation_GetInfo_EmptyCache(t *testing.T) {
	tempDir := t.TempDir()
	mgr := cache.NewManager(tempDir)
	op := cache.NewOperation(mgr)

	info, err := op.GetInfo()
	require.NoError(t, err)
	assert.Contains(t, info, "Cache Information:")
	assert.Contains(t, info, "0 B") // Empty cache should show 0 bytes
}

func TestOperation_GetDirectory(t *testing.T) {
	tempDir := t.TempDir()
	mgr := cache.NewManager(tempDir)
	op := cache.NewOperation(mgr)

	dir := op.GetDirectory()
	assert.Equal(t, tempDir, dir)
}

func TestOperation_SetDirectory(t *testing.T) {
	tempDir := t.TempDir()
	mgr := cache.NewManager(tempDir)
	op := cache.NewOperation(mgr)

	newDir := filepath.Join(tempDir, "new_cache")
	err := op.SetDirectory(newDir)
	require.NoError(t, err)
	assert.Equal(t, newDir, op.GetDirectory())
}

func TestOperation_SetDirectory_Empty(t *testing.T) {
	tempDir := t.TempDir()
	mgr := cache.NewManager(tempDir)
	op := cache.NewOperation(mgr)

	err := op.SetDirectory("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cache directory cannot be empty")
}
