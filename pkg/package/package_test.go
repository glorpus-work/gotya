package pkg

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func setupTestEnvironment(t *testing.T) (string, func()) {
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
		if err := os.MkdirAll(dir, 0750); err != nil {
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
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			t.Fatalf("Failed to create parent directory for %s: %v", path, err)
		}
		// On Windows, we need to ensure the file is closed and handles are released
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
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
				if info.Mode()&0200 == 0 {
					if err := os.Chmod(path, 0666); err != nil {
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
				if err := os.WriteFile(filepath.Join(tempDir, "invalid.json"), []byte("{invalid}"), 0644); err != nil {
					t.Fatalf("Failed to create invalid.json: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			if tt.teardown != nil {
				defer tt.teardown()
			}

			got, err := readPackageMetadata(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("readPackageMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkFn != nil && !tt.checkFn(got) {
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

func TestCreatePackage(t *testing.T) {
	tempDir, cleanup := setupTestEnvironment(t)
	defer cleanup()

	outputDir, err := os.MkdirTemp("", "gotya-output-*")
	if err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	tests := []struct {
		name      string
		sourceDir string
		outputDir string
		pkgName   string
		pkgVer    string
		pkgOS     string
		pkgArch   string
		wantErr   bool
	}{
		{
			name:      "valid package",
			sourceDir: tempDir,
			outputDir: outputDir,
			pkgName:   "test-package",
			pkgVer:    "1.0.0",
			pkgOS:     "linux",
			pkgArch:   "amd64",
			wantErr:   false,
		},
		{
			name:      "nonexistent source dir",
			sourceDir: "/nonexistent/dir",
			outputDir: outputDir,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CreatePackage(
				tt.sourceDir,
				tt.outputDir,
				tt.pkgName,
				tt.pkgVer,
				tt.pkgOS,
				tt.pkgArch,
			)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreatePackage() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// Verify package file was created
				pkgFile := filepath.Join(tt.outputDir,
					fmt.Sprintf("%s_%s_%s_%s.tar.gz",
						tt.pkgName,
						tt.pkgVer,
						tt.pkgOS,
						tt.pkgArch))

				if _, err := os.Stat(pkgFile); os.IsNotExist(err) {
					t.Errorf("Package file was not created: %s", pkgFile)
				}
			}
		})
	}
}
