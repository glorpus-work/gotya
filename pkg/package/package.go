package pkg

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cperrin88/gotya/pkg/logger"
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

// safePathJoin joins path elements and ensures the result is within the base directory
func safePathJoin(baseDir string, elems ...string) (string, error) {
	// Clean and join all path elements
	path := filepath.Join(append([]string{baseDir}, elems...)...)

	// Clean the path to remove any .. or .
	cleanPath := filepath.Clean(path)

	// Verify the final path is still within the base directory
	relPath, err := filepath.Rel(baseDir, cleanPath)
	if err != nil || strings.HasPrefix(relPath, "..") || strings.HasPrefix(relPath, ".") {
		return "", errors.New("invalid path: path traversal detected")
	}

	return cleanPath, nil
}

// calculateFileHash calculates the SHA256 hash of a file
func calculateFileHash(path string) (string, error) {
	// Clean the path to prevent directory traversal
	cleanPath := filepath.Clean(path)
	if !filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("path must be absolute: %s", path)
	}

	file, err := os.Open(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to calculate hash for file %s: %w", path, err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// processFiles processes all files in the source directory and updates the metadata
func processFiles(sourceDir string, meta *Metadata) error {
	filesDir := filepath.Join(sourceDir, "files")

	// Check if files directory exists
	if _, err := os.Stat(filesDir); os.IsNotExist(err) {
		return fmt.Errorf("files directory not found: %s", filesDir)
	} else if err != nil {
		return fmt.Errorf("error checking files directory %s: %w", filesDir, err)
	}

	// Clear existing files
	meta.Files = []File{}

	// Walk through the files directory
	err := filepath.Walk(filesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %s: %w", path, err)
		}

		// Skip directories and special files
		if info.IsDir() || !info.Mode().IsRegular() {
			return nil
		}

		// Calculate relative path
		relPath, err := filepath.Rel(filesDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}

		// Ensure path is clean and uses forward slashes
		relPath = filepath.ToSlash(relPath)

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
	// Clean and validate the output path
	cleanPath, err := safePathJoin(filepath.Dir(outputPath), filepath.Base(outputPath))
	if err != nil {
		return nil, fmt.Errorf("failed to create output file %s: %w", outputPath, err)
	}

	// Create the output file
	file, err := os.Create(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file %s: %w", cleanPath, err)
	}

	// Create buffered writer
	bufWriter := bufio.NewWriter(file)

	// Create gzip writer with best compression
	gzWriter, err := gzip.NewWriterLevel(bufWriter, gzip.BestCompression)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to create gzip writer: %w", err)
	}

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)

	return &tarballWriter{
		file:      file,
		bufWriter: bufWriter,
		gzip:      gzWriter,
		tar:       tarWriter,
	}, nil
}

// close closes all writers in the correct order and returns any errors
func (tw *tarballWriter) close() error {
	var errs []error

	// Close tar writer first to ensure all data is written
	if err := tw.tar.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error closing tar writer: %w", err))
	}

	// Close gzip writer to flush any remaining compressed data
	if err := tw.gzip.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error closing gzip writer: %w", err))
	}

	// Flush the buffer to ensure all data is written to the file
	if err := tw.bufWriter.Flush(); err != nil {
		errs = append(errs, fmt.Errorf("error flushing buffer: %w", err))
	}

	// Finally, close the file
	if err := tw.file.Close(); err != nil {
		errs = append(errs, fmt.Errorf("error closing file %s: %w", tw.file.Name(), err))
	}

	// Return combined errors if any occurred
	if len(errs) > 0 {
		var combinedErr error
		for _, err := range errs {
			if combinedErr == nil {
				combinedErr = err
			} else {
				combinedErr = fmt.Errorf("%v; %w", combinedErr, err)
			}
		}
		return fmt.Errorf("errors occurred while closing tarball writer: %w", combinedErr)
	}

	return nil
}

// addFileToTarball adds a single file to the tarball
func (tw *tarballWriter) addFileToTarball(filePath, tarballPath string) error {
	// Clean and validate the file path
	cleanPath := filepath.Clean(filePath)
	if !filepath.IsAbs(cleanPath) {
		return fmt.Errorf("path must be absolute: %s", filePath)
	}

	// Open the source file with read-only permissions
	file, err := os.Open(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", cleanPath, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log the error but don't mask the original error if any
			logger.Warnf("Failed to close file %s: %v", cleanPath, closeErr)
		}
	}()

	// Get file info to include in the tarball header
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info for %s: %w", cleanPath, err)
	}

	// Skip special files (sockets, devices, etc.)
	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("skipping non-regular file: %s", cleanPath)
	}

	// Create a new tar header with file metadata
	header, err := tar.FileInfoHeader(fileInfo, "")
	if err != nil {
		return fmt.Errorf("failed to create tar header for %s: %w", cleanPath, err)
	}

	// Ensure the header name uses forward slashes and is relative to the tarball root
	header.Name = filepath.ToSlash(tarballPath)

	// Write the header to the tarball
	if err := tw.tar.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header for %s: %w", cleanPath, err)
	}

	// Only copy file content for regular files with size > 0
	if fileInfo.Size() > 0 {
		// Copy the file content to the tarball
		if _, err := io.CopyN(tw.tar, file, fileInfo.Size()); err != nil {
			return fmt.Errorf("failed to write file content to tarball for %s: %w", cleanPath, err)
		}
	}

	return nil
}

// processDirectory walks through a directory and adds its contents to the tarball
func (tw *tarballWriter) processDirectory(dirPath, tarballBase string) error {
	// Clean and validate the directory path
	cleanDirPath := filepath.Clean(dirPath)
	if !filepath.IsAbs(cleanDirPath) {
		return fmt.Errorf("directory path must be absolute: %s", dirPath)
	}

	// Verify the directory exists and is accessible
	dirInfo, err := os.Stat(cleanDirPath)
	if err != nil {
		return fmt.Errorf("failed to access directory %s: %w", cleanDirPath, err)
	}
	if !dirInfo.IsDir() {
		return fmt.Errorf("path is not a directory: %s", cleanDirPath)
	}

	// Read the directory contents
	entries, err := os.ReadDir(cleanDirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", cleanDirPath, err)
	}

	// Process each entry in the directory
	for _, entry := range entries {
		// Skip special files and hidden files
		if entry.Name() == "." || entry.Name() == ".." || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		// Get the full path of the entry
		fullPath := filepath.Join(cleanDirPath, entry.Name())
		tarballPath := filepath.Join(tarballBase, entry.Name())

		// Get file info for better error messages
		fileInfo, err := entry.Info()
		if err != nil {
			return fmt.Errorf("failed to get file info for %s: %w", fullPath, err)
		}

		switch {
		case fileInfo.IsDir():
			// Recursively process subdirectories
			if err := tw.processDirectory(fullPath, tarballPath); err != nil {
				return fmt.Errorf("error processing directory %s: %w", fullPath, err)
			}
		case fileInfo.Mode().IsRegular():
			// Add regular files to the tarball
			if err := tw.addFileToTarball(fullPath, tarballPath); err != nil {
				return fmt.Errorf("error adding file %s to tarball: %w", fullPath, err)
			}
		default:
			// Skip special files (sockets, devices, etc.) but log a warning
			logger.Warnf("Skipping non-regular file: %s (mode: %s)", fullPath, fileInfo.Mode())
		}
	}

	return nil
}

// createTarball creates a tarball from the source directory
func createTarball(sourceDir, outputPath string, meta *Metadata) error {
	// Clean and validate source directory path
	sourceDir = filepath.Clean(sourceDir)
	if !filepath.IsAbs(sourceDir) {
		return fmt.Errorf("source directory path must be absolute: %s", sourceDir)
	}

	// Verify source directory exists and is accessible
	sourceInfo, err := os.Stat(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to access source directory %s: %w", sourceDir, err)
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("source path is not a directory: %s", sourceDir)
	}

	// Initialize tarball writer with error wrapping
	tw, err := newTarballWriter(outputPath)
	if err != nil {
		return fmt.Errorf("failed to initialize tarball writer for %s: %w", outputPath, err)
	}

	// Track any errors that occur during deferred cleanup
	var returnErr error

	// Ensure the tarball writer is properly closed
	defer func() {
		if closeErr := tw.close(); closeErr != nil {
			if returnErr == nil {
				returnErr = fmt.Errorf("error closing tarball writer: %w", closeErr)
			} else {
				// If we already have an error, combine it with the close error
				returnErr = fmt.Errorf("%v (additionally, error closing tarball: %v)", returnErr, closeErr)
			}
		}
	}()

	// Process the source directory
	if err := tw.processDirectory(sourceDir, "."); err != nil {
		return fmt.Errorf("failed to process source directory %s: %w", sourceDir, err)
	}

	// Create a temporary file for the metadata
	tmpFile, err := os.CreateTemp("", "gotya-metadata-*.json")
	if err != nil {
		return fmt.Errorf("failed to create temporary file for metadata: %w", err)
	}
	tmpFileName := tmpFile.Name()

	// Clean up the temporary file when we're done
	defer func() {
		if removeErr := os.Remove(tmpFileName); removeErr != nil && !os.IsNotExist(removeErr) {
			logger.Warnf("Failed to remove temporary file %s: %v", tmpFileName, removeErr)
		}
	}()

	// Write metadata to the temporary file
	metaJSON, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if _, err := tmpFile.Write(metaJSON); err != nil {
		return fmt.Errorf("failed to write metadata to temporary file %s: %w", tmpFileName, err)
	}

	// Ensure the file is flushed to disk
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync metadata to disk: %w", err)
	}

	// Close the file before adding it to the tarball
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary metadata file %s: %w", tmpFileName, err)
	}

	// Add the metadata file to the tarball
	if err := tw.addFileToTarball(tmpFileName, "pkg.json"); err != nil {
		return fmt.Errorf("failed to add metadata to tarball: %w", err)
	}

	// Explicitly close the tarball writer to ensure all data is flushed
	if err := tw.close(); err != nil {
		return fmt.Errorf("error finalizing tarball: %w", err)
	}

	return returnErr
}

// CreatePackage creates a new package from the source directory
func CreatePackage(sourceDir, outputDir, pkgName, pkgVer, pkgOS, pkgArch string) error {
	// Validate input parameters
	if sourceDir == "" {
		return errors.New("source directory path cannot be empty")
	}
	if outputDir == "" {
		return errors.New("output directory path cannot be empty")
	}
	if pkgName == "" {
		return errors.New("package name cannot be empty")
	}
	if pkgVer == "" {
		return errors.New("package version cannot be empty")
	}
	if pkgOS == "" {
		return errors.New("target OS cannot be empty")
	}
	if pkgArch == "" {
		return errors.New("target architecture cannot be empty")
	}

	// Normalize and clean paths
	sourceDir = filepath.Clean(sourceDir)
	outputDir = filepath.Clean(outputDir)

	// Ensure source directory exists and is accessible
	sourceInfo, err := os.Stat(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to access source directory %s: %w", sourceDir, err)
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("source path is not a directory: %s", sourceDir)
	}

	// Ensure output directory exists and is writable
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	// Verify output directory is writable
	if err := verifyDirectoryWritable(outputDir); err != nil {
		return fmt.Errorf("output directory %s is not writable: %w", outputDir, err)
	}

	// Create package metadata with validation
	meta := &Metadata{
		Name:         pkgName,
		Version:      pkgVer,
		Maintainer:   "",
		Description:  "",
		Dependencies: []string{},
		Files:        []File{},
		Hooks:        map[string]string{},
	}

	// Process files and update metadata
	if err := processFiles(sourceDir, meta); err != nil {
		return fmt.Errorf("failed to process files in %s: %w", sourceDir, err)
	}

	// Ensure we have files to package
	if len(meta.Files) == 0 {
		return fmt.Errorf("no files found to package in %s", sourceDir)
	}

	// Create package filename with validation
	if strings.ContainsAny(pkgName, "/\\") {
		return fmt.Errorf("invalid package name '%s': cannot contain path separators", pkgName)
	}

	// Sanitize version string
	versionRegex := regexp.MustCompile(`^[a-zA-Z0-9_.+-]+$`)
	if !versionRegex.MatchString(pkgVer) {
		return fmt.Errorf("invalid version string '%s': must contain only alphanumeric characters, dots, underscores, plus, and hyphens", pkgVer)
	}

	// Use the expected format: {name}_{version}_{os}_{arch}.tar.gz
	outputFile := filepath.Join(outputDir, fmt.Sprintf("%s_%s_%s_%s.tar.gz", pkgName, pkgVer, pkgOS, pkgArch))

	// Check if output file already exists
	if _, err := os.Stat(outputFile); err == nil {
		return fmt.Errorf("output file already exists: %s", outputFile)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check if output file exists: %w", err)
	}

	// Create the tarball with proper error handling
	if err := createTarball(sourceDir, outputFile, meta); err != nil {
		// Clean up partially created file on error
		if removeErr := os.Remove(outputFile); removeErr != nil && !os.IsNotExist(removeErr) {
			logger.Warnf("Failed to clean up partially created package %s: %v", outputFile, removeErr)
		}
		return fmt.Errorf("failed to create package: %w", err)
	}

	// Verify the created package
	if err := verifyPackage(outputFile, meta); err != nil {
		// Clean up invalid package
		if removeErr := os.Remove(outputFile); removeErr != nil && !os.IsNotExist(removeErr) {
			logger.Warnf("Failed to clean up invalid package %s: %v", outputFile, removeErr)
		}
		return fmt.Errorf("package verification failed: %w", err)
	}

	return nil
}

// verifyDirectoryWritable checks if a directory is writable by attempting to create and remove a test file
func verifyDirectoryWritable(dirPath string) error {
	testFile := filepath.Join(dirPath, ".gotya-test-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	file, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("cannot write to directory: %w", err)
	}

	// Try to write and sync to ensure we have write permissions
	if _, err := file.WriteString("test"); err != nil {
		file.Close()
		return fmt.Errorf("cannot write to file in directory: %w", err)
	}

	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("cannot sync file in directory: %w", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("cannot close test file: %w", err)
	}

	// Clean up the test file
	if err := os.Remove(testFile); err != nil {
		return fmt.Errorf("cannot remove test file: %w", err)
	}

	return nil
}

// verifyPackage performs basic verification of the created package
func verifyPackage(pkgPath string, expectedMeta *Metadata) error {
	// Open the package file
	file, err := os.Open(pkgPath)
	if err != nil {
		return fmt.Errorf("failed to open package for verification: %w", err)
	}
	defer file.Close()

	// Get file info for size check
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get package file info: %w", err)
	}

	// Check if the file is empty or too small to be valid
	if fileInfo.Size() < 50 { // Minimum size for a valid gzip file
		return errors.New("package file is too small to be valid")
	}

	// Verify the file is a valid gzip file
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("invalid gzip format: %w", err)
	}
	defer gzipReader.Close()

	// Check for the presence of required files in the tarball
	tarReader := tar.NewReader(gzipReader)
	foundMetadata := false

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tarball: %w", err)
		}

		// Check for the metadata file
		if header.Name == "pkg.json" {
			foundMetadata = true

			// Read and verify the metadata
			var meta Metadata
			if err := json.NewDecoder(tarReader).Decode(&meta); err != nil {
				return fmt.Errorf("invalid metadata in package: %w", err)
			}

			// Verify metadata matches expected values
			if meta.Name != expectedMeta.Name || meta.Version != expectedMeta.Version {
				return fmt.Errorf("metadata mismatch in package, expected %s-%s, got %s-%s",
					expectedMeta.Name, expectedMeta.Version, meta.Name, meta.Version)
			}
		}

		// Skip to the next file in the tarball
		if _, err := io.Copy(io.Discard, tarReader); err != nil {
			return fmt.Errorf("error reading file %s from tarball: %w", header.Name, err)
		}
	}

	if !foundMetadata {
		return errors.New("package is missing required metadata (pkg.json)")
	}

	// Log successful package verification
	logger.Info("Package verified successfully", map[string]interface{}{
		"path":  pkgPath,
		"name":  expectedMeta.Name,
		"files": len(expectedMeta.Files),
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
