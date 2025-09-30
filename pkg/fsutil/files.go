package fsutil

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	pkgerrors "github.com/cperrin88/gotya/pkg/errors"
)

// Move moves a file or directory from src to dst.
// It first attempts to use os.Rename for atomic operation.
// If that fails due to cross-filesystem boundaries, it falls back to copy + delete.
// Returns an error if the move operation fails.
func Move(src, dst string) error {
	if src == "" || dst == "" {
		return pkgerrors.ErrEmptyPaths
	}

	// Get source info to determine if it's a file or directory
	srcInfo, err := os.Stat(src)
	if err != nil {
		return pkgerrors.Wrapf(err, "failed to stat source %s", src)
	}

	// Ensure destination directory exists for files
	if !srcInfo.IsDir() {
		dstDir := filepath.Dir(dst)
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return pkgerrors.Wrapf(err, "failed to create destination directory %s", dstDir)
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
		return pkgerrors.Wrapf(err, "failed to rename %s to %s", src, dst)
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

	return false
}

// moveFile handles moving a single file across filesystem boundaries.
func moveFile(src, dst string) error {
	// Copy file contents
	if err := Copy(src, dst); err != nil {
		return pkgerrors.Wrapf(err, "failed to copy file %s to %s", src, dst)
	}

	// Get source file info to preserve metadata
	srcInfo, err := os.Stat(src)
	if err != nil {
		// If we can't get source info, at least try to remove the source
		_ = os.Remove(src)
		return pkgerrors.Wrap(err, "failed to stat source file after copy")
	}

	// Preserve file permissions and modification time
	if err := os.Chmod(dst, srcInfo.Mode()); err != nil {
		_ = os.Remove(src) // Clean up source even if chmod fails
		return pkgerrors.Wrapf(err, "failed to set permissions on %s", dst)
	}

	if err := os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
		_ = os.Remove(src) // Clean up source even if chtimes fails
		return pkgerrors.Wrapf(err, "failed to set modification time on %s", dst)
	}

	// Remove source file
	if err := os.Remove(src); err != nil {
		return pkgerrors.Wrapf(err, "failed to remove source file %s after copy", src)
	}

	return nil
}

// moveDirectory handles moving a directory across filesystem boundaries.
func moveDirectory(src, dst string) error {
	// Create destination directory with same permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return pkgerrors.Wrapf(err, "failed to stat source directory %s", src)
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return pkgerrors.Wrapf(err, "failed to create destination directory %s", dst)
	}

	// Walk through source directory and copy all contents
	err = filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path from source directory
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return pkgerrors.Wrapf(err, "failed to get relative path for %s", path)
		}

		// Construct destination path
		dstPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			// Create directory with same permissions
			if err := os.MkdirAll(dstPath, d.Type()); err != nil {
				return pkgerrors.Wrapf(err, "failed to create directory %s", dstPath)
			}
		} else {
			// Copy file
			if err := Copy(path, dstPath); err != nil {
				return pkgerrors.Wrapf(err, "failed to copy file %s to %s", path, dstPath)
			}

			// Preserve file permissions and modification time
			srcFileInfo, err := d.Info()
			if err != nil {
				return pkgerrors.Wrapf(err, "failed to get file info for %s", path)
			}

			if err := os.Chmod(dstPath, srcFileInfo.Mode()); err != nil {
				return pkgerrors.Wrapf(err, "failed to set permissions on %s", dstPath)
			}

			if err := os.Chtimes(dstPath, srcFileInfo.ModTime(), srcFileInfo.ModTime()); err != nil {
				return pkgerrors.Wrapf(err, "failed to set modification time on %s", dstPath)
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
		return pkgerrors.Wrapf(err, "failed to remove source directory %s after copy", src)
	}

	return nil
}

// Copy copies the contents of srcFile to dstFile.
func Copy(srcFile, dstFile string) error {
	src, err := os.Open(srcFile)
	if err != nil {
		return pkgerrors.Wrapf(err, "failed to open source file %s", srcFile)
	}
	defer func() {
		if closeErr := src.Close(); closeErr != nil {
			// Log error but don't override copy error
			_ = closeErr
		}
	}()

	dst, err := os.Create(dstFile)
	if err != nil {
		return pkgerrors.Wrapf(err, "failed to create destination file %s", dstFile)
	}
	defer func() {
		if closeErr := dst.Close(); closeErr != nil {
			// Log error but don't override copy error
			_ = closeErr
		}
	}()

	_, err = io.Copy(dst, src)
	if err != nil {
		return pkgerrors.Wrapf(err, "failed to copy from %s to %s", srcFile, dstFile)
	}

	return nil
}

// CreateFilePerm creates a new file with the specified permissions.
func CreateFilePerm(name string, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
}
