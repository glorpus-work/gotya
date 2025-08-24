package pkg

import (
	"os"
	"path"
	"path/filepath"
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

func TestCreatePackage(t *testing.T) {
	tests := []struct {
		name        string
		packageName string
		setup       func(t *testing.T, packageName string) (string, string, string) // sourceDir, outputDir, pkgName, cleanup
		wantErr     bool
		errContains string
	}{{
		name:        "successful package creation",
		packageName: "test01",
		setup: func(t *testing.T, packageName string) (string, string, string) {
			tempDir := prepareTestPackage(t, packageName, []*testFile{
				{path: "files/foo/bar.txt", content: "test content"},
			})
			outputDir := filepath.Join(tempDir, "output")
			require.NoError(t, os.Mkdir(outputDir, 0755))
			return tempDir, outputDir, "test-pkg"
		},
		wantErr: false,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, outputDir, pkgName := tt.setup(t, tt.packageName)

			sourceDir := filepath.Join(tempDir, tt.packageName)

			packagePath, err := CreatePackage(sourceDir, outputDir, pkgName, "1.0.0", "linux", "amd64")

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err, "CreatePackage should not return an error")

				// Verify the package file exists
				expectedPackagePath := filepath.Join(outputDir, pkgName+"_1.0.0_linux_amd64.tar.gz")
				require.NoErrorf(t, err, "Package file %s should exist", expectedPackagePath)
				require.FileExists(t, packagePath)
				require.Equal(t, expectedPackagePath, packagePath)
			}
		})
	}
}
