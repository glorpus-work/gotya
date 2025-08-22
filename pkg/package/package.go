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

	"github.com/cperrin88/gotya/pkg/errors"
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
	// Join path elements
	pathElems := append([]string{baseDir}, elems...)
	fullPath := filepath.Join(pathElems...)

	// Clean the path to remove any '..' or '.'
	cleanPath := filepath.Clean(fullPath)

	// Verify the final path is still within the base directory
	rel, err := filepath.Rel(baseDir, cleanPath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get relative path for %s", cleanPath)
	}

	if strings.HasPrefix(rel, "..") || (len(rel) >= 1 && rel[0] == '.') {
		return "", errors.Wrapf(errors.ErrInvalidPath, "path %s is outside base directory %s", cleanPath, baseDir)
	}

	return cleanPath, nil
}

// validatePath validates that a path is absolute and exists.
func validatePath(path string) (string, error) {
	if path == "" {
		return "", errors.Wrap(errors.ErrInvalidPath, "path cannot be empty")
	}

	// Clean the path
	cleanPath := filepath.Clean(path)

	// Check if the path is absolute
	if !filepath.IsAbs(cleanPath) {
		return "", errors.Wrapf(errors.ErrInvalidPath, "path must be absolute: %s", cleanPath)
	}

	// Check if the path exists
	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		return "", errors.Wrapf(errors.ErrFileNotFound, "path does not exist: %s", cleanPath)
	} else if err != nil {
		return "", errors.Wrapf(err, "failed to access path: %s", cleanPath)
	}

	return cleanPath, nil
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
		return "", errors.Wrapf(err, "failed to open file %s", absPath)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log the error but don't mask the original error if any
			logger.Warnf("Failed to close file %s: %v", absPath, closeErr)
		}
	}()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", errors.Wrapf(err, "failed to calculate hash for file %s", absPath)
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
				return errors.Wrapf(err, "failed to walk path %s", path)
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
				return errors.Wrapf(err, "failed to get relative path for %s", path)
			}

			// Skip hidden files and directories
			if strings.HasPrefix(filepath.Base(relPath), ".") {
				return nil
			}

			// Calculate file hash
			hash, err := calculateFileHash(path)
			if err != nil {
				return errors.Wrapf(err, "failed to calculate hash for file %s", path)
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
			return errors.Wrapf(err, "failed to process files directory")
		}

		// If we found files in the 'files' directory, we're done
		if len(meta.Files) > 0 {
			return nil
		}
	}

	// If no 'files' directory or it was empty, process the source directory directly
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrapf(err, "failed to walk path %s", path)
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
			return errors.Wrapf(err, "failed to get relative path for %s", path)
		}

		// Calculate file hash
		hash, err := calculateFileHash(path)
		if err != nil {
			return errors.Wrapf(err, "failed to calculate hash for file %s", path)
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
		return errors.Wrapf(err, "failed to process source directory")
	}

	return nil
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
		return nil, errors.Wrapf(err, "failed to create output file %s", outputPath)
	}

	// Create the output file
	file, err := os.Create(cleanPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create output file %s", cleanPath)
	}

	// Create buffered writer
	bufWriter := bufio.NewWriter(file)

	// Create gzip writer with best compression
	gzWriter, err := gzip.NewWriterLevel(bufWriter, gzip.BestCompression)
	if err != nil {
		file.Close()
		return nil, errors.Wrapf(err, "failed to create gzip writer")
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
		errs = append(errs, errors.Wrapf(err, "error closing tar writer"))
	}

	// Close gzip writer to flush any remaining compressed data
	if err := tw.gzip.Close(); err != nil {
		errs = append(errs, errors.Wrapf(err, "error closing gzip writer"))
	}

	// Flush the buffer to ensure all data is written to the file
	if err := tw.bufWriter.Flush(); err != nil {
		errs = append(errs, errors.Wrapf(err, "error flushing buffer"))
	}

	// Finally, close the file
	if err := tw.file.Close(); err != nil {
		errs = append(errs, errors.Wrapf(err, "error closing file %s", tw.file.Name()))
	}

	// Return combined errors if any occurred
	if len(errs) > 0 {
		var combinedErr error
		for _, err := range errs {
			if combinedErr == nil {
				combinedErr = err
			} else {
				// If we already have an error, combine it with the close error
				combinedErr = errors.Wrapf(combinedErr, "additionally: %v", err)
			}
		}
		return combinedErr
	}

	return nil
}

// addFileToTarball adds a single file to the tarball.
func (tw *tarballWriter) addFileToTarball(filePath, tarballPath string) error {
	// Clean and validate the file path
	cleanPath := filepath.Clean(filePath)
	if !filepath.IsAbs(cleanPath) {
		return errors.Wrapf(errors.ErrInvalidPath, "path must be absolute: %s", cleanPath)
	}

	// Open the source file with read-only permissions
	file, err := os.Open(cleanPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open source file %s", cleanPath)
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
		return errors.Wrapf(err, "failed to get file info for %s", cleanPath)
	}

	// Skip special files (sockets, devices, etc.)
	if !fileInfo.Mode().IsRegular() {
		return errors.Wrapf(errors.ErrInvalidFile, "skipping non-regular file: %s", cleanPath)
	}

	// Create a new tar header with file metadata
	header, err := tar.FileInfoHeader(fileInfo, "")
	if err != nil {
		return errors.Wrapf(err, "failed to create tar header for %s", cleanPath)
	}

	// Ensure the header name uses forward slashes and is relative to the tarball root
	header.Name = filepath.ToSlash(tarballPath)

	// Write the header to the tarball
	if err := tw.tar.WriteHeader(header); err != nil {
		return errors.Wrapf(err, "failed to write tar header for %s", cleanPath)
	}

	// Only copy file content for regular files with size > 0
	if fileInfo.Size() > 0 {
		// Copy the file content to the tarball
		if _, err := io.CopyN(tw.tar, file, fileInfo.Size()); err != nil {
			return errors.Wrapf(err, "failed to write file contents for %s", cleanPath)
		}
	}

	return nil
}

// processDirectory walks through a directory and adds its contents to the tarball.
func (tw *tarballWriter) processDirectory(dirPath, tarballBase string) error {
	// Clean and validate the directory path
	cleanDirPath, err := validatePath(dirPath)
	if err != nil {
		return errors.Wrapf(err, "invalid directory path")
	}

	// Ensure the directory exists and is actually a directory
	fileInfo, err := os.Stat(cleanDirPath)
	if err != nil {
		return errors.Wrapf(err, "failed to stat directory %s", cleanDirPath)
	}
	if !fileInfo.IsDir() {
		return errors.Wrapf(errors.ErrInvalidPath, "path is not a directory: %s", cleanDirPath)
	}

	// Create a directory entry in the tarball
	header := &tar.Header{
		Name:     filepath.ToSlash(tarballBase) + "/",
		Mode:     int64(fileInfo.Mode()),
		ModTime:  fileInfo.ModTime(),
		Typeflag: tar.TypeDir,
	}

	if err := tw.tar.WriteHeader(header); err != nil {
		return errors.Wrapf(err, "failed to write directory header for %s", cleanDirPath)
	}

	// Read the directory contents
	entries, err := os.ReadDir(cleanDirPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read directory %s", cleanDirPath)
	}

	// Process each entry in the directory
	for _, entry := range entries {
		entryPath := filepath.Join(cleanDirPath, entry.Name())
		tarballPath := filepath.Join(tarballBase, entry.Name())

		if entry.IsDir() {
			// Recursively process subdirectories
			if err := tw.processDirectory(entryPath, tarballPath); err != nil {
				return errors.Wrapf(err, "failed to process directory %s", entryPath)
			}
		} else {
			// Add regular files to the tarball
			if err := tw.addFileToTarball(entryPath, tarballPath); err != nil {
				return errors.Wrapf(err, "failed to add file %s", entryPath)
			}
			// Skip special files (sockets, devices, etc.) but log a warning
			logger.Warnf("Skipping non-regular file: %s (mode: %s)", entryPath, fileInfo.Mode())
		}
	}

	return nil
}

func createTarball(sourceDir, outputPath string, meta *Metadata) error {
	// Clean and validate source directory path
	sourceDir, err := validatePath(sourceDir)
	if err != nil {
		return errors.Wrapf(err, "invalid source directory")
	}

	// Ensure the source directory exists and is readable
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return errors.Wrapf(errors.ErrFileNotFound, "source directory does not exist: %s", sourceDir)
	} else if err != nil {
		return errors.Wrapf(err, "failed to access source directory %s", sourceDir)
	}

	// Create tarball writer
	tw, err := newTarballWriter(outputPath)
	if err != nil {
		return errors.Wrapf(err, "failed to create tarball writer")
	}
	defer func() {
		if closeErr := tw.close(); closeErr != nil {
			logger.Warnf("Error closing tarball writer: %v", closeErr)
		}
	}()

	// Create meta directory in the tarball
	metaDir := "meta"
	if err := tw.processDirectory(filepath.Join(sourceDir, metaDir), metaDir); err != nil {
		return errors.Wrapf(err, "failed to add meta directory to tarball")
	}

	// Add files to the tarball
	filesDir := filepath.Join(sourceDir, "files")
	if _, err := os.Stat(filesDir); err == nil {
		// If files directory exists, add its contents to the root of the tarball
		if err := tw.processDirectory(filesDir, "."); err != nil {
			return errors.Wrapf(err, "failed to add files directory to tarball")
		}
	} else if !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to check files directory")
	} else {
		// If no files directory, add all files from source directory except meta
		entries, err := os.ReadDir(sourceDir)
		if err != nil {
			return errors.Wrapf(err, "failed to read source directory")
		}

		for _, entry := range entries {
			entryPath := filepath.Join(sourceDir, entry.Name())

			// Skip the meta directory as it's already been processed
			if entry.Name() == "meta" || entry.Name() == ".git" || entry.Name() == ".gitignore" {
				continue
			}

			if entry.IsDir() {
				if err := tw.processDirectory(entryPath, entry.Name()); err != nil {
					return errors.Wrapf(err, "failed to add directory %s to tarball", entry.Name())
				}
			} else {
				if err := tw.addFileToTarball(entryPath, entry.Name()); err != nil {
					return errors.Wrapf(err, "failed to add file %s to tarball", entry.Name())
				}
			}
		}
	}

	return nil
}

// CreatePackage creates a new package from the source directory.
func CreatePackage(sourceDir, outputDir, pkgName, pkgVer, pkgOS, pkgArch string) error {
	// Validate package name
	if pkgName == "" {
		return errors.Wrap(errors.ErrValidation, "package name cannot be empty")
	}
	if !packageNameRegex.MatchString(pkgName) {
		return errors.Wrapf(errors.ErrValidation, "invalid package name: %s - must match %s", pkgName, packageNameRegex.String())
	}

	// Validate package version
	if pkgVer == "" {
		return errors.Wrap(errors.ErrValidation, "package version cannot be empty")
	}
	if !versionRegex.MatchString(pkgVer) {
		return errors.Wrapf(errors.ErrValidation, "invalid package version: %s - must match %s", pkgVer, versionRegex.String())
	}

	// Validate OS and architecture
	if pkgOS == "" {
		return errors.Wrap(errors.ErrValidation, "OS cannot be empty")
	}
	if pkgArch == "" {
		return errors.Wrap(errors.ErrValidation, "architecture cannot be empty")
	}

	// Clean and validate source directory path
	sourceDir, err := validatePath(sourceDir)
	if err != nil {
		return errors.Wrapf(err, "invalid source directory")
	}

	// Ensure source directory exists and is readable
	if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
		return errors.Wrapf(errors.ErrFileNotFound, "source directory does not exist: %s", sourceDir)
	} else if err != nil {
		return errors.Wrapf(err, "failed to access source directory %s", sourceDir)
	}

	// Clean and validate output directory path
	outputDir, err = validatePath(outputDir)
	if err != nil {
		return errors.Wrapf(err, "invalid output directory")
	}

	// Ensure output directory exists and is writable
	if err := verifyDirectoryWritable(outputDir); err != nil {
		return errors.Wrapf(err, "output directory %s is not writable", outputDir)
	}

	// Create package metadata
	meta := &Metadata{
		Name:        pkgName,
		Version:     pkgVer,
		Description: fmt.Sprintf("Package %s version %s", pkgName, pkgVer),
	}

	// Process files and update metadata
	if err := processFiles(sourceDir, meta); err != nil {
		return errors.Wrapf(err, "failed to process files in %s", sourceDir)
	}

	// Create the output filename
	outputFile := filepath.Join(outputDir, fmt.Sprintf("%s_%s_%s_%s.tar.gz", pkgName, pkgVer, pkgOS, pkgArch))

	// Create the tarball
	if err := createTarball(sourceDir, outputFile, meta); err != nil {
		// Clean up the output file if it was partially created
		if removeErr := os.Remove(outputFile); removeErr != nil && !os.IsNotExist(removeErr) {
			logger.Warnf("Failed to clean up partially created package file %s: %v", outputFile, removeErr)
		}
		return errors.Wrapf(err, "failed to create package %s", outputFile)
	}

	// Verify the created package
	if err := verifyPackage(outputFile, meta); err != nil {
		// Clean up the output file if verification failed
		if removeErr := os.Remove(outputFile); removeErr != nil && !os.IsNotExist(removeErr) {
			logger.Warnf("Failed to clean up package file after verification failure %s: %v", outputFile, removeErr)
		}
		return errors.Wrapf(err, "package verification failed for %s", outputFile)
	}

	return nil
}

func verifyDirectoryWritable(dirPath string) error {
	testFile := filepath.Join(dirPath, ".gotya-test-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	file, err := os.Create(testFile)
	if err != nil {
		return errors.Wrapf(err, "cannot write to directory %s", dirPath)
	}

	// Close and remove the test file
	if err := file.Close(); err != nil {
		logger.Warnf("Failed to close test file %s: %v", testFile, err)
	}

	if err := os.Remove(testFile); err != nil {
		logger.Warnf("Failed to remove test file %s: %v", testFile, err)
	}

	return nil
}

// verifyPackage verifies the integrity of a package file.
func verifyPackage(pkgPath string, expectedMeta *Metadata) error {
	// Clean and validate package path
	cleanPkgPath, err := validatePath(pkgPath)
	if err != nil {
		return errors.Wrapf(err, "invalid package path")
	}

	// Check if package file exists and is readable
	fileInfo, err := os.Stat(cleanPkgPath)
	if os.IsNotExist(err) {
		return errors.Wrapf(errors.ErrFileNotFound, "package file does not exist: %s", cleanPkgPath)
	} else if err != nil {
		return errors.Wrapf(err, "failed to access package file %s", cleanPkgPath)
	}

	// Check if it's a regular file
	if !fileInfo.Mode().IsRegular() {
		return errors.Wrapf(errors.ErrInvalidFile, "not a regular file: %s", cleanPkgPath)
	}

	// Open the package file
	file, err := os.Open(cleanPkgPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open package file %s", cleanPkgPath)
	}
	defer file.Close()

	// Create a gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return errors.Wrapf(err, "failed to create gzip reader for %s", cleanPkgPath)
	}
	defer gzReader.Close()

	// Create a tar reader
	tarReader := tar.NewReader(gzReader)

	// Track found files and their hashes
	foundFiles := make(map[string]string)
	var metaData *Metadata

	// Process each file in the tarball
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return errors.Wrapf(err, "error reading tar archive %s", cleanPkgPath)
		}

		// Skip directories
		if header.Typeflag == tar.TypeDir {
			continue
		}

		// Read the file content
		content, err := io.ReadAll(tarReader)
		if err != nil {
			return errors.Wrapf(err, "error reading file %s from archive %s", header.Name, cleanPkgPath)
		}

		// Check if this is the metadata file
		if header.Name == "meta/package.json" {
			metaData = &Metadata{}
			if err := json.Unmarshal(content, metaData); err != nil {
				return errors.Wrapf(err, "failed to parse package metadata in %s", cleanPkgPath)
			}
			continue
		}

		// Calculate file hash
		hasher := sha256.New()
		hasher.Write(content)
		hash := hex.EncodeToString(hasher.Sum(nil))

		// Store the file hash
		foundFiles[header.Name] = hash
	}

	// Verify metadata was found
	if metaData == nil {
		return errors.Wrap(errors.ErrPackageInvalid, "package metadata not found in archive")
	}

	// Verify package name and version match
	if expectedMeta != nil {
		if metaData.Name != expectedMeta.Name || metaData.Version != expectedMeta.Version {
			return errors.Wrapf(errors.ErrValidation, "package name/version mismatch: expected %s-%s, got %s-%s",
				expectedMeta.Name, expectedMeta.Version, metaData.Name, metaData.Version)
		}

		// Verify all files in metadata exist in the archive and hashes match
		for _, file := range metaData.Files {
			hash, exists := foundFiles[file.Path]
			if !exists {
				return errors.Wrapf(errors.ErrValidation, "file %s is missing from package", file.Path)
			}

			// Verify file hash if specified in metadata
			if file.Digest != "" && hash != file.Digest {
				return errors.Wrapf(errors.ErrValidation, "hash mismatch for file %s: expected %s, got %s",
					file.Path, file.Digest, hash)
			}
		}

		// Check for missing files in the archive
		for _, expectedFile := range expectedMeta.Files {
			// Check both with and without 'files/' prefix for backward compatibility
			_, found1 := foundFiles[expectedFile.Path]
			var ok2, ok3 bool

			if strings.HasPrefix(expectedFile.Path, "files/") {
				_, ok2 = foundFiles[strings.TrimPrefix(expectedFile.Path, "files/")]
			} else {
				_, ok3 = foundFiles["files/"+expectedFile.Path]
			}

			if !found1 && !ok2 && !ok3 {
				return errors.Wrapf(errors.ErrValidation, "required file %s not found in package", expectedFile.Path)
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
			return nil, errors.Wrapf(errors.ErrFileNotFound, "metadata file not found: %s", metadataPath)
		}
		return nil, errors.Wrapf(err, "failed to open metadata file %s", metadataPath)
	}
	defer file.Close()

	// Read the file content
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read metadata file %s", metadataPath)
	}

	// Parse the JSON data
	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, errors.Wrapf(err, "failed to parse metadata from %s", metadataPath)
	}

	// Validate the metadata
	if meta.Name == "" {
		return nil, errors.Wrap(errors.ErrValidation, "missing required field: name")
	}
	if meta.Version == "" {
		return nil, errors.Wrap(errors.ErrValidation, "missing required field: version")
	}
	if meta.Description == "" {
		return nil, errors.Wrap(errors.ErrValidation, "missing required field: description")
	}

	return &meta, nil
}
