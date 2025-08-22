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
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cperrin88/gotya/pkg/logger"
)

// Common validation patterns.
var (
	packageNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	versionRegex     = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.+_-]*$`)
)

// in the package meta directory.
type Metadata struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Maintainer   string            `json:"maintainer,omitempty"`
	Description  string            `json:"description"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Files        []File            `json:"files,omitempty"`
	Hooks        map[string]string `json:"hooks,omitempty"`
}

// File represents a file entry in the package metadata.
type File struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Mode   uint32 `json:"mode"`
	Digest string `json:"digest"`
}

// safePathJoin joins path elements and ensures the result is within the base directory.
func safePathJoin(baseDir string, elems ...string) (string, error) {
	// Clean and join all path elements
	path := filepath.Join(append([]string{baseDir}, elems...)...)

	// Clean the path to remove any .. or .
	cleanPath := filepath.Clean(path)

	// Verify the final path is still within the base directory
	relPath, err := filepath.Rel(baseDir, cleanPath)
	if err != nil || strings.HasPrefix(relPath, "..") || strings.HasPrefix(relPath, ".") {
		return "", NewPathTraversalError(path)
	}

	return cleanPath, nil
}

// validatePath checks if a path is safe and valid
func validatePath(path string) (string, error) {
	// Clean the path first to handle any . or ..
	cleanPath := filepath.Clean(path)

	// Check for path traversal attempts
	if strings.HasPrefix(cleanPath, "..") || strings.Contains(cleanPath, "\\..") ||
		strings.Contains(cleanPath, "/..") || (len(cleanPath) >= 1 && cleanPath[0] == '/') {
		return "", NewPathTraversalError(path)
	}

	// For Windows, check for drive-relative paths
	if len(cleanPath) >= 2 && cleanPath[1] == ':' {
		// This is a Windows absolute path (e.g., C:\\path)
		if !filepath.IsAbs(cleanPath) {
			return "", NewPathTraversalError(path)
		}
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", NewFileOperationError("get absolute path", path, err)
	}

	// Additional check to prevent path traversal using symlinks
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", NewFileOperationError("resolve symlinks", absPath, err)
		}
		// If the file doesn't exist, we can't resolve symlinks, but that's okay for our purposes
		return absPath, nil
	}

	// Verify the resolved path is still within the expected directory structure
	if !filepath.IsAbs(resolvedPath) {
		resolvedPath, err = filepath.Abs(resolvedPath)
		if err != nil {
			return "", NewFileOperationError("get absolute path for resolved path", resolvedPath, err)
		}
	}

	// For extra safety, check if the resolved path is still under the same directory
	// as the original path (or a parent directory)
	rel, err := filepath.Rel(filepath.Dir(absPath), resolvedPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", NewPathTraversalError(fmt.Sprintf("resolved path %s is outside of base directory", resolvedPath))
	}

	return absPath, nil
}

// calculateFileHash calculates the SHA256 hash of a file.
func calculateFileHash(path string) (string, error) {
	// Validate the path
	absPath, err := validatePath(path)
	if err != nil {
		return "", err
	}

	file, err := os.Open(absPath)
	if err != nil {
		return "", NewFileOperationError("open file", path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log the error but don't mask the original error if any
			logger.Warnf("Failed to close file %s: %v", absPath, closeErr)
		}
	}()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", NewHashCalculationError(path, err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// processFiles processes all files in the source directory and updates the metadata.
func processFiles(sourceDir string, meta *Metadata) error {
	// Clear existing files
	meta.Files = []File{}

	// First, check if we have a 'files' subdirectory
	filesDir := filepath.Join(sourceDir, "files")
	if _, err := os.Stat(filesDir); err == nil {
		// Process files from the 'files' subdirectory
		err := filepath.Walk(filesDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return NewFileOperationError("walk path", path, err)
			}

			// Skip the files directory itself
			if path == filesDir {
				return nil
			}

			// Skip directories
			if info.IsDir() {
				return nil
			}

			// Get relative path from the files directory
			relPath, err := filepath.Rel(filesDir, path)
			if err != nil {
				return NewFileOperationError("get relative path", path, err)
			}

			// Skip hidden files and directories
			if strings.HasPrefix(filepath.Base(relPath), ".") {
				return nil
			}

			// Calculate file hash
			hash, err := calculateFileHash(path)
			if err != nil {
				return fmt.Errorf("%w: %s", err, path)
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

		if err != nil {
			return fmt.Errorf("error processing files directory: %w", err)
		}

		// If we found files in the 'files' directory, we're done
		if len(meta.Files) > 0 {
			return nil
		}
	}

	// If no 'files' directory or it was empty, process the source directory directly
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return NewFileOperationError("walk path", path, err)
		}

		// Skip the source directory itself
		if path == sourceDir {
			return nil
		}

		// Skip the 'meta' directory and its contents
		if strings.HasPrefix(path, filepath.Join(sourceDir, "meta")) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip the 'files' directory (we already processed it)
		if strings.HasPrefix(path, filepath.Join(sourceDir, "files")) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip hidden files and directories
		if strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return NewFileOperationError("get relative path", path, err)
		}

		// Calculate file hash
		hash, err := calculateFileHash(path)
		if err != nil {
			return fmt.Errorf("%w: %s", err, path)
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

	if err != nil {
		return fmt.Errorf("error processing source directory: %w", err)
	}

	return err
}

// tarballWriter wraps the writers needed for creating a tarball.
type tarballWriter struct {
	file      *os.File
	bufWriter *bufio.Writer
	gzip      *gzip.Writer
	tar       *tar.Writer
}

// newTarballWriter creates and initializes a new tarballWriter.
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

// close closes all writers in the correct order and returns any errors.
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
				combinedErr = fmt.Errorf("%w; %w", combinedErr, err)
			}
		}
		return fmt.Errorf("errors occurred while closing tarball writer: %w", combinedErr)
	}

	return nil
}

// addFileToTarball adds a single file to the tarball.
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

// processDirectory walks through a directory and adds its contents to the tarball.
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

func createTarball(sourceDir, outputPath string, meta *Metadata) error {
	// Clean and validate source directory path
	sourceDir = filepath.Clean(sourceDir)
	if !filepath.IsAbs(sourceDir) {
		return ErrInvalidSourceDirectory
	}

	// Verify source directory exists and is accessible
	sourceInfo, err := os.Stat(sourceDir)
	if err != nil {
		return ErrDirectoryStatFailed
	}
	if !sourceInfo.IsDir() {
		return ErrNotADirectory
	}

	// Initialize tarball writer with error wrapping
	tw, err := newTarballWriter(outputPath)
	if err != nil {
		return NewFileOperationError("initialize tarball writer", outputPath, err)
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
				returnErr = fmt.Errorf("%w (additionally, error closing tarball: %w)", returnErr, closeErr)
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

// CreatePackage creates a new package from the source directory.
func CreatePackage(sourceDir, outputDir, pkgName, pkgVer, pkgOS, pkgArch string) error {
	// Validate input parameters
	if sourceDir == "" {
		return ErrSourceDirEmpty
	}
	if outputDir == "" {
		return ErrOutputDirEmpty
	}
	if pkgName == "" {
		return ErrPackageNameEmpty
	}
	if pkgVer == "" {
		return ErrPackageVersionEmpty
	}
	if pkgOS == "" {
		return ErrTargetOSEmpty
	}
	if pkgArch == "" {
		return ErrTargetArchEmpty
	}

	// Normalize and clean paths
	sourceDir = filepath.Clean(sourceDir)
	outputDir = filepath.Clean(outputDir)

	// Ensure source directory exists and is accessible
	sourceInfo, err := os.Stat(sourceDir)
	if err != nil {
		return NewFileOperationError("access", sourceDir, err)
	}

	if !sourceInfo.IsDir() {
		return NewFileOperationError("check directory", sourceDir, fmt.Errorf("not a directory"))
	}

	// Ensure output directory exists and is writable
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return NewFileOperationError("create directory", outputDir, err)
	}

	// Verify we can write to the output directory
	if err := verifyDirectoryWritable(outputDir); err != nil {
		return fmt.Errorf("%w: %s", ErrDirectoryNotWritable, outputDir)
	}

	// Check if there are any files to package
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return NewFileOperationError("read directory", sourceDir, err)
	}

	if len(entries) == 0 {
		return fmt.Errorf("%w: %s", ErrNoFilesFound, sourceDir)
	}

	// Validate package name and version
	if strings.ContainsAny(pkgName, "/\\") || !packageNameRegex.MatchString(pkgName) {
		return fmt.Errorf("%w: %s", ErrInvalidPackageName, pkgName)
	}

	if !versionRegex.MatchString(pkgVer) {
		return fmt.Errorf("%w: %s", ErrInvalidVersionString, pkgVer)
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
		return fmt.Errorf("%w: %s", ErrNoFilesFound, sourceDir)
	}

	// Create output filename
	outputFile := filepath.Join(outputDir, fmt.Sprintf("%s_%s_%s_%s.tar.gz", pkgName, pkgVer, pkgOS, pkgArch))

	// Check if output file already exists
	if _, err := os.Stat(outputFile); err == nil {
		return fmt.Errorf("%w: %s", ErrOutputFileExists, outputFile)
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

// verifyDirectoryWritable checks if a directory is writable by attempting to create and remove a test file.
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

// verifyPackage verifies the integrity of a package file.
func verifyPackage(pkgPath string, expectedMeta *Metadata) error {
	// Check if package file exists and has a reasonable size
	fileInfo, err := os.Stat(pkgPath)
	if err != nil {
		return NewFileOperationError("access", pkgPath, err)
	}

	// Check minimum size (smallest possible gzip file is ~20 bytes)
	const minPkgSize = 50 // Minimum size for a valid gzip file
	if fileInfo.Size() < minPkgSize {
		return ErrPackageTooSmall
	}

	// Open the package file
	file, err := os.Open(pkgPath)
	if err != nil {
		return NewFileOperationError("open", pkgPath, err)
	}
	defer file.Close()

	// Create a gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return NewFileOperationError("read gzip", pkgPath, err)
	}
	defer gzReader.Close()

	// Create a tar reader
	tarReader := tar.NewReader(gzReader)

	// Look for the metadata file in the tarball
	var metaDataFound bool
	var meta Metadata

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return NewFileOperationError("read tarball", pkgPath, err)
		}

		// We're only interested in the metadata file
		if header.Name == "pkg.json" {
			// Decode the metadata
			if err := json.NewDecoder(tarReader).Decode(&meta); err != nil {
				return NewFileOperationError("decode metadata", pkgPath, err)
			}
			metaDataFound = true
			break
		}
	}

	if !metaDataFound {
		return ErrMetadataMissing
	}

	// Log successful package verification
	logger.Info("Package verified successfully", map[string]interface{}{
		"path":    pkgPath,
		"name":    meta.Name,
		"version": meta.Version,
	})

	// Verify package name and version if expected metadata is provided
	if expectedMeta != nil {
		if meta.Name != expectedMeta.Name || meta.Version != expectedMeta.Version {
			return NewPackageVerificationError(
				expectedMeta.Name,
				expectedMeta.Version,
				meta.Name,
				meta.Version,
			)
		}

		// Verify files if expected metadata contains files
		if len(expectedMeta.Files) > 0 {
			// Reset file pointer to start of tarball
			if _, err := file.Seek(0, 0); err != nil {
				return NewFileOperationError("seek file", pkgPath, err)
			}

			// Recreate gzip and tar readers after seek
			if err := gzReader.Reset(file); err != nil {
				return NewFileOperationError("reset gzip reader", pkgPath, err)
			}
			tarReader = tar.NewReader(gzReader)

			// Track which files we've found
			foundFiles := make(map[string]bool)

			// Process all files in the tarball
			for {
				header, err := tarReader.Next()
				if err == io.EOF {
					break // End of archive
				}
				if err != nil {
					return NewFileOperationError("read tarball entry", pkgPath, err)
				}

				// Skip directories and special files
				if header.Typeflag != tar.TypeReg {
					continue
				}

				// Mark file as found
				foundFiles[header.Name] = true

				// Verify file size and permissions if specified in metadata
				for _, expectedFile := range expectedMeta.Files {
					if expectedFile.Path == header.Name {
						if expectedFile.Size > 0 && header.Size != expectedFile.Size {
							return NewFileOperationError("verify file size", header.Name,
								fmt.Errorf("size mismatch: expected %d, got %d", expectedFile.Size, header.Size))
						}

						if expectedFile.Mode > 0 && header.Mode != int64(expectedFile.Mode) {
							return NewFileOperationError("verify file mode", header.Name,
								fmt.Errorf("permissions mismatch: expected %o, got %o", expectedFile.Mode, header.Mode))
						}

						// Verify file hash if specified
						if expectedFile.Digest != "" {
							hash := sha256.New()
							if _, err := io.Copy(hash, tarReader); err != nil {
								return NewFileOperationError("calculate file hash", header.Name, err)
							}
							actualDigest := hex.EncodeToString(hash.Sum(nil))
							if actualDigest != expectedFile.Digest {
								return NewFileOperationError("verify file hash", header.Name,
									fmt.Errorf("hash mismatch: expected %s, got %s", expectedFile.Digest, actualDigest))
							}
						}
					}
				}
			}

			// Check for missing files
			for _, expectedFile := range expectedMeta.Files {
				// Check both with and without 'files/' prefix for backward compatibility
				found := foundFiles[expectedFile.Path] ||
					strings.HasPrefix(expectedFile.Path, "files/") && foundFiles[strings.TrimPrefix(expectedFile.Path, "files/")] ||
					!strings.HasPrefix(expectedFile.Path, "files/") && foundFiles["files/"+expectedFile.Path]

				if !found {
					return NewFileOperationError("verify file exists", expectedFile.Path,
						fmt.Errorf("file not found in package"))
				}
			}
		}
	}

	// If we got here, verification was successful
	return nil
}

// readPackageMetadata reads and parses package metadata from a JSON file.
func readPackageMetadata(metadataPath string) (*Metadata, error) {
	// Open the metadata file
	file, err := os.Open(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrMetadataFileNotFound
		}
		return nil, NewFileOperationError("open metadata file", metadataPath, err)
	}
	defer file.Close()

	// Read the file content
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, NewFileOperationError("read metadata file", metadataPath, err)
	}

	// Parse the JSON data
	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Validate required fields
	if meta.Name == "" {
		return nil, ErrNameRequired
	}

	if meta.Version == "" {
		return nil, ErrVersionRequired
	}

	if meta.Description == "" {
		return nil, ErrDescriptionRequired
	}

	return &meta, nil
}
