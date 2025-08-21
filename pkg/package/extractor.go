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

// extractTar extracts a tar stream to the specified directory
func extractTar(reader io.Reader, extractDir string) error {
	tarReader := tar.NewReader(reader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		targetPath := filepath.Join(extractDir, header.Name)

		// Security check: ensure target path is within extraction directory
		if !strings.HasPrefix(targetPath, extractDir) {
			return fmt.Errorf("invalid file path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
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

		case tar.TypeReg:
			// Create directory if it doesn't exist
			if err := util.EnsureFileDir(targetPath); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", targetPath, err)
			}

			file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", targetPath, err)
			}

			if _, err := io.Copy(file, tarReader); err != nil {
				file.Close()
				return fmt.Errorf("failed to extract file %s: %w", targetPath, err)
			}
			file.Close()

		case tar.TypeSymlink:
			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return fmt.Errorf("failed to create symlink %s: %w", targetPath, err)
			}

		case tar.TypeLink:
			linkTarget := filepath.Join(extractDir, header.Linkname)
			if err := os.Link(linkTarget, targetPath); err != nil {
				return fmt.Errorf("failed to create hard link %s: %w", targetPath, err)
			}
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
