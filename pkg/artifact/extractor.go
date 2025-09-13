package artifact

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cperrin88/gotya/pkg/fsutil"
	"github.com/cperrin88/gotya/pkg/permissions"
)

// Artifact artifact provides functionality for working with artifact files and metadata.
// It handles artifact creation, extraction, and management with support for various
// archive formats and artifact metadata.

// ArtifactStructure represents the expected structure of a artifact.
type ArtifactStructure struct {
	FilesDir   string            `json:"files_dir"`   // Directory containing files to install.
	ScriptsDir string            `json:"scripts_dir"` // Directory containing pre/post install scripts.
	Metadata   *ArtifactMetadata `json:"metadata"`    // Artifact metadata.
}

// ExtractArtifact extracts an archive artifact and returns its structure.
func ExtractArtifact(packagePath, extractDir string) (*ArtifactStructure, error) {
	// Extract the archive using the appropriate method
	if err := ExtractArchive(packagePath, extractDir); err != nil {
		return nil, fmt.Errorf("failed to extract archive: %w", err)
	}

	// Parse the artifact structure
	structure, err := parseArtifactStructure(extractDir)
	if err != nil {
		return nil, fmt.Errorf("failed to parse artifact structure: %w", err)
	}

	return structure, nil
}

// wrapExtractorError wraps an error with additional context about the extraction process.
// It handles common system errors and adds file path context when available.
func wrapExtractorError(err error, path string) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("file not found: %w", err)
	case errors.Is(err, os.ErrPermission):
		return fmt.Errorf("permission denied: %w", err)
	case errors.Is(err, os.ErrExist):
		return fmt.Errorf("file already exists: %w", err)
	default:
		if path != "" {
			return fmt.Errorf("error processing %s: %w", filepath.ToSlash(path), err)
		}
		return err
	}
}

// extractTarGz extracts a tar.gz file to the specified directory.
func extractTarGz(gzipStream io.Reader, extractDir string) error {
	uncompressedStream, err := gzip.NewReader(gzipStream)
	if err != nil {
		return wrapExtractorError(err, "gzip reader")
	}
	defer uncompressedStream.Close()

	return extractTar(uncompressedStream, extractDir)
}

// ExtractArchive extracts tar.gz archive files.
func ExtractArchive(packagePath, extractDir string) error {
	if !strings.HasSuffix(strings.ToLower(packagePath), ".tar.gz") &&
		!strings.HasSuffix(strings.ToLower(packagePath), ".tgz") {
		return fmt.Errorf("%w: %s", ErrUnsupportedArchiveFormat, filepath.Ext(packagePath))
	}

	file, err := os.Open(packagePath)
	if err != nil {
		return wrapExtractorError(err, packagePath)
	}
	defer file.Close()

	return extractTarGz(file, extractDir)
}

// tarExtractor handles the extraction of tar archives.
type tarExtractor struct {
	tarReader  *tar.Reader
	extractDir string
}

// newTarExtractor creates a new tarExtractor instance.
func newTarExtractor(reader io.Reader, extractDir string) *tarExtractor {
	return &tarExtractor{
		tarReader:  tar.NewReader(reader),
		extractDir: extractDir,
	}
}

// validatePath ensures the target path is safe and within the extraction directory.
func (t *tarExtractor) validatePath(header *tar.Header) (string, error) {
	// Skip the current directory entry
	if header.Name == "." {
		return "", nil
	}

	// Clean the path to prevent directory traversal
	cleanPath := filepath.Clean(header.Name)
	if cleanPath == "." || cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
		return "", fmt.Errorf("%w: %s", ErrInvalidFilePath, header.Name)
	}

	targetPath := filepath.Join(t.extractDir, cleanPath)

	// Check for path traversal using filepath.Rel
	relPath, err := filepath.Rel(t.extractDir, targetPath)
	if err != nil || strings.HasPrefix(relPath, "..") || (relPath != "" && relPath[0] == '/') {
		return "", fmt.Errorf("%w: %s", ErrInvalidFilePath, header.Name)
	}

	// Ensure the target is within the extraction directory
	if !filepath.IsAbs(targetPath) {
		absPath, err := filepath.Abs(targetPath)
		if err != nil {
			return "", fmt.Errorf("%w: %s", ErrInvalidFilePath, header.Name)
		}
		targetPath = absPath
	}

	if !strings.HasPrefix(targetPath, t.extractDir) {
		return "", fmt.Errorf("%w: %s", ErrInvalidFilePath, header.Name)
	}

	return targetPath, nil
}

// safeFileMode converts tar header mode to os.FileMode safely.
func safeFileMode(mode int64) os.FileMode {
	// Use type assertion to ensure we handle the conversion safely
	var perm os.FileMode
	if mode >= 0 && mode <= int64(permissions.FileModeMask) {
		// Safe to convert directly since we've checked the bounds
		perm = os.FileMode(mode)
	} else {
		// If mode is out of bounds, use a safe default (read/write for owner, read for others)
		perm = permissions.FileModeDefault
	}
	return perm
}

// ensureDir creates a directory and all necessary parent directories with default permissions if they don't exist.
//
// Deprecated: Use fsutil.EnsureDir instead.
func ensureDir(dirPath string) error {
	return fsutil.EnsureDir(dirPath)
}

// ensureFileDir creates the parent directory of a file path if it doesn't exist.
//
// Deprecated: Use fsutil.EnsureFileDir instead.
func ensureFileDir(filePath string) error {
	return fsutil.EnsureFileDir(filePath)
}

// extractDirectory handles the extraction of a directory entry.
func (t *tarExtractor) extractDirectory(header *tar.Header, targetPath string) error {
	// Create the directory with default permissions
	if err := os.MkdirAll(targetPath, permissions.DirModeDefault); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
	}

	// Set the original mode if it's more restrictive than our default
	if header.Mode&permissions.FileModeMask < permissions.DirModeDefault {
		if err := os.Chmod(targetPath, safeFileMode(header.Mode)); err != nil {
			return fmt.Errorf("failed to set permissions for %s: %w", targetPath, err)
		}
	}
	return nil
}

// extractRegularFile handles the extraction of a regular file.
func (t *tarExtractor) extractRegularFile(_ *tar.Header, targetPath string) error {
	// Create directory if it doesn't exist
	if err := ensureFileDir(targetPath); err != nil {
		return fmt.Errorf("failed to create parent directory for %s: %w", targetPath, err)
	}

	// Create the file with default permissions
	file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, permissions.FileModeDefault)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", targetPath, err)
	}

	// Copy file content
	if _, err := io.Copy(file, t.tarReader); err != nil {
		file.Close()
		return fmt.Errorf("failed to extract file %s: %w", targetPath, err)
	}

	return file.Close()
}

// validateSymlinkTarget checks if a symlink target is valid and within the base directory.
// Returns an error if the target is invalid or points outside the base directory.
func validateSymlinkTarget(linkPath, linkTarget, baseDir string) error {
	// Resolve the target relative to the link's directory
	targetPath := filepath.Join(filepath.Dir(linkPath), linkTarget)
	targetPath = filepath.Clean(targetPath)

	// Get absolute paths for both target and base directory
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("invalid symlink target: %w", err)
	}

	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("invalid base directory: %w", err)
	}

	// Ensure the target is within the base directory
	if !strings.HasPrefix(targetAbs, baseAbs+string(os.PathSeparator)) {
		return fmt.Errorf("%w: %s", ErrInvalidSymlinkTarget, linkTarget)
	}

	return nil
}

// extractSymlink handles the extraction of a symlink.
func (t *tarExtractor) extractSymlink(header *tar.Header, targetPath string) error {
	// Validate symlink target
	if !filepath.IsAbs(header.Linkname) {
		targetDir := filepath.Dir(targetPath)
		if err := validateSymlinkTarget(targetPath, header.Linkname, t.extractDir); err != nil {
			return err
		}

		// Create parent directories if they don't exist
		if err := os.MkdirAll(targetDir, permissions.DirModeDefault); err != nil {
			return wrapExtractorError(err, targetDir)
		}

		// Create the symlink
		if err := os.Symlink(header.Linkname, targetPath); err != nil {
			return wrapExtractorError(err, targetPath)
		}
	} else {
		return fmt.Errorf("%w: %s", ErrInvalidLinkTarget, header.Linkname)
	}

	return nil
}

// extractHardlink handles the extraction of a hard link.
func (t *tarExtractor) extractHardlink(header *tar.Header, targetPath string) error {
	// Sanitize link target path
	linkTarget := filepath.Join(t.extractDir, filepath.Clean("/"+header.Linkname))

	// Verify link target is within extraction directory
	relLinkPath, err := filepath.Rel(t.extractDir, linkTarget)
	if err != nil || strings.HasPrefix(relLinkPath, "..") || strings.HasPrefix(relLinkPath, ".") {
		return fmt.Errorf("%w: %s", ErrInvalidLinkTarget, header.Linkname)
	}

	// On Windows, we need to be careful with hard links
	// First remove the target if it exists
	if _, err := os.Lstat(targetPath); err == nil {
		if err := os.Remove(targetPath); err != nil {
			return fmt.Errorf("failed to remove existing file %s: %w", targetPath, err)
		}
	}

	// Create the hard link
	if err := os.Link(linkTarget, targetPath); err != nil {
		return fmt.Errorf("failed to create hard link %s: %w", targetPath, err)
	}

	return nil
}

// extractTar extracts a tar stream to the specified directory.
func extractTar(reader io.Reader, extractDir string) error {
	extractor := newTarExtractor(reader, extractDir)

	for {
		header, err := extractor.tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tar entry: %w", err)
		}

		targetPath, err := extractor.validatePath(header)
		if err != nil {
			return fmt.Errorf("invalid path validation: %w", err)
		}

		// Skip entries with empty paths (like the current directory entry)
		if targetPath == "" {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := extractor.extractDirectory(header, targetPath); err != nil {
				return fmt.Errorf("error extracting directory %s: %w", header.Name, err)
			}
		case tar.TypeReg, tar.TypeGNUSparse:
			if err := extractor.extractRegularFile(header, targetPath); err != nil {
				return fmt.Errorf("error extracting file %s: %w", header.Name, err)
			}
		case tar.TypeSymlink:
			if err := extractor.extractSymlink(header, targetPath); err != nil {
				return fmt.Errorf("error extracting symlink %s: %w", header.Name, err)
			}
		case tar.TypeLink:
			if err := extractor.extractHardlink(header, targetPath); err != nil {
				return fmt.Errorf("error extracting hardlink %s: %w", header.Name, err)
			}
		default:
			return fmt.Errorf("%w: %v", ErrUnsupportedFileType, header.Typeflag)
		}
	}

	return nil
}

// LoadMetadata loads artifact metadata from a file.
func LoadMetadata(metadataPath string) (*ArtifactMetadata, error) {
	file, err := os.Open(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("open metadata file: %w", err)
	}
	defer file.Close()

	var metadata ArtifactMetadata
	if err := json.NewDecoder(file).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("decode metadata: %w", err)
	}

	return &metadata, nil
}

// parseArtifactStructure parses the extracted artifact directory structure.
func parseArtifactStructure(extractDir string) (*ArtifactStructure, error) {
	// Look for metadata file in meta/artifact.json
	metadataPath := filepath.Join(extractDir, "meta", "artifact.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		// Fall back to artifact.json for backward compatibility
		metadataPath = filepath.Join(extractDir, "artifact.json")
		if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
			return nil, ErrMetadataNotFound
		} else if err != nil {
			return nil, fmt.Errorf("error checking for metadata file: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("error checking for metadata file: %w", err)
	}

	// Load metadata
	metadata, err := LoadMetadata(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("error loading artifact metadata: %w", err)
	}

	// Determine artifact structure
	structure := &ArtifactStructure{
		Metadata: metadata,
	}

	// Check for standard directory structure
	filesDir := filepath.Join(extractDir, "files")
	if _, err := os.Stat(filesDir); err == nil {
		structure.FilesDir = filesDir
	}

	scriptsDir := filepath.Join(extractDir, "scripts")
	if _, err := os.Stat(scriptsDir); err == nil {
		structure.ScriptsDir = scriptsDir
	}

	// If no standard structure, assume the root is the files directory
	if structure.FilesDir == "" {
		structure.FilesDir = extractDir
	}

	return structure, nil
}

// CopyFiles recursively copies files from src to dst, tracking installed files.
func CopyFiles(src, dst string) ([]string, error) {
	var installedFiles []string

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path from src
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		targetPath := filepath.Join(dst, relPath)

		switch {
		case info.IsDir():
			if err := ensureDir(targetPath); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}

		case info.Mode()&os.ModeSymlink != 0:
			// Handle symlink
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("failed to read symlink %s: %w", path, err)
			}

			// Validate the symlink target is safe
			if err := validateSymlinkTarget(path, linkTarget, dst); err != nil {
				return fmt.Errorf("invalid symlink %s: %w", path, err)
			}

			// Create the parent directory if it doesn't exist
			if err := ensureFileDir(targetPath); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", targetPath, err)
			}

			// Remove existing symlink or file if it exists
			if err := os.RemoveAll(targetPath); err != nil {
				return fmt.Errorf("failed to remove existing symlink %s: %w", targetPath, err)
			}

			// Create the symlink
			if err := os.Symlink(linkTarget, targetPath); err != nil {
				return fmt.Errorf("failed to create symlink %s -> %s: %w", targetPath, linkTarget, err)
			}

			installedFiles = append(installedFiles, targetPath)

		default:
			// Regular file
			if err := copyFile(path, targetPath, info.Mode()); err != nil {
				return fmt.Errorf("failed to copy file %s: %w", path, err)
			}
			installedFiles = append(installedFiles, targetPath)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to copy files: %w", err)
	}

	return installedFiles, nil
}

// copyFile copies a single file from src to dst with the given mode.
// If mode is 0, permissions.FileModeDefault will be used.
func copyFile(src, dst string, mode os.FileMode) error {
	// Ensure destination directory exists
	if err := ensureFileDir(dst); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer srcFile.Close()

	// Use default file mode if none specified
	if mode == 0 {
		mode = permissions.FileModeDefault
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer dstFile.Close()

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy data from %s to %s: %w", src, dst, err)
	}

	// Ensure all data is written to disk
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync data to %s: %w", dst, err)
	}

	return nil
}

// RemoveFiles removes all files in the given list.
func RemoveFiles(files []string) error {
	var errs []string

	for _, file := range files {
		if err := os.RemoveAll(file); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", file, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors removing files: %s", strings.Join(errs, "; "))
	}

	return nil
}
