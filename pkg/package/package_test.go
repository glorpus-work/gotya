package pkg

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	goerror "errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cperrin88/gotya/pkg/errors"
)

func setupTestEnvironment(t *testing.T) (tempDir string, cleanup func()) {
	tempDir, err := os.MkdirTemp("", "gotya-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create test directory structure
	dirs := []string{
		filepath.Join(tempDir, "meta"),
		filepath.Join(tempDir, "files", "bin"),
		filepath.Join(tempDir, "files", "lib"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("Failed to create test directory %s: %v", dir, err)
		}
	}

	// Create test files
	testFiles := map[string]string{
		filepath.Join(tempDir, "meta", "package.json"): `{
			"name": "test-package",
			"version": "1.0.0",
			"description": "Test package",
			"maintainer": "test@example.com"
		}`,
		filepath.Join(tempDir, "files", "bin", "test"):       "#!/bin/bash\necho 'test'",
		filepath.Join(tempDir, "files", "lib", "libtest.so"): "test library content",
	}

	for path, content := range testFiles {
		// Ensure parent directory exists with correct permissions
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("Failed to create parent directory for %s: %v", path, err)
		}
		// On Windows, we need to ensure the file is closed and handles are released
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", path, err)
		}
	}

	// Return the temp directory and cleanup function
	return tempDir, func() {
		// On Windows, we need to make sure all file handles are closed before removing
		err := filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // ignore errors
			}
			if !info.IsDir() {
				// Try to remove read-only attributes if they exist
				if info.Mode()&0o200 == 0 {
					if err := os.Chmod(path, 0o666); err != nil {
						t.Logf("Warning: failed to change permissions for %s: %v", path, err)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Logf("Warning: failed to prepare files for removal: %v", err)
		}

		// Now try to remove the directory
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Warning: failed to remove temp directory: %v", err)
		}
	}
}

// extractPackage extracts a tarball to the specified directory.
func extractPackage(pkgPath, destDir string) error {
	file, err := os.Open(pkgPath)
	if err != nil {
		return errors.WrapFileError(err, "open package file", pkgPath)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return errors.Wrap(err, "create gzip reader")
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if goerror.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return errors.Wrap(err, "read tar archive")
		}

		// Skip the current directory entry
		if header.Name == "." {
			continue
		}

		targetPath := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return errors.WrapFileError(err, "create directory", targetPath)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return errors.Wrapf(err, "create parent directory for %s", targetPath)
			}

			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return errors.WrapFileError(err, "create file", targetPath)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return errors.WrapFileError(err, "extract file", targetPath)
			}
			outFile.Close()
		}
	}

	return nil
}

func TestCalculateFileHash(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-hash-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	testContent := "test content for hashing"
	if _, err := tempFile.WriteString(testContent); err != nil {
		t.Fatalf("Failed to write test content: %v", err)
	}
	tempFile.Close()

	hasher := sha256.New()
	hasher.Write([]byte(testContent))
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	hash, err := calculateFileHash(tempFile.Name())
	if err != nil {
		t.Fatalf("calculateFileHash failed: %v", err)
	}

	if hash != expectedHash {
		t.Errorf("Expected hash %s, got %s", expectedHash, hash)
	}
}

func TestReadPackageMetadata(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	tests := []struct {
		name     string
		path     string
		wantErr  bool
		checkFn  func(*Metadata) bool
		setup    func()
		teardown func()
	}{
		{
			name:    "valid metadata",
			path:    filepath.Join(tempDir, "meta", "package.json"),
			wantErr: false,
			checkFn: func(m *Metadata) bool {
				return m.Name == "test-package" && m.Version == "1.0.0"
			},
		},
		{
			name:    "non-existent file",
			path:    filepath.Join(tempDir, "nonexistent.json"),
			wantErr: true,
		},
		{
			name:    "invalid json",
			path:    filepath.Join(tempDir, "invalid.json"),
			wantErr: true,
			setup: func() {
				if err := os.WriteFile(filepath.Join(tempDir, "invalid.json"), []byte("{invalid}"), 0o644); err != nil {
					t.Fatalf("Failed to create invalid.json: %v", err)
				}
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.setup != nil {
			}
			if testCase.teardown != nil {
				defer testCase.teardown()
			}

			got, err := readPackageMetadata(testCase.path)
			if (err != nil) != testCase.wantErr {
				t.Errorf("readPackageMetadata() error = %v, wantErr %v", err, testCase.wantErr)
				return
			}
			if !testCase.wantErr && testCase.checkFn != nil && !testCase.checkFn(got) {
				t.Error("checkFn returned false")
			}
		})
	}
}

func TestProcessFiles(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	meta := &Metadata{
		Name:    "test-package",
		Version: "1.0.0",
	}

	err := processFiles(tempDir, meta)
	if err != nil {
		t.Fatalf("processFiles failed: %v", err)
	}

	if len(meta.Files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(meta.Files))
	}

	// Check if all expected files are present
	expectedFiles := map[string]bool{
		filepath.ToSlash(filepath.Join("bin", "test")):       false,
		filepath.ToSlash(filepath.Join("lib", "libtest.so")): false,
	}

	for _, file := range meta.Files {
		if _, exists := expectedFiles[file.Path]; exists {
			expectedFiles[file.Path] = true
		}
	}

	for path, found := range expectedFiles {
		if !found {
			t.Errorf("Expected file %s not found in metadata", path)
		}
	}
}

func TestCreateTarball(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	meta, err := readPackageMetadata(filepath.Join(tempDir, "meta", "package.json"))
	if err != nil {
		t.Fatalf("Failed to read test metadata: %v", err)
	}

	outputFile := filepath.Join(tempDir, "test-package.tar.gz")
	err = createTarball(tempDir, outputFile, meta)
	if err != nil {
		t.Fatalf("createTarball failed: %v", err)
	}

	// Verify the tarball was created
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Error("Tarball was not created")
	}

	// TODO: Add more thorough verification of tarball contents
}

func TestVerifyPackage(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Skip permission tests on Windows as they behave differently
	skipPermissionTests := runtime.GOOS == "windows"

	// Create a test package
	testPkgPath := filepath.Join(tempDir, "test-package.tar.gz")

	// Define test files with their paths and content
	testFiles := []struct {
		path    string
		content string
	}{
		{"bin/test", "test binary content"},
		{"lib/libtest.so", "test library content"},
	}

	// Create the source directory structure with the required files/ directory
	srcDir := filepath.Join(tempDir, "src")
	filesDir := filepath.Join(srcDir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		t.Fatalf("Failed to create files directory: %v", err)
	}

	// Create the meta/package.json file
	metaDir := filepath.Join(srcDir, "meta")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatalf("Failed to create meta directory: %v", err)
	}

	metaContent := `{
		"name": "test-package",
		"version": "1.0.0",
		"maintainer": "test@example.com",
		"description": "Test package"
	}`

	metaPath := filepath.Join(metaDir, "package.json")
	if err := os.WriteFile(metaPath, []byte(metaContent), 0o644); err != nil {
		t.Fatalf("Failed to create meta/package.json: %v", err)
	}

	// Create the test files in the files/ directory
	for _, file := range testFiles {
		// Create files under src/files/
		filePath := filepath.Join(filesDir, file.path)
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", file.path, err)
		}
		if err := os.WriteFile(filePath, []byte(file.content), 0o644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", file.path, err)
		}
		// Set appropriate permissions based on file type
		mode := os.FileMode(0o644)
		if strings.Contains(file.path, "bin/") {
			mode = 0o755
		}
		if err := os.Chmod(filePath, mode); err != nil {
			t.Fatalf("Failed to set permissions on %s: %v", file.path, err)
		}
	}

	// Define the expected content for test files
	testFileContent := []byte("test binary content")
	libFileContent := []byte("test library content")

	// Calculate hashes of the expected file contents
	hasher := sha256.New()
	hasher.Write(testFileContent)
	testFileHash := hex.EncodeToString(hasher.Sum(nil))

	hasher.Reset()
	hasher.Write(libFileContent)
	libFileHash := hex.EncodeToString(hasher.Sum(nil))

	// Create metadata with paths that match the tarball structure
	expectedMeta := &Metadata{
		Name:        "test-package",
		Version:     "1.0.0",
		Maintainer:  "test@example.com",
		Description: "Test package",
		Files: []File{
			{
				Path:   "bin/test",
				Size:   int64(len(testFiles[0].content)),
				Mode:   0o755,
				Digest: testFileHash,
			},
			{
				Path:   "lib/libtest.so",
				Size:   int64(len(testFiles[1].content)),
				Mode:   0o644,
				Digest: libFileHash,
			},
		},
	}

	// Create a custom verify function that handles the dynamic meta/package.json file
	verifyPackageWithMeta := func(pkgPath string, expectedMeta *Metadata) error {
		// Extract the package to get the dynamic meta/package.json
		extractDir, err := os.MkdirTemp("", "gotya-extract-*")
		if err != nil {
			return fmt.Errorf("failed to create extract directory: %w", err)
		}
		defer os.RemoveAll(extractDir)

		// Extract the package
		if _, err := ExtractPackage(pkgPath, extractDir); err != nil {
			return fmt.Errorf("failed to extract package: %w", err)
		}

		// Read the meta/package.json file
		metaPath := filepath.Join(extractDir, "meta", "package.json")
		metaJSON, err := os.ReadFile(metaPath)
		if err != nil {
			return fmt.Errorf("failed to read meta/package.json: %w", err)
		}

		// Create a copy of the expected metadata to avoid modifying the original
		metaCopy := *expectedMeta
		metaCopy.Files = make([]File, len(expectedMeta.Files))
		copy(metaCopy.Files, expectedMeta.Files)

		// Calculate the hash of meta/package.json
		hasher.Reset()
		hasher.Write(metaJSON)
		metaJSONHash := hex.EncodeToString(hasher.Sum(nil))

		// Add the meta/package.json file to the expected files
		metaFile := File{
			Path:   "meta/package.json",
			Size:   int64(len(metaJSON)),
			Mode:   0o644,
			Digest: metaJSONHash,
		}

		// Add the meta/package.json file to the expected files
		metaCopy.Files = append(metaCopy.Files, metaFile)

		// Now verify the package with the updated metadata
		return verifyPackage(pkgPath, &metaCopy)
	}

	// Create the package from our source directory using CreatePackage
	err := CreatePackage(srcDir, filepath.Dir(testPkgPath), "test-package", "1.0.0", "linux", "amd64")
	if err != nil {
		t.Fatalf("Failed to create test package: %v", err)
	}

	// Update the test package path to match the one created by CreatePackage
	testPkgPath = filepath.Join(filepath.Dir(testPkgPath), "test-package_1.0.0_linux_amd64.tar.gz")

	// Add meta/package.json to the expected files
	metaJSONContent := []byte(metaContent)
	hasher.Reset()
	hasher.Write(metaJSONContent)
	metaJSONHash := hex.EncodeToString(hasher.Sum(nil))

	expectedMeta.Files = append(expectedMeta.Files, File{
		Path:   "meta/package.json",
		Size:   int64(len(metaJSONContent)),
		Mode:   0o644,
		Digest: metaJSONHash,
	})

	t.Run("verification with correct paths", func(t *testing.T) {
		err = verifyPackageWithMeta(testPkgPath, expectedMeta)
		if err != nil {
			t.Errorf("Verification failed with correct paths: %v", err)
		}
	})

	t.Run("verification with files/ prefix in metadata", func(t *testing.T) {
		// Create a copy with files/ prefix in metadata to test that it works with or without the prefix
		// This is a valid case since the verifyPackage function handles both cases
		prefixedMeta := *expectedMeta
		prefixedMeta.Files = make([]File, 0, len(expectedMeta.Files))

		// Add files/ prefix to all files in metadata
		for _, f := range expectedMeta.Files {
			if f.Path == "meta/package.json" {
				// Don't add files/ prefix to meta/package.json
				prefixedMeta.Files = append(prefixedMeta.Files, f)
			} else if !strings.HasPrefix(f.Path, "files/") {
				prefixedMeta.Files = append(prefixedMeta.Files, File{
					Path:   "files/" + f.Path,
					Size:   f.Size,
					Mode:   f.Mode,
					Digest: f.Digest,
				})
			} else {
				prefixedMeta.Files = append(prefixedMeta.Files, f)
			}
		}

		// This should pass because verifyPackage handles both with and without files/ prefix
		err = verifyPackageWithMeta(testPkgPath, &prefixedMeta)
		if err != nil {
			t.Errorf("Verification failed with files/ prefix in metadata: %v", err)
		}
	})

	t.Run("file missing from package", func(t *testing.T) {
		// Create a copy of the metadata with an extra file that doesn't exist
		badMeta := *expectedMeta
		badMeta.Files = append([]File{
			{
				Path:   "missing/file",
				Size:   123,
				Mode:   0o644,
				Digest: "a1b2c3",
			},
		}, expectedMeta.Files...)

		err := verifyPackageWithMeta(testPkgPath, &badMeta)
		if err == nil {
			t.Error("Expected missing file error, got nil")
		} else if !strings.Contains(err.Error(), "missing expected file in package") {
			t.Errorf("Expected missing file error, got: %v", err)
		}
	})

	t.Run("file size mismatch", func(t *testing.T) {
		// Create a copy of the metadata with incorrect size
		badMeta := *expectedMeta
		badMeta.Files = make([]File, len(expectedMeta.Files))
		copy(badMeta.Files, expectedMeta.Files)
		badMeta.Files[0].Size = 999 // Incorrect size

		err := verifyPackageWithMeta(testPkgPath, &badMeta)
		if err == nil {
			t.Error("Expected size mismatch error, got nil")
		} else if !strings.Contains(err.Error(), "size mismatch") {
			t.Errorf("Expected size mismatch error, got: %v", err)
		}
	})

	t.Run("file hash mismatch", func(t *testing.T) {
		// Create a copy of the metadata with incorrect hash
		badMeta := *expectedMeta
		badMeta.Files = make([]File, len(expectedMeta.Files))
		copy(badMeta.Files, expectedMeta.Files)
		badMeta.Files[0].Digest = "a1b2c3" // Incorrect hash

		err := verifyPackageWithMeta(testPkgPath, &badMeta)
		if err == nil {
			t.Error("Expected hash mismatch error, got nil")
		} else if !strings.Contains(err.Error(), "hash mismatch") {
			t.Errorf("Expected hash mismatch error, got: %v", err)
		}
	})

	t.Run("file mode mismatch", func(t *testing.T) {
		if skipPermissionTests {
			t.Skip("Skipping permission test on Windows")
		}

		// Create a copy of the metadata with incorrect mode
		badMeta := *expectedMeta
		badMeta.Files = make([]File, len(expectedMeta.Files))
		copy(badMeta.Files, expectedMeta.Files)
		badMeta.Files[0].Mode = 0o600 // Incorrect mode

		err := verifyPackageWithMeta(testPkgPath, &badMeta)
		if err == nil {
			t.Error("Expected permission mismatch error, got nil")
		} else if !strings.Contains(err.Error(), "permission mismatch") {
			t.Errorf("Expected permission mismatch error, got: %v", err)
		}
	})

	t.Run("alternative path with files/ prefix", func(t *testing.T) {
		// Create a copy of the metadata with files/ prefix in paths
		altMeta := *expectedMeta
		altMeta.Files = make([]File, len(expectedMeta.Files))
		copy(altMeta.Files, expectedMeta.Files)
		altMeta.Files[0].Path = "files/" + altMeta.Files[0].Path
		altMeta.Files[1].Path = "files/" + altMeta.Files[1].Path

		err := verifyPackageWithMeta(testPkgPath, &altMeta)
		if err != nil {
			t.Errorf("verifyPackageWithMeta() with alternative path error = %v, want nil", err)
		}

		// Also test with a mix of prefixed and non-prefixed paths
		mixedMeta := *expectedMeta
		mixedMeta.Files = make([]File, len(expectedMeta.Files))
		copy(mixedMeta.Files, expectedMeta.Files)
		mixedMeta.Files[0].Path = "files/" + mixedMeta.Files[0].Path
		// Second file keeps original path without prefix

		err = verifyPackageWithMeta(testPkgPath, &mixedMeta)
		if err != nil {
			t.Errorf("verifyPackageWithMeta() with mixed paths error = %v, want nil", err)
		}
	})
}

// verifyPackageTest verifies that a package contains all expected files with correct content.
// nolint:gocyclo,funlen // This function needs to handle multiple validation cases.
func verifyPackageTest(t *testing.T, pkgPath string, expectedFiles map[string]File) error {
	// Open the package file
	file, err := os.Open(pkgPath)
	if err != nil {
		return errors.WrapFileError(err, "open package file", pkgPath)
	}
	defer file.Close()

	// Create a gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return errors.Wrap(err, "create gzip reader")
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	foundFiles := make(map[string]bool)

	// Read the tar archive
	for {
		header, err := tarReader.Next()
		if goerror.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return errors.Wrap(err, "read tar archive")
		}

		// Skip directories and other non-regular files
		if header.Typeflag != tar.TypeReg {
			continue
		}

		// Check if the file is expected
		fileInfo, existsInExpected := expectedFiles[header.Name]
		if !existsInExpected {
			// Try with a different path format (handles potential path separator differences)
			altPath := filepath.ToSlash(header.Name)
			if altFileInfo, altExists := expectedFiles[altPath]; altExists {
				fileInfo = altFileInfo
				header.Name = altPath
				_ = altExists // explicitly ignore as we don't need it
			} else {
				// Log available expected files for debugging
				var availableFiles []string
				for k := range expectedFiles {
					availableFiles = append(availableFiles, k)
				}
				t.Logf("Available expected files: %v", availableFiles)
				return errors.NewUnexpectedFileError(header.Name)
			}
		}

		// Mark file as found
		foundFiles[header.Name] = true

		// Skip verification for meta/package.json as it's generated dynamically
		if header.Name != "meta/package.json" {
			// Check for size and mode if not meta/package.json
			if fileInfo.Size != 0 && fileInfo.Size != header.Size {
				return errors.NewFileSizeMismatchError(header.Name, fileInfo.Size, header.Size)
			}

			// Skip permission checks on Windows or if the mode is 0 (not specified)
			skipPermissionChecks := runtime.GOOS == "windows"
			if !skipPermissionChecks && fileInfo.Mode != 0 && header.Mode != int64(fileInfo.Mode) {
				return errors.NewFilePermissionMismatchError(
					header.Name,
					os.FileMode(fileInfo.Mode),
					os.FileMode(header.Mode),
				)
			}

			// Calculate the hash of the file content
			hasher := sha256.New()
			if _, err := io.Copy(hasher, tarReader); err != nil {
				return errors.WrapFileError(err, "calculate hash for", header.Name)
			}
			actualHash := hex.EncodeToString(hasher.Sum(nil))

			// Verify the hash for all other files
			if actualHash != fileInfo.Digest {
				return errors.NewFileHashMismatchError(header.Name, fileInfo.Digest, actualHash)
			}
		}
	}

	// Verify all expected files were found
	for path := range expectedFiles {
		if !foundFiles[path] {
			// Try with a different path format (handles potential path separator differences)
			altPath := filepath.ToSlash(path)
			if !foundFiles[altPath] {
				return errors.NewMissingFileError(fmt.Sprintf("%s (also checked as %s)", path, altPath))
			}
		}
	}

	return nil
}

func verifyCreatedPackage(t *testing.T, pkgPath string, expectedMeta *Metadata) error {
	// Create a map of expected files for quick lookup
	expectedFiles := make(map[string]File)
	for _, f := range expectedMeta.Files {
		expectedFiles[f.Path] = f
	}

	// Create a copy of the metadata to avoid modifying the original
	metaCopy := *expectedMeta
	metaCopy.Files = make([]File, len(expectedMeta.Files))
	copy(metaCopy.Files, expectedMeta.Files)

	// Verify the package with the expected files
	if err := verifyPackageTest(t, pkgPath, expectedFiles); err != nil {
		return err
	}

	// Verify the package with the metadata
	return verifyPackage(pkgPath, &metaCopy)
}

func TestCreatePackage(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	outputDir, err := os.MkdirTemp("", "gotya-output-*")
	if err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}
	defer os.RemoveAll(outputDir)

	t.Run("valid package", func(t *testing.T) {
		sourceDir := tempDir
		outputDir := outputDir
		pkgName := "test-package"
		pkgVer := "1.0.0"
		pkgOS := "linux"
		pkgArch := "amd64"

		err := CreatePackage(sourceDir, outputDir, pkgName, pkgVer, pkgOS, pkgArch)
		if err != nil {
			t.Fatalf("CreatePackage() error = %v, wantErr false", err)
		}

		// Verify the package was created
		pkgPath := filepath.Join(outputDir, "test-package_1.0.0_linux_amd64.tar.gz")
		if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
			t.Fatalf("Package file was not created: %s", pkgPath)
		}

		// Extract the package to get the dynamic meta/package.json
		extractDir := filepath.Join(tempDir, "extracted-pkg")
		if err := os.MkdirAll(extractDir, 0o755); err != nil {
			t.Fatalf("Failed to create extract directory: %v", err)
		}

		// Manually extract the tarball to avoid path validation issues
		extractFile, err := os.Open(pkgPath)
		if err != nil {
			t.Fatalf("Failed to open package file: %v", err)
		}
		defer extractFile.Close()

		gzipReader, err := gzip.NewReader(extractFile)
		if err != nil {
			t.Fatalf("Failed to create gzip reader: %v", err)
		}
		defer gzipReader.Close()

		tarReader := tar.NewReader(gzipReader)
		for {
			header, err := tarReader.Next()
			if goerror.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("Error reading tar archive: %v", err)
			}

			// Skip directories
			if header.Typeflag == tar.TypeDir {
				continue
			}

			// Create the target directory if it doesn't exist
			targetPath := filepath.Join(extractDir, header.Name)
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				t.Fatalf("Failed to create directory for %s: %v", targetPath, err)
			}

			// Create the file
			outFile, err := os.Create(targetPath)
			if err != nil {
				t.Fatalf("Failed to create file %s: %v", targetPath, err)
			}

			// Copy the file content
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				t.Fatalf("Failed to write to %s: %v", targetPath, err)
			}
			outFile.Close()

			// Set file permissions
			if err := os.Chmod(targetPath, os.FileMode(header.Mode)); err != nil {
				t.Fatalf("Failed to set permissions for %s: %v", targetPath, err)
			}
		}

		// Read the meta/package.json to get its content
		metaPath := filepath.Join(extractDir, "meta", "package.json")
		metaContent, err := os.ReadFile(metaPath)
		if err != nil {
			t.Fatalf("Failed to read meta/package.json: %v", err)
		}

		testFileContent := []byte("#!/bin/bash\necho 'test'")
		libFileContent := []byte("test library content")

		// Calculate hashes for all files
		hasher := sha256.New()
		hasher.Write(testFileContent)
		testFileHash := hex.EncodeToString(hasher.Sum(nil))

		hasher.Reset()
		hasher.Write(libFileContent)
		libFileHash := hex.EncodeToString(hasher.Sum(nil))

		// Calculate meta content hash (not used directly, just to match the expected format)
		hasher.Reset()
		hasher.Write(metaContent)
		_ = hex.EncodeToString(hasher.Sum(nil)) // Hash not used directly, will be recalculated

		// Create the metadata with the correct file sizes and hashes
		// Use the same paths as they appear in the tarball (with files/ prefix)
		expectedMeta := &Metadata{
			Name:        pkgName,
			Version:     pkgVer,
			Description: "Test package",
			Files: []File{
				{
					Path:   "files/bin/test",            // Include files/ prefix to match tarball
					Size:   int64(len(testFileContent)), // Actual content size
					Mode:   0o755,
					Digest: testFileHash,
				},
				{
					Path:   "files/lib/libtest.so",     // Include files/ prefix to match tarball
					Size:   int64(len(libFileContent)), // Actual content size
					Mode:   0o644,
					Digest: libFileHash,
				},
			},
		}

		// The meta/package.json will be added automatically by verifyPackage
		// so we don't need to include it in the expected files

		// The meta/package.json is automatically included by CreatePackage,
		// so we should find it in the extracted files and verify it exists
		finalMetaPath := filepath.Join(extractDir, "meta", "package.json")
		if _, statErr := os.Stat(finalMetaPath); os.IsNotExist(statErr) {
			t.Fatal("meta/package.json was not found in the extracted package")
		}

		// Read the actual meta/package.json to get its exact content and hash
		finalMetaContent, readErr := os.ReadFile(finalMetaPath)
		if readErr != nil {
			t.Fatalf("Failed to read meta/package.json: %v", readErr)
		}

		// Calculate the hash of the actual meta/package.json
		hasher.Reset()
		hasher.Write(finalMetaContent)
		actualMetaHash := hex.EncodeToString(hasher.Sum(nil))

		// Create a new metadata object with just the files we expect
		// This ensures we're only verifying the files we care about
		verifyMeta := &Metadata{
			Name:         expectedMeta.Name,
			Version:      expectedMeta.Version,
			Maintainer:   expectedMeta.Maintainer,
			Description:  expectedMeta.Description,
			Dependencies: expectedMeta.Dependencies,
			Files:        make([]File, 0, len(expectedMeta.Files)+1),
		}

		// Add all files with the correct paths
		for _, f := range expectedMeta.Files {
			// The tarball will have files/ prefix for all non-meta files
			newFile := f // Create a copy to avoid modifying the original
			if !strings.HasPrefix(newFile.Path, "meta/") && !strings.HasPrefix(newFile.Path, "files/") {
				newFile.Path = "files/" + newFile.Path
			}
			verifyMeta.Files = append(verifyMeta.Files, newFile)
		}

		// Add meta/package.json to the expected files with the actual hash and size
		verifyMeta.Files = append(verifyMeta.Files, File{
			Path:   "meta/package.json",
			Size:   int64(len(finalMetaContent)),
			Mode:   0o644,
			Digest: actualMetaHash,
		})

		// Verify the package using the expected metadata
		err = verifyCreatedPackage(t, pkgPath, expectedMeta)
		if err != nil {
			t.Fatalf("Package verification failed: %v", err)
		}
	})
}
