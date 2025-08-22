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
		return "", errors.Wrapf(errors.ErrInvalidPath, "path cannot be empty")
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
		return "", errors.Wrapf(err, "failed to access path %s", cleanPath)
	}

	return cleanPath, nil
}

// calculateFileHash calculates the SHA256 hash of a file.
func calculateFileHash(path string) (string, error) {
	// Validate the path
	abspath, err := validatePath(path)
	if err != nil {
		return "", errors.Wrapf(err, "failed to validate path %s", path)
	}

	file, err := os.Open(abspath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open file %s", abspath)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log the error but don't mask the original error if any
			logger.Warnf("Failed to close file %s: %v", abspath, closeErr)
		}
	}()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", errors.Wrapf(err, "failed to calculate hash for %s", abspath)
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
				return errors.Wrapf(err, "error accessing %s", path)
			}

			// Skip the files directory itself
			if path == filesDir {
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

			// Get relative path from the files directory
			relPath, err := filepath.Rel(filesDir, path)
			if err != nil {
				return errors.Wrapf(err, "failed to get relative path for %s", path)
			}

			// Calculate file hash
			hash, err := calculateFileHash(path)
			if err != nil {
				return errors.Wrapf(err, "failed to calculate hash for %s", path)
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
			return errors.Wrapf(err, "error walking the path %s", filesDir)
		}

		// If we found files in the 'files' directory, we're done
		if len(meta.Files) > 0 {
			return nil
		}
	}

	// If no 'files' directory or it was empty, process the source directory directly
	cleanSourceDir, err := validatePath(sourceDir)
	if err != nil {
		return errors.Wrapf(err, "invalid source directory %s", sourceDir)
	}

	err = filepath.Walk(cleanSourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrapf(err, "error accessing %s", path)
		}

		// Skip the source directory itself
		if path == cleanSourceDir {
			return nil
		}

		// Skip the 'meta' directory and its contents
		if strings.HasPrefix(path, filepath.Join(cleanSourceDir, "meta")) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip the 'files' directory (we already processed it)
		if strings.HasPrefix(path, filepath.Join(cleanSourceDir, "files")) {
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

		// Calculate relative path
		relPath, err := filepath.Rel(cleanSourceDir, path)
		if err != nil {
			return errors.Wrapf(err, "failed to get relative path for %s", path)
		}

		// Calculate file hash
		hash, err := calculateFileHash(path)
		if err != nil {
			return errors.Wrapf(err, "failed to calculate hash for %s", path)
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
		return errors.Wrapf(err, "error walking the path %s", cleanSourceDir)
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

// CreatePackage creates a new package from the source directory.
func CreatePackage(sourceDir, outputDir, pkgName, pkgVer, pkgOS, pkgArch string) error {
	// Validate package name
	if pkgName == "" {
		return errors.Wrapf(errors.ErrNameRequired, "package name cannot be empty")
	}
	if !packageNameRegex.MatchString(pkgName) {
		return errors.Wrapf(errors.ErrInvalidPackageName, "invalid package name: %s - must match %s", pkgName, packageNameRegex.String())
	}

	// Validate package version
	if pkgVer == "" {
		return errors.Wrapf(errors.ErrVersionRequired, "package version cannot be empty")
	}
	if !versionRegex.MatchString(pkgVer) {
		return errors.Wrapf(errors.ErrInvalidVersionString, "invalid package version: %s - must match %s", pkgVer, versionRegex.String())
	}

	// Validate OS and architecture
	if pkgOS == "" {
		return errors.Wrapf(errors.ErrTargetOSEmpty, "OS cannot be empty")
	}
	if pkgArch == "" {
		return errors.Wrapf(errors.ErrTargetArchEmpty, "architecture cannot be empty")
	}

	// Clean and validate source directory path
	cleanSourceDir, err := validatePath(sourceDir)
	if err != nil {
		return errors.Wrapf(err, "invalid source directory %s", sourceDir)
	}

	// Ensure source directory exists and is readable
	if _, err := os.Stat(cleanSourceDir); os.IsNotExist(err) {
		return errors.Wrapf(err, "source directory does not exist: %s", cleanSourceDir)
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
			return errors.Wrapf(removeErr, "failed to clean up partially created package file %s", outputFile)
		}
		return errors.Wrapf(err, "failed to create package %s", outputFile)
	}

	// Verify the created package
	if err := verifyPackage(outputFile, meta); err != nil {
		// Clean up the output file if verification failed
		if removeErr := os.Remove(outputFile); removeErr != nil && !os.IsNotExist(removeErr) {
			return errors.Wrapf(removeErr, "failed to clean up package file after verification failure %s", outputFile)
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

// createTarball creates a gzipped tarball from the source directory.
func createTarball(sourceDir, outputPath string, meta *Metadata) error {
	// Create the output file
	file, err := os.Create(outputPath)
	if err != nil {
		return errors.Wrapf(err, "failed to create output file %s", outputPath)
	}
	defer file.Close()

	// Create a gzip writer
	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	// Create a tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Walk through the source directory and add files to the tarball
	err = filepath.Walk(sourceDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrapf(err, "error walking to %s", filePath)
		}

		// Get the relative path
		relPath, err := filepath.Rel(sourceDir, filePath)
		if err != nil {
			return errors.Wrapf(err, "error getting relative path for %s", filePath)
		}

		// Skip the output file if it's in the source directory
		if relPath == filepath.Base(outputPath) {
			return nil
		}

		// Create a new tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return errors.Wrapf(err, "error creating tar header for %s", filePath)
		}

		// Update the header name to use forward slashes and relative path
		header.Name = filepath.ToSlash(relPath)

		// Write the header to the tarball
		if err := tarWriter.WriteHeader(header); err != nil {
			return errors.Wrapf(err, "error writing header for %s", filePath)
		}

		// If it's a regular file, write its content
		if info.Mode().IsRegular() {
			file, err := os.Open(filePath)
			if err != nil {
				return errors.Wrapf(err, "error opening file %s", filePath)
			}
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return errors.Wrapf(err, "error writing file content for %s", filePath)
			}
		}

		return nil
	})
	if err != nil {
		return errors.Wrap(err, "error walking source directory")
	}

	// Add the metadata file to the tarball
	metaJSON, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return errors.Wrap(err, "error marshaling metadata to JSON")
	}

	header := &tar.Header{
		Name:    "meta/package.json",
		Size:    int64(len(metaJSON)),
		Mode:    0o644,
		ModTime: time.Now(),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return errors.Wrap(err, "error writing metadata header")
	}

	if _, err := tarWriter.Write(metaJSON); err != nil {
		return errors.Wrap(err, "error writing metadata content")
	}

	return nil
}

// readPackageMetadata reads and parses package metadata from a JSON file.
func readPackageMetadata(metadataPath string) (*Metadata, error) {
	// Open the metadata file
	file, err := os.Open(metadataPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open metadata file %s", metadataPath)
	}
	defer file.Close()

	// Read the file content
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read metadata file %s", metadataPath)
	}

	// Parse the JSON content
	var meta Metadata
	if err := json.Unmarshal(content, &meta); err != nil {
		return nil, errors.Wrapf(err, "failed to parse metadata from %s", metadataPath)
	}

	// Validate required fields
	if meta.Name == "" {
		return nil, errors.Wrap(errors.ErrNameRequired, "package name is required")
	}
	if meta.Version == "" {
		return nil, errors.Wrap(errors.ErrVersionRequired, "package version is required")
	}

	return &meta, nil
}

// verifyPackage verifies the integrity of a package file.
func verifyPackage(pkgPath string, expectedMeta *Metadata) error {
	// Clean and validate package path
	cleanPkgPath, err := validatePath(pkgPath)
	if err != nil {
		return errors.Wrap(err, "invalid package path")
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
			return errors.Wrapf(ErrValidationFailed, "package name/version mismatch: expected %s-%s, got %s-%s",
				expectedMeta.Name, expectedMeta.Version, metaData.Name, metaData.Version)
		}

		// Verify all files in metadata exist in the archive and hashes match
		for _, file := range metaData.Files {
			// Check both with and without 'files/' prefix for backward compatibility
			hash, found1 := foundFiles[file.Path]
			var hash2 string
			var found2 bool

			if strings.HasPrefix(file.Path, "files/") {
				hash2, found2 = foundFiles[strings.TrimPrefix(file.Path, "files/")]
			} else {
				hash2, found2 = foundFiles["files/"+file.Path]
			}

			if !found1 && !found2 {
				return errors.Wrapf(errors.ErrValidation, "file %s is missing from package", file.Path)
			}

			// Verify file hash if specified in metadata
			if file.Digest != "" {
				if (found1 && hash != file.Digest) && (found2 && hash2 != file.Digest) {
					return errors.Wrapf(errors.ErrValidation, "hash mismatch for file %s: expected %s",
						file.Path, file.Digest)
				}
			}
		}
	}

	return nil
}
