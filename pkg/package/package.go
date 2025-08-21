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

// tarballWriter wraps the writers needed for creating a tarball
type tarballWriter struct {
	file      *os.File
	bufWriter *bufio.Writer
	gzip      *gzip.Writer
	tar       *tar.Writer
}

// newTarballWriter creates and initializes a new tarballWriter
func newTarballWriter(outputPath string) (*tarballWriter, error) {
	// Create output file with secure permissions (owner read/write only)
	file, err := os.OpenFile(outputPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}

	// Create buffered writer for better performance
	bufWriter := bufio.NewWriterSize(file, 32*1024) // 32KB buffer

	// Create gzip writer
	gzipWriter := gzip.NewWriter(bufWriter)

	// Create tar writer
	tarWriter := tar.NewWriter(gzipWriter)

	return &tarballWriter{
		file:      file,
		bufWriter: bufWriter,
		gzip:      gzipWriter,
		tar:       tarWriter,
	}, nil
}

// close closes all writers in the correct order and returns any errors
func (tw *tarballWriter) close() error {
	var errs []error

	// Close tar writer
	if err := tw.tar.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close tar writer: %w", err))
	}

	// Close gzip writer
	if err := tw.gzip.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close gzip writer: %w", err))
	}

	// Flush buffer
	if err := tw.bufWriter.Flush(); err != nil {
		errs = append(errs, fmt.Errorf("failed to flush buffer: %w", err))
	}

	// Close file
	if err := tw.file.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close file: %w", err))
	}

	// Return first error if any
	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}

// addFileToTarball adds a single file to the tarball
func (tw *tarballWriter) addFileToTarball(filePath, tarballPath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return fmt.Errorf("failed to create tar header for %s: %w", filePath, err)
	}

	header.Name = tarballPath

	if err := tw.tar.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header for %s: %w", filePath, err)
	}

	if info.IsDir() {
		return nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	if _, err := io.Copy(tw.tar, file); err != nil {
		return fmt.Errorf("failed to write file content for %s: %w", filePath, err)
	}

	return nil
}

// processDirectory walks through a directory and adds its contents to the tarball
func (tw *tarballWriter) processDirectory(dirPath, tarballBase string) error {
	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking path %s: %w", path, err)
		}

		// Skip the base directory itself
		if path == dirPath {
			return nil
		}

		// Calculate relative path within the directory
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}

		// Create tarball path with forward slashes for compatibility
		tarballPath := filepath.ToSlash(filepath.Join(tarballBase, relPath))

		return tw.addFileToTarball(path, tarballPath)
	})
}

// createTarball creates a tarball from the source directory
func createTarball(sourceDir, outputPath string, meta *Metadata) error {
	// Initialize tarball writer
	tw, err := newTarballWriter(outputPath)
	if err != nil {
		return err
	}

	// Ensure all writers are properly closed
	defer func() {
		if closeErr := tw.close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	// Process meta directory
	metaDir := filepath.Join(sourceDir, "meta")
	if err := tw.processDirectory(metaDir, "meta"); err != nil {
		return fmt.Errorf("error processing meta directory: %w", err)
	}

	// Process files directory
	filesDir := filepath.Join(sourceDir, "files")
	if err := tw.processDirectory(filesDir, "files"); err != nil {
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
