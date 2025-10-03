// Package archive provides utilities for creating and extracting artifact archives.
package archive

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/glorpus-work/gotya/pkg/fsutil"
	"github.com/mholt/archives"
)

// Manager handles archive extraction and creation operations.
type Manager struct{}

// NewManager creates a new Manager instance.
func NewManager() *Manager {
	return &Manager{}
}

// ExtractAll extracts all files from an archive to the specified destination directory
func (am *Manager) ExtractAll(ctx context.Context, archivePath, destDir string) error {
	// Open the archive file
	fsys, err := archives.FileSystem(ctx, archivePath, nil)
	if err != nil {
		return fmt.Errorf("failed to open archive file: %w", err)
	}
	// Ensure archive FS is closed after extraction
	if closer, ok := fsys.(io.Closer); ok {
		defer func() { _ = closer.Close() }()
	}

	// Ensure the destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Walk through all files in the archive and extract them via helper
	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return am.extractEntry(fsys, path, destDir, d)
	}

	return fs.WalkDir(fsys, ".", walkFn)
}

// ExtractFile extracts a specific file from an archive to the specified destination
func (am *Manager) ExtractFile(ctx context.Context, archivePath, filePath, destPath string) error {
	// Open the archive file
	fsys, err := archives.FileSystem(ctx, archivePath, nil)
	if err != nil {
		return fmt.Errorf("failed to open archive file: %w", err)
	}
	// Ensure archive FS is closed after extraction
	if closer, ok := fsys.(io.Closer); ok {
		defer func() { _ = closer.Close() }()
	}

	// Open the source file in the archive
	srcFile, err := fsys.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", filePath, err)
	}
	defer func() { _ = srcFile.Close() }()

	// Ensure the destination directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Create the destination file
	dstFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", destPath, err)
	}
	defer func() { _ = dstFile.Close() }()

	// Copy the file content
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file %s to %s: %w", filePath, destPath, err)
	}

	return nil
}

// Create creates an archive from the specified source directory
func (am *Manager) Create(ctx context.Context, sourceDir, archivePath string) error {
	// Compute absolute native and forward-slash normalized roots
	absolutePath, err := filepath.Abs(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for source directory: %w", err)
	}

	// Get files from disk with robust root variants
	archiveFiles, err := archives.FilesFromDisk(ctx, nil, map[string]string{
		absolutePath + string(os.PathSeparator): "",
	})
	if err != nil {
		return fmt.Errorf("failed to read files from disk: %w", err)
	}

	// Create the output file
	file, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", archivePath, err)
	}
	// Ensure data is flushed and handle is released promptly
	defer func() {
		_ = file.Sync()
		_ = file.Close()
	}()

	format := archives.CompressedArchive{
		Compression: archives.Gz{},
		Archival:    archives.Tar{},
	}

	// Create the archive
	err = format.Archive(ctx, file, archiveFiles)
	if err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}

	return nil
}

// extractEntry processes a single archive entry and writes it to destDir.
func (am *Manager) extractEntry(fsys fs.FS, path, destDir string, d fs.DirEntry) error {
	// Skip the root directory
	if path == "." {
		return nil
	}

	targetPath := filepath.Join(destDir, path)

	if d.IsDir() {
		return os.MkdirAll(targetPath, 0755)
	}

	// Handle regular files and symlinks
	info, err := d.Info()
	if err != nil {
		return fmt.Errorf("failed to get file info for %s: %w", path, err)
	}

	// Handle symlinks
	if info.Mode()&os.ModeSymlink != 0 {
		return am.writeSymlink(fsys, path, targetPath)
	}

	// Handle regular files
	return am.writeRegularFile(fsys, path, targetPath, info)
}

// writeSymlink creates a symlink at targetPath with contents from the archive entry at path.
func (am *Manager) writeSymlink(fsys fs.FS, path, targetPath string) error {
	linkTarget, err := fsys.Open(path)
	if err != nil {
		return fmt.Errorf("failed to read symlink %s: %w", path, err)
	}
	defer func() { _ = linkTarget.Close() }()

	// Read the symlink target
	targetBytes, err := io.ReadAll(linkTarget)
	if err != nil {
		return fmt.Errorf("failed to read symlink target %s: %w", path, err)
	}

	// Ensure the target directory exists
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory for symlink %s: %w", path, err)
	}

	// Remove existing file/symlink if it exists
	_ = os.Remove(targetPath)

	return os.Symlink(string(targetBytes), targetPath)
}

// writeRegularFile writes a regular file from the archive entry to targetPath and preserves metadata.
func (am *Manager) writeRegularFile(fsys fs.FS, path, targetPath string, info fs.FileInfo) error {
	srcFile, err := fsys.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", path, err)
	}
	defer func() { _ = srcFile.Close() }()

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory for %s: %w", path, err)
	}

	dstFile, err := fsutil.CreateFilePerm(targetPath, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", targetPath, err)
	}
	defer func() { _ = dstFile.Close() }()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file %s: %w", path, err)
	}

	if err := os.Chmod(targetPath, info.Mode().Perm()); err != nil {
		return fmt.Errorf("failed to set permissions for %s: %w", targetPath, err)
	}
	if err := os.Chtimes(targetPath, info.ModTime(), info.ModTime()); err != nil {
		return fmt.Errorf("failed to set modification time for %s: %w", targetPath, err)
	}
	return nil
}
