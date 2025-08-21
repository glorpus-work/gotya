package pkg

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cperrin88/gotya/pkg/util"
)

// PackageStructure represents the expected structure of a package
type PackageStructure struct {
	FilesDir   string // Directory containing files to install
	ScriptsDir string // Directory containing pre/post install scripts
	Metadata   *PackageMetadata
}

// ExtractPackage extracts an archive package and returns its structure
func ExtractPackage(packagePath, extractDir string) (*PackageStructure, error) {
	// Extract the archive using the appropriate method
	if err := ExtractArchive(packagePath, extractDir); err != nil {
		return nil, fmt.Errorf("failed to extract archive: %w", err)
	}

	// Parse the package structure
	structure, err := parsePackageStructure(extractDir)
	if err != nil {
		return nil, fmt.Errorf("failed to parse package structure: %w", err)
	}

	return structure, nil
}

// extractTarGz extracts a tar.gz file using stdlib gzip
func extractTarGz(packagePath, extractDir string) error {
	// Open the .tar.gz file
	file, err := os.Open(packagePath)
	if err != nil {
		return fmt.Errorf("failed to open package file: %w", err)
	}
	defer file.Close()

	// Create gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Extract tar from decompressed stream
	return extractTar(gzReader, extractDir)
}

// ExtractArchive extracts tar.gz archive files
func ExtractArchive(packagePath, extractDir string) error {
	// Create extraction directory
	if err := util.EnsureDir(extractDir); err != nil {
		return fmt.Errorf("failed to create extraction directory: %w", err)
	}

	// Check for supported file extension
	if !strings.HasSuffix(packagePath, ".tar.gz") && !strings.HasSuffix(packagePath, ".tgz") {
		return fmt.Errorf("unsupported archive format: %s (only .tar.gz and .tgz files are supported)", packagePath)
	}

	return extractTarGz(packagePath, extractDir)
}

// tarExtractor handles the extraction of tar archives
type tarExtractor struct {
	tarReader  *tar.Reader
	extractDir string
}

// newTarExtractor creates a new tarExtractor instance
func newTarExtractor(reader io.Reader, extractDir string) *tarExtractor {
	return &tarExtractor{
		tarReader:  tar.NewReader(reader),
		extractDir: extractDir,
	}
}

// validatePath ensures the target path is safe and within the extraction directory
func (e *tarExtractor) validatePath(header *tar.Header) (string, error) {
	// Sanitize the path to prevent directory traversal
	targetPath := filepath.Join(e.extractDir, filepath.Clean("/"+header.Name))

	// Security check: ensure target path is within extraction directory
	relPath, err := filepath.Rel(e.extractDir, targetPath)
	if err != nil || strings.HasPrefix(relPath, "..") || strings.HasPrefix(relPath, ".") {
		return "", fmt.Errorf("invalid file path in archive: %s", header.Name)
	}

	// Additional check for absolute paths or path traversal
	if !filepath.IsAbs(e.extractDir) || filepath.IsAbs(header.Name) || strings.Contains(header.Name, "..") {
		return "", fmt.Errorf("invalid file path in archive: %s", header.Name)
	}

	return targetPath, nil
}

// extractDirectory handles the extraction of a directory entry
func (e *tarExtractor) extractDirectory(header *tar.Header, targetPath string) error {
	// Ensure the directory exists with secure permissions
	if err := util.EnsureDir(targetPath); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
	}

	// Set the original mode if it's more restrictive than our default
	if header.Mode&0777 < 0750 {
		if err := os.Chmod(targetPath, os.FileMode(header.Mode)); err != nil {
			return fmt.Errorf("failed to set permissions for %s: %w", targetPath, err)
		}
	}
	return nil
}

// extractRegularFile handles the extraction of a regular file
func (e *tarExtractor) extractRegularFile(header *tar.Header, targetPath string) error {
	// Create directory if it doesn't exist
	if err := util.EnsureFileDir(targetPath); err != nil {
		return fmt.Errorf("failed to create parent directory for %s: %w", targetPath, err)
	}

	// Create the file with the specified mode
	file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", targetPath, err)
	}

	// Copy file content
	if _, err := io.Copy(file, e.tarReader); err != nil {
		file.Close()
		return fmt.Errorf("failed to extract file %s: %w", targetPath, err)
	}

	return file.Close()
}

// extractSymlink handles the extraction of a symlink
func (e *tarExtractor) extractSymlink(header *tar.Header, targetPath string) error {
	// On Windows, we need to be careful with symlinks
	// First remove the target if it exists
	if _, err := os.Lstat(targetPath); err == nil {
		if err := os.Remove(targetPath); err != nil {
			return fmt.Errorf("failed to remove existing symlink %s: %w", targetPath, err)
		}
	}

	// Create the symlink
	if err := os.Symlink(header.Linkname, targetPath); err != nil {
		return fmt.Errorf("failed to create symlink %s: %w", targetPath, err)
	}

	return nil
}

// extractHardlink handles the extraction of a hard link
func (e *tarExtractor) extractHardlink(header *tar.Header, targetPath string) error {
	// Sanitize link target path
	linkTarget := filepath.Join(e.extractDir, filepath.Clean("/"+header.Linkname))

	// Verify link target is within extraction directory
	relLinkPath, err := filepath.Rel(e.extractDir, linkTarget)
	if err != nil || strings.HasPrefix(relLinkPath, "..") || strings.HasPrefix(relLinkPath, ".") {
		return fmt.Errorf("invalid link target in archive: %s", header.Linkname)
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

// extractTar extracts a tar stream to the specified directory
func extractTar(reader io.Reader, extractDir string) error {
	extractor := newTarExtractor(reader, extractDir)

	for {
		header, err := extractor.tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		targetPath, err := extractor.validatePath(header)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := extractor.extractDirectory(header, targetPath); err != nil {
				return err
			}

		case tar.TypeReg:
			if err := extractor.extractRegularFile(header, targetPath); err != nil {
				return err
			}

		case tar.TypeSymlink:
			if err := extractor.extractSymlink(header, targetPath); err != nil {
				return err
			}

		case tar.TypeLink:
			if err := extractor.extractHardlink(header, targetPath); err != nil {
				return err
			}

		// Add support for other file types if needed
		default:
			return fmt.Errorf("unsupported file type %v in archive", header.Typeflag)
		}
	}

	return nil
}

// parsePackageStructure parses the extracted package directory structure
func parsePackageStructure(extractDir string) (*PackageStructure, error) {
	structure := &PackageStructure{}

	// Look for metadata.json file
	metadataPath := filepath.Join(extractDir, "metadata.json")
	if _, err := os.Stat(metadataPath); err == nil {
		file, err := os.Open(metadataPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open metadata file: %w", err)
		}
		defer file.Close()

		metadata, err := ParseMetadataFromReader(file)
		if err != nil {
			return nil, fmt.Errorf("failed to parse metadata: %w", err)
		}
		structure.Metadata = metadata
	} else {
		return nil, fmt.Errorf("metadata.json not found in package")
	}

	// Look for common directory structures
	possibleFilesDirs := []string{"files", "data", "usr", "opt"}
	for _, dir := range possibleFilesDirs {
		dirPath := filepath.Join(extractDir, dir)
		if stat, err := os.Stat(dirPath); err == nil && stat.IsDir() {
			structure.FilesDir = dirPath
			break
		}
	}

	// Look for scripts directory
	possibleScriptsDirs := []string{"scripts", "DEBIAN", "control"}
	for _, dir := range possibleScriptsDirs {
		dirPath := filepath.Join(extractDir, dir)
		if stat, err := os.Stat(dirPath); err == nil && stat.IsDir() {
			structure.ScriptsDir = dirPath
			break
		}
	}

	return structure, nil
}

// TODO: Script execution would require external commands or Go script evaluation
// For now, we'll focus on the file extraction and installation parts
// Scripts could be handled by:
// 1. Using a Go script engine like tengo or yaegi
// 2. Having predefined hooks in Go code
// 3. Using a sandboxed execution environment

// CopyFiles recursively copies files from src to dst, tracking installed files
func CopyFiles(src, dst string) ([]string, error) {
	var installedFiles []string

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path from src
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			// Create directory with secure permissions
			if err := util.EnsureDir(targetPath); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
			// Set the original mode if it's more restrictive than our default
			if info.Mode()&0777 < 0750 {
				if err := os.Chmod(targetPath, info.Mode()); err != nil {
					return fmt.Errorf("failed to set permissions for %s: %w", targetPath, err)
				}
			}
		} else if info.Mode()&os.ModeSymlink != 0 {
			// Handle symlink
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("failed to read symlink %s: %w", path, err)
			}
			if err := os.Symlink(linkTarget, targetPath); err != nil {
				return fmt.Errorf("failed to create symlink %s: %w", targetPath, err)
			}
			installedFiles = append(installedFiles, targetPath)
		} else {
			// Copy regular file
			if err := copyFile(path, targetPath, info.Mode()); err != nil {
				return fmt.Errorf("failed to copy file %s: %w", path, err)
			}
			installedFiles = append(installedFiles, targetPath)
		}

		return nil
	})

	return installedFiles, err
}

// copyFile copies a single file from src to dst with the given mode
func copyFile(src, dst string, mode os.FileMode) error {
	// Ensure destination directory exists
	if err := util.EnsureFileDir(dst); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// RemoveFiles removes all files in the given list
func RemoveFiles(files []string) error {
	var errors []string

	// Remove files in reverse order (deepest first)
	for i := len(files) - 1; i >= 0; i-- {
		if err := os.Remove(files[i]); err != nil && !os.IsNotExist(err) {
			errors = append(errors, fmt.Sprintf("failed to remove %s: %v", files[i], err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors removing files: %s", strings.Join(errors, "; "))
	}

	return nil
}
