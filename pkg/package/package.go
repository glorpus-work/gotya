package pkg

import (
	"archive/tar"
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

// File permission constants.
const (
	// DefaultFileMode is the default file mode for regular files (0o644).
	DefaultFileMode = 0o644
	// DefaultDirMode is the default directory mode (0o755).
	DefaultDirMode = 0o755
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

// CreatePackage creates a new package from the source directory.
func CreatePackage(sourceDir, outputDir, pkgName, pkgVer, pkgOS, pkgArch string) error {
	// Validate package name
	if pkgName == "" {
		return errors.Wrapf(errors.ErrNameRequired, "package name cannot be empty")
	}
	if !packageNameRegex.MatchString(pkgName) {
		return errors.Wrapf(
			errors.ErrInvalidPackageName,
			"invalid package name: %s - must match %s",
			pkgName,
			packageNameRegex.String(),
		)
	}

	// Validate package version
	if pkgVer == "" {
		return errors.Wrapf(errors.ErrVersionRequired, "package version cannot be empty")
	}
	if !versionRegex.MatchString(pkgVer) {
		return errors.Wrapf(
			errors.ErrInvalidVersionString,
			"invalid package version: %s - must match %s",
			pkgVer,
			versionRegex.String(),
		)
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

	// Check if files/ directory exists and is not empty
	filesDir := filepath.Join(cleanSourceDir, "files")
	if _, err := os.Stat(filesDir); os.IsNotExist(err) {
		return errors.Wrapf(errors.ErrPackageInvalid, "package must contain a 'files' directory")
	}

	// Check if files directory is empty
	dir, err := os.Open(filesDir)
	if err != nil {
		return errors.Wrapf(err, "failed to open files directory %s", filesDir)
	}
	defer dir.Close()

	_, err = dir.Readdirnames(1)
	if err == io.EOF {
		return errors.Wrapf(errors.ErrPackageInvalid, "package 'files' directory must not be empty")
	} else if err != nil {
		return errors.Wrapf(err, "failed to read files directory %s", filesDir)
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
		Mode:    DefaultFileMode,
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
	file, err := os.Open(metadataPath)
	if err != nil {
		return nil, errors.WrapFileError(err, "open", metadataPath)
	}
	defer file.Close()

	var meta Metadata
	if err := json.NewDecoder(file).Decode(&meta); err != nil {
		return nil, errors.WrapJSONError(err, "decode metadata")
	}

	return &meta, nil
}

// openPackageFile opens the package file and returns a reader.
func openPackageFile(pkgPath string) (*os.File, error) {
	file, err := os.Open(pkgPath)
	if err != nil {
		return nil, errors.WrapFileError(err, "open package file", pkgPath)
	}
	return file, nil
}

// createGzipReader creates a gzip reader from a file.
func createGzipReader(file *os.File) (*gzip.Reader, error) {
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, errors.Wrap(err, "create gzip reader")
	}
	return gzipReader, nil
}

// verifyPackage verifies the integrity of a package file.
func verifyPackage(pkgPath string, expectedMeta *Metadata) error {
	if expectedMeta == nil {
		return errors.Wrap(errors.ErrValidationFailed, "metadata cannot be nil")
	}

	logger.Debugf("Starting verification of package: %s", pkgPath)

	// Open the package file
	file, err := openPackageFile(pkgPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create a gzip reader
	gzipReader, err := createGzipReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	// Process the package contents
	foundFiles, err := processPackageContents(gzipReader, expectedMeta)
	if err != nil {
		return errors.Wrap(err, "failed to process package contents")
	}

	// Verify all expected files were found
	if err := verifyExpectedFiles(foundFiles, expectedMeta.Files); err != nil {
		return errors.Wrap(err, "package verification failed")
	}

	return nil
}

// processPackageContents processes the contents of a package file and verifies each file.
func processPackageContents(reader io.Reader, expectedMeta *Metadata) (map[string]bool, error) {
	tarReader := tar.NewReader(reader)
	foundFiles := make(map[string]bool)
	expectedFiles, err := createExpectedFilesMap(expectedMeta)
	if err != nil {
		return nil, err
	}

	// Process each file in the tarball
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "read tar header")
		}

		// Skip non-regular files
		if !isRegularFile(header.Typeflag) {
			continue
		}

		// Process and verify the file
		if err := processFile(header, tarReader, expectedFiles, foundFiles); err != nil {
			return nil, err
		}
	}

	return foundFiles, nil
}

// isRegularFile checks if the tar header represents a regular file.
func isRegularFile(flag byte) bool {
	return flag == tar.TypeReg || flag == tar.TypeRegA
}

// processFile processes and verifies a single file from the package.
func processFile(header *tar.Header, reader io.Reader, expectedFiles map[string]File, foundFiles map[string]bool) error {
	// Normalize the path for comparison
	normalizedPath := filepath.ToSlash(header.Name)

	// Find the file in expected files
	fileInfo, exists := findExpectedFile(normalizedPath, expectedFiles)
	if !exists {
		// Log available expected files for debugging
		logAvailableFiles(expectedFiles)
		return errors.NewUnexpectedFileError(normalizedPath)
	}

	// Mark file as found
	foundFiles[normalizedPath] = true

	// Verify file metadata
	if err := verifyFileMetadata(header, fileInfo); err != nil {
		return err
	}

	// Verify file content hash if we have a digest
	if fileInfo.Digest != "" {
		hasher := sha256.New()
		if _, err := io.Copy(hasher, reader); err != nil {
			return errors.WrapFileError(err, "calculate hash for", header.Name)
		}

		// Verify the hash for all other files
		actualHash := hex.EncodeToString(hasher.Sum(nil))
		if actualHash != fileInfo.Digest {
			return errors.NewFileHashMismatchError(header.Name, fileInfo.Digest, actualHash)
		}
	}

	return nil
}

// findExpectedFile finds a file in the expected files map, trying alternative paths if necessary.
func findExpectedFile(path string, expectedFiles map[string]File) (File, bool) {
	// Try exact match first
	if fileInfo, exists := expectedFiles[path]; exists {
		return fileInfo, true
	}

	// Try with a different path format (handles potential path separator differences)
	altPath := filepath.ToSlash(filepath.Join(filepath.Dir(path), filepath.Base(path)))
	if altFileInfo, altExists := expectedFiles[altPath]; altExists {
		return altFileInfo, true
	}

	return File{}, false
}

// logAvailableFiles logs the list of available expected files for debugging.
func logAvailableFiles(expectedFiles map[string]File) {
	var availableFiles []string
	for k := range expectedFiles {
		availableFiles = append(availableFiles, k)
	}
	logger.Debugf("Available expected files: %v", availableFiles)
}

// verifyFileMetadata verifies the file metadata (size and mode).
func verifyFileMetadata(header *tar.Header, fileInfo File) error {
	// Verify file size
	if header.Size != fileInfo.Size {
		return errors.NewFileSizeMismatchError(header.Name, fileInfo.Size, header.Size)
	}

	// Verify file mode - convert both to the same type for comparison
	if uint32(header.Mode) != fileInfo.Mode {
		return errors.NewFileModeMismatchError(header.Name, uint32(header.Mode), fileInfo.Mode)
	}

	return nil
}

// createExpectedFilesMap creates a map of expected files from the metadata.
func createExpectedFilesMap(meta *Metadata) (map[string]File, error) {
	expectedFiles := make(map[string]File)

	// Add regular files
	for _, f := range meta.Files {
		expectedFiles[f.Path] = f
	}

	// Add metadata file if needed
	if meta != nil {
		if err := addMetadataFile(expectedFiles, meta); err != nil {
			return nil, err
		}
	}

	return expectedFiles, nil
}

// addMetadataFile adds the package metadata file to the expected files map.
func addMetadataFile(expectedFiles map[string]File, meta *Metadata) error {
	// Create a temporary file to store the metadata
	tmpFile, err := os.CreateTemp("", "package-meta-*.json")
	if err != nil {
		return errors.Wrap(err, "create temporary file for metadata")
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Write the expected metadata to the temporary file
	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(meta); err != nil {
		tmpFile.Close()
		return errors.Wrap(err, "encode expected metadata")
	}
	if err := tmpFile.Close(); err != nil {
		return errors.Wrap(err, "close temporary metadata file")
	}

	// Add the metadata file to expected files
	hash, err := calculateFileHash(tmpPath)
	if err != nil {
		return errors.Wrap(err, "calculate hash for expected metadata")
	}

	fileInfo, err := os.Stat(tmpPath)
	if err != nil {
		return errors.Wrap(err, "get file info for expected metadata")
	}

	expectedFiles["meta/package.json"] = File{
		Path:   "meta/package.json",
		Size:   fileInfo.Size(),
		Mode:   uint32(fileInfo.Mode()),
		Digest: hash,
	}

	return nil
}

// verifyExpectedFiles verifies that all expected files were found in the package.
func verifyExpectedFiles(foundFiles map[string]bool, expectedFiles []File) error {
	for _, file := range expectedFiles {
		if !foundFiles[file.Path] {
			// Check if the file exists with a different path prefix
			var altPath string
			if strings.HasPrefix(file.Path, "files/") {
				altPath = strings.TrimPrefix(file.Path, "files/")
			} else {
				altPath = filepath.Join("files", file.Path)
			}

			if !foundFiles[altPath] {
				return errors.NewMissingFileError(file.Path)
			}
		}
	}

	return nil
}
