package pkg

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cperrin88/gotya/pkg/logger"
	"github.com/cperrin88/gotya/pkg/util"
	log "github.com/sirupsen/logrus"
)

// Metadata represents the structure of package.json
// in the package meta directory
type Metadata struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Maintainer   string            `json:"maintainer,omitempty"`
	Description  string            `json:"description"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Files        []File            `json:"files,omitempty"`
	Hooks        map[string]string `json:"hooks,omitempty"`
}

// File represents a file entry in the package metadata
type File struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Mode   uint32 `json:"mode"`
	Digest string `json:"digest"`
}

// calculateFileHash calculates the SHA256 hash of a file
func calculateFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// processFiles processes all files in the source directory and updates the metadata
func processFiles(sourceDir string, meta *Metadata) error {
	filesDir := filepath.Join(sourceDir, "files")

	// Check if files directory exists
	if _, err := os.Stat(filesDir); os.IsNotExist(err) {
		return fmt.Errorf("files directory not found: %s", filesDir)
	}

	// Clear existing files
	meta.Files = []File{}

	// Walk through the files directory
	err := filepath.Walk(filesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Calculate relative path
		relPath, err := filepath.Rel(filesDir, path)
		if err != nil {
			return err
		}

		// Calculate file hash
		hash, err := calculateFileHash(path)
		if err != nil {
			return fmt.Errorf("failed to calculate hash for %s: %w", path, err)
		}

		// Add file to metadata
		meta.Files = append(meta.Files, File{
			Path:   filepath.ToSlash(relPath), // Use forward slashes for consistency
			Size:   info.Size(),
			Mode:   uint32(info.Mode()),
			Digest: hash,
		})

		return nil
	})

	return err
}

// createTarball creates a tarball from the source directory
func createTarball(sourceDir, outputPath string, meta *Metadata) (err error) {
	// Create output file with explicit permissions
	file, err := os.OpenFile(outputPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}

	// Ensure the file is closed in case of error
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close output file: %w", closeErr)
		}
	}()

	// Create buffered writer for better performance
	bufWriter := bufio.NewWriterSize(file, 32*1024) // 32KB buffer

	// Create gzip writer
	gzipWriter := gzip.NewWriter(bufWriter)

	// Ensure gzip writer is closed
	defer func() {
		if closeErr := gzipWriter.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close gzip writer: %w", closeErr)
		}
	}()

	// Create tar writer
	tarWriter := tar.NewWriter(gzipWriter)

	// Ensure tar writer is closed
	defer func() {
		if closeErr := tarWriter.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close tar writer: %w", closeErr)
		}
	}()

	// Function to add a file to the tarball
	addFileToTarball := func(filePath, tarballPath string) error {
		info, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("failed to stat file %s: %w", filePath, err)
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("failed to create tar header for %s: %w", filePath, err)
		}

		// Update the name to be relative to the package root
		header.Name = tarballPath

		// Write the header
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header for %s: %w", filePath, err)
		}

		// If it's a regular file, write its content
		if !info.IsDir() {
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("failed to open file %s: %w", filePath, err)
			}

			// Use a closure to ensure the file is closed
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return fmt.Errorf("failed to write file content for %s: %w", filePath, err)
			}
		}

		return nil
	}

	// Add metadata files
	metaDir := filepath.Join(sourceDir, "meta")
	err = filepath.Walk(metaDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking path %s: %w", path, err)
		}

		// Skip the meta directory itself
		if path == metaDir {
			return nil
		}

		// Calculate relative path within meta directory
		relPath, err := filepath.Rel(metaDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}

		// Convert to forward slashes for tarball compatibility
		tarballPath := filepath.ToSlash(filepath.Join("meta", relPath))

		return addFileToTarball(path, tarballPath)
	})

	if err != nil {
		return fmt.Errorf("error processing meta directory: %w", err)
	}

	// Add files directory
	filesDir := filepath.Join(sourceDir, "files")
	err = filepath.Walk(filesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking path %s: %w", path, err)
		}

		// Skip the files directory itself
		if path == filesDir {
			return nil
		}

		// Calculate relative path within files directory
		relPath, err := filepath.Rel(filesDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("failed to create tar header for %s: %w", path, err)
		}

		// Update the name to be relative to the package root
		header.Name = filepath.ToSlash(filepath.Join("files", relPath))

		// Write the header
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header for %s: %w", path, err)
		}

		// If it's a regular file, write its content
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open file %s: %w", path, err)
			}

			// Use a closure to ensure the file is closed
			defer func() {
				if closeErr := file.Close(); closeErr != nil && err == nil {
					err = fmt.Errorf("failed to close file %s: %w", path, closeErr)
				}
			}()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return fmt.Errorf("failed to write file content for %s: %w", path, err)
			}

			// On Windows, we don't need to explicitly sync the file as it's being read
			// and we're not modifying it, just reading its contents
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("error processing files directory: %w", err)
	}

	return nil
}

// CreatePackage creates a new package from the source directory
func CreatePackage(sourceDir, outputDir, pkgName, pkgVer, pkgOS, pkgArch string) error {
	// Normalize paths
	sourceDir = filepath.Clean(sourceDir)
	outputDir = filepath.Clean(outputDir)

	// Check if source directory exists
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return fmt.Errorf("source directory does not exist: %s", sourceDir)
	}

	// Create output directory if it doesn't exist
	if err := util.EnsureDir(outputDir); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Read package metadata
	metaPath := filepath.Join(sourceDir, "meta", "package.json")
	meta, err := readPackageMetadata(metaPath)
	if err != nil {
		return fmt.Errorf("failed to read package metadata: %w", err)
	}

	// Apply overrides if provided
	if pkgName != "" {
		meta.Name = pkgName
	}
	if pkgVer != "" {
		meta.Version = pkgVer
	}

	// Process files and update metadata
	if err := processFiles(sourceDir, meta); err != nil {
		return fmt.Errorf("failed to process files: %w", err)
	}

	// Create output filename
	if pkgOS == "" {
		pkgOS = "any"
	}
	if pkgArch == "" {
		pkgArch = "any"
	}
	outputFile := filepath.Join(outputDir, fmt.Sprintf("%s_%s_%s_%s.tar.gz",
		meta.Name, meta.Version, pkgOS, pkgArch))

	// Create tarball
	if err := createTarball(sourceDir, outputFile, meta); err != nil {
		return fmt.Errorf("failed to create package: %w", err)
	}

	// Update metadata in the tarball (optional, if you want to include the updated metadata)
	// This would require re-adding the updated package.json to the tarball

	logger.Info("Package created successfully", log.Fields{
		"file":    outputFile,
		"name":    meta.Name,
		"version": meta.Version,
		"files":   len(meta.Files),
	})

	return nil
}

// readPackageMetadata reads and parses the package metadata file
func readPackageMetadata(path string) (*Metadata, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("package metadata file not found: %s", path)
	}

	// Read the file
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open package metadata: %w", err)
	}
	defer file.Close()

	// Parse the JSON
	var meta Metadata
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&meta); err != nil {
		return nil, fmt.Errorf("failed to parse package metadata: %w", err)
	}

	// Validate required fields
	if meta.Name == "" {
		return nil, fmt.Errorf("package name is required")
	}
	if meta.Version == "" {
		return nil, fmt.Errorf("package version is required")
	}
	if meta.Description == "" {
		return nil, fmt.Errorf("package description is required")
	}

	return &meta, nil
}
