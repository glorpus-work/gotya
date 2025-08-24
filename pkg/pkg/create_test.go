package pkg

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testFile struct {
	path    string
	content string
	isDir   bool
}

func prepareTestPackage(t *testing.T, packageName string, files []*testFile) string {
	t.Helper()

	tempDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tempDir, packageName), DirModeSecure); err != nil {
		return ""
	}
	for _, file := range files {
		fullPath := path.Join(tempDir, packageName, file.path)
		if file.isDir {
			if err := os.MkdirAll(fullPath, DirModeSecure); err != nil {
				return ""
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fullPath), DirModeSecure); err != nil {
			return ""
		}

		if err := os.WriteFile(fullPath, []byte(file.content), FileModeSecure); err != nil {
			return ""
		}

	}

	return tempDir
}

// verifyPackageFiles verifies that all expected files exist in the pkg archive and have the correct content.
// It returns an error if any file is missing, has incorrect content, or if there are extra files.
func verifyPackageFiles(t *testing.T, pkgPath string, expectedFiles []*testFile) error {
	t.Helper()

	// Track which files we've found
	foundFiles := make(map[string]bool)
	expectedFileMap := make(map[string]*testFile)

	// Create a map of expected files for easier lookup
	for _, f := range expectedFiles {
		if !f.isDir {
			expectedFileMap[filepath.ToSlash(f.path)] = f
		}
	}

	// Open the pkg file
	file, err := os.Open(pkgPath)
	if err != nil {
		return fmt.Errorf("failed to open pkg file: %w", err)
	}
	defer file.Close()

	// Create a gzip reader
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	// Create a tar reader
	tr := tar.NewReader(gzr)

	// Process each file in the archive
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("error reading tar header: %w", err)
		}

		// Skip directories and special files
		if header.Typeflag != tar.TypeReg {
			continue
		}

		// Read the file content
		content, err := io.ReadAll(tr)
		if err != nil {
			return fmt.Errorf("error reading file %s from archive: %w", header.Name, err)
		}

		// Check if this is an expected file
		expectedFile, expected := expectedFileMap[header.Name]
		if expected {
			// Verify the content matches
			if string(content) != expectedFile.content {
				return fmt.Errorf("file %s has unexpected content\nExpected: %q\nGot: %q",
					header.Name, expectedFile.content, string(content))
			}
			foundFiles[header.Name] = true
		} else if !strings.HasPrefix(header.Name, "meta/") {
			// Skip files in meta directory, but check others
			return fmt.Errorf("unexpected file found in pkg: %s", header.Name)
		}
	}

	// Verify all expected files were found
	for pkgFilePath, _ := range expectedFileMap {
		if !foundFiles[pkgFilePath] {
			return fmt.Errorf("expected file not found in pkg: %s", pkgFilePath)
		}
	}

	return nil
}

func TestCreatePackage(t *testing.T) {
	tests := []struct {
		name         string
		files        []*testFile
		maintainer   string
		description  string
		dependencies []string
		hooks        map[string]string
		wantErr      bool
		errContains  string
	}{{
		name: "successful pkg creation with all metadata",
		files: []*testFile{
			{path: "files/foo/bar.txt", content: "test content"},
			{path: "meta/post-install.tengo", content: "test content"},
		},
		maintainer:   "test@example.com",
		description:  "Test pkg description",
		dependencies: []string{"dep1", "dep2"},
		hooks:        map[string]string{"post-install": "post-install.tengo"},
		wantErr:      false,
	}, {
		name:    "successful pkg creation with minimal metadata",
		files:   []*testFile{{path: "files/foo/bar.txt", content: "test content"}},
		wantErr: false,
	}, {
		name:        "error on empty source directory",
		wantErr:     true,
		errContains: "invalid pkg structure",
	}, {
		name: "unreferenced hooks script",
		files: []*testFile{
			{path: "files/foo/bar.txt", content: "test content"},
			{path: "meta/post-install.tengo", content: "test content"},
		},
		wantErr:     true,
		errContains: "is not empty but no hooks are defined",
	}, {
		name: "referenced hooks script missing",
		files: []*testFile{
			{path: "files/foo/bar.txt", content: "test content"},
		},
		hooks:       map[string]string{"post-install": "post-install.tengo"},
		wantErr:     true,
		errContains: "hooks are defined but meta directory",
	}, {
		name: "pkg.json present",
		files: []*testFile{
			{path: "meta/pkg.json", content: "stuff"},
			{path: "files/foo/bar.txt", content: "test content"},
		},
		wantErr:     true,
		errContains: "invalid file in meta directory: pkg.json",
	}, {
		name: "file in wrong folder",
		files: []*testFile{
			{path: "files/foo/bar.txt", content: "test content"},
			{path: "files2/foo/bar.txt", content: "test content"},
		},
		wantErr:     true,
		errContains: "unexpected file: files2/foo/bar.txt",
	}, {
		name: "missing files directory",
		files: []*testFile{
			{path: "meta/post-install.tengo", content: "test content"},
		},
		hooks:       map[string]string{"post-install": "post-install.tengo"},
		wantErr:     true,
		errContains: "missing required directory: files",
	}, {
		name: "single file in root",
		files: []*testFile{
			{path: "test.txt", content: "test content"},
		},
		wantErr: true,
	}, {
		name: "nested directory structure",
		files: []*testFile{
			{path: "files/dir2/test.txt", content: "nested file"},
			{path: "files/dir1/another.txt", content: "another file"},
		},
		wantErr: false,
	}, {
		name: "empty directories",
		files: []*testFile{
			{path: "files/empty_dir", isDir: true},
			{path: "files/empty_dir/nested_empty", isDir: true},
		},
		wantErr:     true,
		errContains: "no files found in the files directory",
	}}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkgName := fmt.Sprintf("test%d", i)

			tempDir := prepareTestPackage(t, pkgName, tt.files)

			outputDir := filepath.Join(tempDir, "output")
			sourceDir := filepath.Join(tempDir, pkgName)

			packagePath, err := CreatePackage(
				sourceDir,
				outputDir,
				pkgName,
				"1.0.0",
				"linux",
				"amd64",
				tt.maintainer,
				tt.description,
				tt.dependencies,
				tt.hooks,
			)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err, "CreatePackage should not return an error")

				// Verify the pkg file exists
				expectedPackagePath := filepath.Join(outputDir, pkgName+"_1.0.0_linux_amd64.tar.gz")
				require.NoErrorf(t, err, "Package file %s should exist", expectedPackagePath)
				require.FileExists(t, packagePath)
				require.Equal(t, expectedPackagePath, packagePath)
				err := verifyPackageFiles(t, packagePath, tt.files)
				require.NoError(t, err)
			}
		})
	}
}
