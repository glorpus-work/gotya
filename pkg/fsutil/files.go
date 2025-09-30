package fsutil

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// Move moves a file or directory from src to dst.
// It first attempts to use os.Rename for atomic operation.
// If that fails due to cross-filesystem boundaries, it falls back to copy + delete.
// Returns an error if the move operation fails.
func Move(src, dst string) error {
	if src == "" || dst == "" {
		return fmt.Errorf("source and destination paths cannot be empty")
	}

	// Get source info to determine if it's a file or directory
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source %s: %w", src, err)
	}

	// Ensure destination directory exists for files
	if !srcInfo.IsDir() {
		dstDir := filepath.Dir(dst)
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return fmt.Errorf("failed to create destination directory %s: %w", dstDir, err)
		}
	}

	// Try atomic rename first (works for both files and directories)
	err = os.Rename(src, dst)
	if err == nil {
		return nil // Success!
	}

	// Check if the error is due to cross-filesystem boundaries
	if !isCrossFilesystemError(err) {
		// Not a cross-filesystem error, return the original error
		return fmt.Errorf("failed to rename %s to %s: %w", src, dst, err)
	}

	// Cross-filesystem move required - fallback to copy + delete
	if srcInfo.IsDir() {
		return moveDirectory(src, dst)
	}
	return moveFile(src, dst)
}

// isCrossFilesystemError determines if an error from os.Rename indicates
// a cross-filesystem boundary issue that requires fallback to copy+delete.
// Uses errors.As to check for specific syscall errors rather than string matching.
func isCrossFilesystemError(err error) bool {
	if err == nil {
		return false
	}

	// Check for syscall.SyscallError which wraps syscall errors
	var linkError *os.LinkError
	if errors.As(err, &linkError) {
		// Check if the underlying error is EXDEV (cross-device link)
		if errno, ok := linkError.Err.(syscall.Errno); ok {
			return errno == syscall.EXDEV
		}
	}

	// Check for *os.PathError which might contain syscall errors
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return isCrossFilesystemError(pathErr.Err)
	}

	// Fallback to string matching for cases where error types don't work
	// (e.g., on Windows or other systems where EXDEV isn't available)
	errMsg := strings.ToLower(err.Error())
	crossDevicePatterns := []string{
		"cross-device",
		"cross device",
		"invalid cross-device",
		"resource busy", // Sometimes indicates cross-filesystem
	}

	for _, pattern := range crossDevicePatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	// On Windows, the error might be different, but let's be conservative
	// and only treat obvious cross-device errors as such
	if runtime.GOOS == "windows" {
		return strings.Contains(errMsg, "cross-device") || strings.Contains(errMsg, "device")
	}

	return false
}

// moveFile handles moving a single file across filesystem boundaries.
func moveFile(src, dst string) error {
	// Copy file contents
	if err := Copy(src, dst); err != nil {
		return fmt.Errorf("failed to copy file %s to %s: %w", src, dst, err)
	}

	// Get source file info to preserve metadata
	srcInfo, err := os.Stat(src)
	if err != nil {
		// If we can't get source info, at least try to remove the source
		_ = os.Remove(src)
		return fmt.Errorf("failed to stat source file after copy: %w", err)
	}

	// Preserve file permissions and modification time
	if err := os.Chmod(dst, srcInfo.Mode()); err != nil {
		_ = os.Remove(src) // Clean up source even if chmod fails
		return fmt.Errorf("failed to set permissions on %s: %w", dst, err)
	}

	if err := os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
		_ = os.Remove(src) // Clean up source even if chtimes fails
		return fmt.Errorf("failed to set modification time on %s: %w", dst, err)
	}

	// Remove source file
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("failed to remove source file %s after copy: %w", src, err)
	}

	return nil
}

// moveDirectory handles moving a directory across filesystem boundaries.
func moveDirectory(src, dst string) error {
	// Create destination directory with same permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source directory %s: %w", src, err)
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dst, err)
	}

	// Walk through source directory and copy all contents
	err = filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path from source directory
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}

		// Construct destination path
		dstPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			// Create directory with same permissions
			if err := os.MkdirAll(dstPath, d.Type()); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dstPath, err)
			}
		} else {
			// Copy file
			if err := Copy(path, dstPath); err != nil {
				return fmt.Errorf("failed to copy file %s to %s: %w", path, dstPath, err)
			}

			// Preserve file permissions and modification time
			srcFileInfo, err := d.Info()
			if err != nil {
				return fmt.Errorf("failed to get file info for %s: %w", path, err)
			}

			if err := os.Chmod(dstPath, srcFileInfo.Mode()); err != nil {
				return fmt.Errorf("failed to set permissions on %s: %w", dstPath, err)
			}

			if err := os.Chtimes(dstPath, srcFileInfo.ModTime(), srcFileInfo.ModTime()); err != nil {
				return fmt.Errorf("failed to set modification time on %s: %w", dstPath, err)
			}
		}

		return nil
	})

	// Check for errors during directory traversal
	if err != nil {
		return err
	}

	// Remove source directory after successful copy
	// Note: We do this outside the WalkDir to avoid issues with directory removal during traversal
	if err := os.RemoveAll(src); err != nil {
		return fmt.Errorf("failed to remove source directory %s after copy: %w", src, err)
	}

	return nil
}

// Copy copies the contents of srcFile to dstFile.
func Copy(srcFile, dstFile string) error {
	src, err := os.Open(srcFile)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", srcFile, err)
	}
	defer src.Close()

	dst, err := os.Create(dstFile)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dstFile, err)
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return fmt.Errorf("failed to copy from %s to %s: %w", srcFile, dstFile, err)
	}

	return nil
}

// CreateFilePerm creates a new file with the specified permissions.
func CreateFilePerm(name string, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
}
