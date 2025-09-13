package artifact

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	goerrors "errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/fsutil"
	"github.com/cperrin88/gotya/pkg/logger"
)

// Common validation patterns.

// Common validation patterns.
var (
	packageNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	versionRegex     = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.+_-]*$`)
)

// in the artifact meta directory.
type Metadata struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Maintainer   string            `json:"maintainer,omitempty"`
	Description  string            `json:"description"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Files        []File            `json:"files,omitempty"`
	Hooks        map[string]string `json:"hooks,omitempty"`
}

// File represents a file entry in the artifact metadata.
type File struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Mode   uint32 `json:"mode"`
	Digest string `json:"digest"`
}

// validatePath validates that a path exists and converts it to an absolute path.
func validatePath(path string) (string, error) {
	if path == "" {
		return "", errors.Wrapf(errors.ErrInvalidPath, "path cannot be empty")
	}

	// Clean the path
	cleanPath := filepath.Clean(path)

	// Convert to absolute path if it's not already
	if !filepath.IsAbs(cleanPath) {
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			return "", errors.Wrapf(err, "failed to convert path to absolute: %s", cleanPath)
		}
		cleanPath = absPath
	}

	// Check if the path exists
	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		return "", errors.Wrapf(errors.ErrFileNotFound, "path does not exist: %s", cleanPath)
	} else if err != nil {
		return "", errors.Wrapf(err, "failed to access path %s", cleanPath)
	}

	return cleanPath, nil
}

// validateArtifactStructure validates the artifact directory structure.
// It ensures that:
// - The source directory contains a 'files' subdirectory
// - No artifact.json exists in the source directory
// - If a meta directory exists, it only contains allowed files (*.tengo hooks scripts)
func validateArtifactStructure(sourceDir string) error {
	// Check for required files directory
	filesDir := filepath.Join(sourceDir, "files")
	if _, err := os.Stat(filesDir); os.IsNotExist(err) {
		return errors.Wrapf(errors.ErrInvalidPath, "missing required directory: files")
	} else if err != nil {
		return errors.Wrapf(err, "failed to access files directory")
	}

	// Check for artifact.json in source directory (shouldn't exist yet)
	if _, err := os.Stat(filepath.Join(sourceDir, "artifact.json")); err == nil {
		return errors.Wrapf(errors.ErrInvalidPath, "artifact.json already exists in source directory")
	} else if !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to check for artifact.json")
	}

	// Check if meta directory exists
	metaDir := filepath.Join(sourceDir, "meta")
	if _, err := os.Stat(metaDir); os.IsNotExist(err) {
		// Meta directory doesn't exist, which is fine
		return nil
	} else if err != nil {
		return errors.Wrap(err, "failed to access meta directory")
	}

	// Meta directory exists, validate its contents
	entries, err := os.ReadDir(metaDir)
	if err != nil {
		return errors.Wrap(err, "failed to read meta directory")
	}

	for _, entry := range entries {
		// Skip directories
		if entry.IsDir() {
			continue
		}

		// Only allow .tengo files in meta directory
		if filepath.Ext(entry.Name()) != ".tengo" {
			return errors.Wrapf(errors.ErrInvalidPath,
				"invalid file in meta directory: %s (only .tengo files allowed)", entry.Name())
		}
	}

	return nil
}

// processArtifactFiles processes all files in the files/ directory of a artifact.
// It returns a slice of File structs containing metadata about each file.
// The function will return an error if:
// - The files directory is empty
// - Any file cannot be read or hashed
// - Any symlink points outside the files directory
func processArtifactFiles(filesDir string) ([]File, error) {
	// Verify the files directory exists and is accessible
	if _, err := os.Stat(filesDir); os.IsNotExist(err) {
		return nil, errors.Wrapf(errors.ErrFileNotFound, "files directory not found: %s", filesDir)
	} else if err != nil {
		return nil, errors.Wrapf(err, "failed to access files directory: %s", filesDir)
	}

	var files []File
	err := filepath.Walk(filesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrapf(err, "error accessing path %s", path)
		}

		// Skip the root directory
		if path == filesDir {
			return nil
		}

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return errors.Wrapf(err, "failed to read symlink %s", path)
			}

			// Ensure symlink points within the files directory
			targetPath := filepath.Join(filepath.Dir(path), target)
			relPath, err := filepath.Rel(filesDir, targetPath)
			if err != nil || strings.HasPrefix(relPath, "..") {
				return errors.Wrapf(errors.ErrInvalidPath,
					"symlink %s points outside the files directory", path)
			}
			return nil
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Calculate file hash
		hash, err := calculateFileHash(path)
		if err != nil {
			return errors.Wrapf(err, "failed to calculate hash for %s", path)
		}

		// Get relative path from the files directory
		relPath, err := filepath.Rel(filesDir, path)
		if err != nil {
			return errors.Wrapf(err, "failed to get relative path for %s", path)
		}

		// Add file to results
		files = append(files, File{
			Path:   filepath.ToSlash(filepath.Join("files", relPath)), // Use forward slashes for consistency
			Size:   info.Size(),
			Mode:   uint32(info.Mode()),
			Digest: hash,
		})

		return nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "error walking the files directory")
	}

	// Ensure we found at least one file
	if len(files) == 0 {
		return nil, goerrors.New("no files found in the files directory")
	}

	return files, nil
}

// processHookScripts processes hooks scripts in the meta/ directory.
// It validates that only referenced hooks scripts exist and returns their metadata.
// The function will return an error if:
// - The meta directory is not accessible (when it should be)
// - Any hooks script cannot be read or hashed
// - Any file in meta/ is not referenced in the hooks map
func processHookScripts(metaDir string, hooks map[string]string) ([]File, error) {
	// If no hooks are defined, ensure the meta directory is empty or doesn't exist
	if len(hooks) == 0 {
		if _, err := os.Stat(metaDir); os.IsNotExist(err) {
			// Meta directory doesn't exist, which is fine
			return nil, nil
		}
		// Directory exists, check if it's empty
		entries, err := os.ReadDir(metaDir)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read meta directory %s", metaDir)
		}
		if len(entries) > 0 {
			return nil, errors.Wrapf(errors.ErrInvalidPath,
				"meta directory %s is not empty but no hooks are defined", metaDir)
		}
		return nil, nil
	}

	// We have hooks defined, so the meta directory must exist
	if _, err := os.Stat(metaDir); os.IsNotExist(err) {
		return nil, errors.Wrapf(errors.ErrInvalidPath,
			"hooks are defined but meta directory %s does not exist", metaDir)
	} else if err != nil {
		return nil, errors.Wrapf(err, "failed to access meta directory %s", metaDir)
	}

	// Create a map of expected hooks scripts for quick lookup
	expectedHooks := make(map[string]bool, len(hooks))
	for _, hookPath := range hooks {
		expectedHooks[hookPath] = true
	}

	var hookFiles []File

	// Process each file in the meta directory
	entries, err := os.ReadDir(metaDir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read meta directory %s", metaDir)
	}

	for _, entry := range entries {
		// Skip directories
		if entry.IsDir() {
			continue
		}

		// Only process .tengo files
		if filepath.Ext(entry.Name()) != ".tengo" {
			continue
		}

		hookPath := filepath.Join(metaDir, entry.Name())
		info, err := os.Stat(hookPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get file info for %s", hookPath)
		}

		// Check if this hooks is expected
		if !expectedHooks[entry.Name()] {
			return nil, errors.Wrapf(errors.ErrInvalidPath,
				"unexpected hooks script: %s (not referenced in hooks)", entry.Name())
		}

		// Calculate file hash
		hash, err := calculateFileHash(hookPath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to calculate hash for hooks script %s", hookPath)
		}

		// Add to results
		hookFiles = append(hookFiles, File{
			Path:   filepath.ToSlash(entry.Name()),
			Size:   info.Size(),
			Mode:   uint32(info.Mode()),
			Digest: hash,
		})

		// Remove from expected hooks to track which ones we've processed
		delete(expectedHooks, entry.Name())
	}

	// Check if any expected hooks were not found
	if len(expectedHooks) > 0 {
		missingHooks := make([]string, 0, len(expectedHooks))
		for hook := range expectedHooks {
			missingHooks = append(missingHooks, hook)
		}
		return nil, errors.Wrapf(errors.ErrFileNotFound,
			"missing hooks scripts: %v", missingHooks)
	}

	return hookFiles, nil
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

// processFiles orchestrates the processing of artifact files and hooks scripts.
// It validates the artifact structure and processes files according to the artifact design.
// The function will return an error if:
// - The artifact structure is invalid
// - No files are found in the artifact
// - Any file processing fails
// - Any hooks script validation fails
func processFiles(sourceDir string, meta *Metadata) error {
	// Clear existing files
	meta.Files = []File{}

	// 1. Validate artifact structure first
	if err := validateArtifactStructure(sourceDir); err != nil {
		return errors.Wrap(err, "invalid artifact structure")
	}

	// 2. Process artifact files from the 'files' directory
	filesDir := filepath.Join(sourceDir, "files")
	files, err := processArtifactFiles(filesDir)
	if err != nil {
		return errors.Wrapf(err, "failed to process artifact files in %s", filesDir)
	}
	meta.Files = files

	// 3. Process hooks scripts
	metaDir := filepath.Join(sourceDir, "meta")
	hookFiles, err := processHookScripts(metaDir, meta.Hooks)
	if err != nil {
		return errors.Wrap(err, "failed to process hooks scripts")
	}

	// Add hooks files to the files list with 'meta/' prefix
	for _, hookFile := range hookFiles {
		hookFile.Path = filepath.ToSlash(filepath.Join("meta", hookFile.Path))
		meta.Files = append(meta.Files, hookFile)
	}

	// 4. Ensure we have at least one file in the artifact
	if len(meta.Files) == 0 {
		return goerrors.New("artifact must contain at least one file")
	}

	return nil
}

// CreateArtifact creates a new artifact from the source directory.
// It validates input parameters, processes files, and creates a verified artifact.
// The artifact will be created in the output directory with the specified name, version, OS, and architecture.
// Additional metadata like maintainer, description, and dependencies can be provided.
func CreateArtifact(
	sourceDir, outputDir, pkgName, pkgVer, pkgOS, pkgArch string,
	maintainer string,
	description string,
	dependencies []string,
	hooks map[string]string,
) (string, error) {
	// Validate artifact name and version
	switch {
	case pkgName == "":
		return "", errors.Wrapf(errors.ErrNameRequired, "artifact name cannot be empty")
	case !packageNameRegex.MatchString(pkgName):
		return "", errors.Wrapf(
			errors.ErrInvalidArtifactName,
			"invalid artifact name: %s - must match %s",
			pkgName,
			packageNameRegex.String(),
		)
	case pkgVer == "":
		return "", errors.Wrapf(errors.ErrVersionRequired, "artifact version cannot be empty")
	case !versionRegex.MatchString(pkgVer):
		return "", errors.Wrapf(
			errors.ErrInvalidVersionString,
			"invalid artifact version: %s - must match %s",
			pkgVer,
			versionRegex.String(),
		)
	case pkgOS == "":
		return "", errors.Wrapf(errors.ErrTargetOSEmpty, "OS cannot be empty")
	case pkgArch == "":
		return "", errors.Wrapf(errors.ErrTargetArchEmpty, "architecture cannot be empty")
	}

	// Validate and clean paths
	cleanSourceDir, err := validatePath(sourceDir)
	if err != nil {
		return "", errors.Wrapf(err, "invalid source directory %s", sourceDir)
	}

	// Clean the output directory path
	outputDir = filepath.Clean(outputDir)

	// Create output directory if it doesn't exist
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		if err := os.MkdirAll(outputDir, fsutil.DirModeDefault); err != nil {
			return "", errors.Wrapf(err, "failed to create output directory %s", outputDir)
		}
	} else if err != nil {
		return "", errors.Wrapf(err, "failed to access output directory %s", outputDir)
	}

	// Ensure output directory is writable
	if err := verifyDirectoryWritable(outputDir); err != nil {
		return "", errors.Wrapf(err, "output directory %s is not writable", outputDir)
	}

	// Initialize metadata with provided values or defaults
	if description == "" {
		description = fmt.Sprintf("Artifact %s version %s", pkgName, pkgVer)
	}

	meta := &Metadata{
		Name:         pkgName,
		Version:      pkgVer,
		Maintainer:   maintainer,
		Description:  description,
		Dependencies: dependencies,
		Hooks:        hooks,
		Files:        []File{},
	}

	// Process files and update metadata - this validates source directory and file structure
	if err := processFiles(cleanSourceDir, meta); err != nil {
		return "", errors.Wrapf(err, "failed to process files in %s", cleanSourceDir)
	}

	// Create output filename and artifact
	outputFile := filepath.Join(outputDir, fmt.Sprintf("%s_%s_%s_%s.tar.gz", pkgName, pkgVer, pkgOS, pkgArch))

	// Create and verify the tarball
	if err := createTarball(cleanSourceDir, outputFile, meta); err != nil {
		// Clean up the output file if it was partially created
		if removeErr := os.Remove(outputFile); removeErr != nil && !os.IsNotExist(removeErr) {
			return "", errors.Wrapf(removeErr, "failed to clean up partially created artifact file %s", outputFile)
		}
		return "", errors.Wrapf(err, "failed to create artifact %s", outputFile)
	}

	// Verify the created artifact
	if err := verifyArtifact(outputFile, meta); err != nil {
		if removeErr := os.Remove(outputFile); removeErr != nil && !os.IsNotExist(removeErr) {
			return "", errors.Wrapf(removeErr, "failed to clean up artifact file after verification failure %s", outputFile)
		}
		return "", errors.Wrapf(err, "artifact verification failed for %s", outputFile)
	}

	return outputFile, nil
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

	metaJSON = append(metaJSON, '\n')

	header := &tar.Header{
		Name:    "meta/artifact.json",
		Size:    int64(len(metaJSON)),
		Mode:    fsutil.FileModeDefault,
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

// openArtifactFile opens the artifact file and returns a reader.
func openArtifactFile(pkgPath string) (*os.File, error) {
	file, err := os.Open(pkgPath)
	if err != nil {
		return nil, errors.WrapFileError(err, "open artifact file", pkgPath)
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

// verifyArtifact verifies the integrity of a artifact file.
func verifyArtifact(pkgPath string, expectedMeta *Metadata) error {
	if expectedMeta == nil {
		return errors.Wrap(errors.ErrValidationFailed, "metadata cannot be nil")
	}

	logger.Debugf("Starting verification of artifact: %s", pkgPath)

	// Open the artifact file
	file, err := openArtifactFile(pkgPath)
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

	// Process the artifact contents
	foundFiles, err := processArtifactContents(gzipReader, expectedMeta)
	if err != nil {
		return errors.Wrap(err, "failed to process artifact contents")
	}

	// Verify all expected files were found
	if err := verifyExpectedFiles(foundFiles, expectedMeta.Files); err != nil {
		return errors.Wrap(err, "artifact verification failed")
	}

	return nil
}

// processArtifactContents processes the contents of a artifact file and verifies each file.
func processArtifactContents(reader io.Reader, expectedMeta *Metadata) (map[string]bool, error) {
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
	return flag == tar.TypeReg
}

// processFile processes and verifies a single file from the artifact.
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

	if err := addMetadataFile(expectedFiles, meta); err != nil {
		return nil, err
	}

	return expectedFiles, nil
}

// addMetadataFile adds the artifact metadata file to the expected files map.
// It generates the metadata in memory without writing to disk.
func addMetadataFile(expectedFiles map[string]File, meta *Metadata) error {
	// Marshal the metadata to JSON with indentation for consistent hashing
	jsonData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return errors.Wrap(err, "marshal metadata to JSON")
	}

	// Add newline to match file writing behavior
	jsonData = append(jsonData, '\n')

	// Calculate the hash of the JSON data
	hash := sha256.Sum256(jsonData)
	hexHash := hex.EncodeToString(hash[:])

	// Add the metadata file to expected files
	expectedFiles["meta/artifact.json"] = File{
		Path:   "meta/artifact.json",
		Size:   int64(len(jsonData)),
		Mode:   fsutil.FileModeDefault,
		Digest: hexHash,
	}

	return nil
}

// verifyExpectedFiles verifies that all expected files were found in the artifact.
func verifyExpectedFiles(foundFiles map[string]bool, expectedFiles []File) error {
	for _, file := range expectedFiles {
		if !foundFiles[file.Path] {
			return errors.NewMissingFileError(file.Path)
		}
	}

	return nil
}
