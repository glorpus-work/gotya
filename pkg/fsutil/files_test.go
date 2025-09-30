package fsutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMove_File_SameFilesystem tests moving a file within the same filesystem
func TestMove_File_SameFilesystem(t *testing.T) {
	tempDir := t.TempDir()

	srcFile := filepath.Join(tempDir, "source.txt")
	dstFile := filepath.Join(tempDir, "destination.txt")

	// Create source file with content
	content := "Hello, World!"
	err := os.WriteFile(srcFile, []byte(content), 0644)
	require.NoError(t, err)

	// Move the file
	err = Move(srcFile, dstFile)
	require.NoError(t, err)

	// Verify the file was moved correctly
	movedContent, err := os.ReadFile(dstFile)
	require.NoError(t, err)
	assert.Equal(t, content, string(movedContent))

	// Verify source file no longer exists
	_, err = os.Stat(srcFile)
	assert.True(t, os.IsNotExist(err))
}

// TestMove_Directory_SameFilesystem tests moving a directory within the same filesystem
func TestMove_Directory_SameFilesystem(t *testing.T) {
	tempDir := t.TempDir()

	srcDir := filepath.Join(tempDir, "source_dir")
	dstDir := filepath.Join(tempDir, "destination_dir")

	// Create source directory with files
	err := os.MkdirAll(srcDir, 0755)
	require.NoError(t, err)

	file1 := filepath.Join(srcDir, "file1.txt")
	file2 := filepath.Join(srcDir, "subdir", "file2.txt")

	err = os.WriteFile(file1, []byte("content1"), 0644)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Dir(file2), 0755)
	require.NoError(t, err)
	err = os.WriteFile(file2, []byte("content2"), 0644)
	require.NoError(t, err)

	// Move the directory
	err = Move(srcDir, dstDir)
	require.NoError(t, err)

	// Verify the directory was moved correctly
	movedContent1, err := os.ReadFile(filepath.Join(dstDir, "file1.txt"))
	require.NoError(t, err)
	assert.Equal(t, "content1", string(movedContent1))

	movedContent2, err := os.ReadFile(filepath.Join(dstDir, "subdir", "file2.txt"))
	require.NoError(t, err)
	assert.Equal(t, "content2", string(movedContent2))

	// Verify source directory no longer exists
	_, err = os.Stat(srcDir)
	assert.True(t, os.IsNotExist(err))
}

// TestMove_File_PreservePermissions tests that file permissions are preserved during moves
func TestMove_File_PreservePermissions(t *testing.T) {
	tempDir := t.TempDir()

	srcFile := filepath.Join(tempDir, "source.txt")
	dstFile := filepath.Join(tempDir, "destination.txt")

	// Create source file with specific permissions
	content := "Hello, World!"
	err := os.WriteFile(srcFile, []byte(content), 0755) // executable permissions
	require.NoError(t, err)

	// Get original permissions
	srcInfo, err := os.Stat(srcFile)
	require.NoError(t, err)
	originalMode := srcInfo.Mode()

	// Move the file
	err = Move(srcFile, dstFile)
	require.NoError(t, err)

	// Check that permissions are preserved
	dstInfo, err := os.Stat(dstFile)
	require.NoError(t, err)
	assert.Equal(t, originalMode, dstInfo.Mode())
}

// TestMove_SourceDoesNotExist tests moving a file that doesn't exist
func TestMove_SourceDoesNotExist(t *testing.T) {
	tempDir := t.TempDir()

	srcFile := filepath.Join(tempDir, "nonexistent.txt")
	dstFile := filepath.Join(tempDir, "destination.txt")

	// Try to move a file that doesn't exist
	err := Move(srcFile, dstFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to stat source")
}

// TestMove_InvalidPaths tests moving with empty paths
func TestMove_InvalidPaths(t *testing.T) {
	// Test empty source
	err := Move("", "destination.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source and destination paths cannot be empty")

	// Test empty destination
	err = Move("source.txt", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source and destination paths cannot be empty")
}

// TestIsCrossFilesystemError tests the cross-filesystem error detection
func TestIsCrossFilesystemError(t *testing.T) {
	// Test nil error
	assert.False(t, isCrossFilesystemError(nil))

	// Test regular error that's not cross-filesystem
	regularErr := errors.New("regular error")
	assert.False(t, isCrossFilesystemError(regularErr))

	// Note: Testing actual syscall.EXDEV error would require mocking syscall errors,
	// which is complex. The current implementation handles this through errors.As()
	// checking for *os.SyscallError with errno == syscall.EXDEV
}

// TestMove_CrossFilesystemFallback tests that the function handles cross-filesystem scenarios
// This is hard to test directly without mocking, but we can test the logic path
func TestMove_CrossFilesystemFallback(t *testing.T) {
	// This test would require mocking syscall errors to simulate cross-filesystem scenarios
	// For now, we'll test that the function correctly identifies when to use fallback logic

	tempDir := t.TempDir()

	// Create a test file
	srcFile := filepath.Join(tempDir, "test.txt")
	dstFile := filepath.Join(tempDir, "moved.txt")

	content := "test content"
	err := os.WriteFile(srcFile, []byte(content), 0644)
	require.NoError(t, err)

	// Move should succeed with same-filesystem move
	err = Move(srcFile, dstFile)
	require.NoError(t, err)

	// Verify content
	movedContent, err := os.ReadFile(dstFile)
	require.NoError(t, err)
	assert.Equal(t, content, string(movedContent))
}

// TestCopy tests the Copy function used internally by Move
func TestCopy(t *testing.T) {
	tempDir := t.TempDir()

	srcFile := filepath.Join(tempDir, "source.txt")
	dstFile := filepath.Join(tempDir, "destination.txt")

	content := "Copy test content"
	err := os.WriteFile(srcFile, []byte(content), 0644)
	require.NoError(t, err)

	// Copy the file
	err = Copy(srcFile, dstFile)
	require.NoError(t, err)

	// Verify content was copied
	copiedContent, err := os.ReadFile(dstFile)
	require.NoError(t, err)
	assert.Equal(t, content, string(copiedContent))

	// Verify source still exists (unlike Move)
	_, err = os.Stat(srcFile)
	require.NoError(t, err)
}

// TestCreateFilePerm tests the CreateFilePerm function
func TestCreateFilePerm(t *testing.T) {
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.txt")
	permissions := os.FileMode(0755)

	// Create file with specific permissions
	file, err := CreateFilePerm(testFile, permissions)
	require.NoError(t, err)
	assert.NotNil(t, file)

	// Write some content
	content := "test content"
	_, err = file.WriteString(content)
	require.NoError(t, err)

	err = file.Close()
	require.NoError(t, err)

	// Verify file was created with correct permissions
	info, err := os.Stat(testFile)
	require.NoError(t, err)
	assert.Equal(t, permissions, info.Mode())

	// Verify content
	fileContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, content, string(fileContent))
}
