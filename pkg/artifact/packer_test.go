package artifact

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPacker_Pack(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T) (string, string) // returns (inputDir, outputDir)
		packer       *Packer
		expectedErr  error
		expectOutput bool
	}{
		{
			name: "successful package creation",
			setup: func(t *testing.T) (string, string) {
				tempDir := t.TempDir()
				inputDir := filepath.Join(tempDir, "input")
				outputDir := filepath.Join(tempDir, "output")

				// Create input directory structure
				err := os.MkdirAll(filepath.Join(inputDir, artifactDataDir), 0755)
				require.NoError(t, err)
				err = os.MkdirAll(filepath.Join(inputDir, artifactMetaDir), 0755)
				require.NoError(t, err)

				// Create a sample file in data directory
				err = os.WriteFile(filepath.Join(inputDir, "data", "test.txt"), []byte("test content"), 0644)
				require.NoError(t, err)

				// Create a hook script
				err = os.WriteFile(filepath.Join(inputDir, "meta", "pre-install.tengo"), []byte("// pre-install hook"), 0644)
				require.NoError(t, err)

				return inputDir, outputDir
			},
			packer: &Packer{
				name:        "test-package",
				version:     "1.0.0",
				os:          "linux",
				arch:        "amd64",
				maintainer:  "test@example.com",
				description: "Test package",
				hooks:       map[string]string{"pre-install": "pre-install.tengo"},
			},
			expectedErr:  nil,
			expectOutput: true,
		},
		{
			name: "missing input directory",
			setup: func(t *testing.T) (string, string) {
				tempDir := t.TempDir()
				return filepath.Join(tempDir, "nonexistent"), filepath.Join(tempDir, "output")
			},
			packer: &Packer{
				name: "test-package",
			},
			expectedErr:  errors.ErrInvalidPath,
			expectOutput: false,
		},
		{
			name: "invalid file in root directory",
			setup: func(t *testing.T) (string, string) {
				tempDir := t.TempDir()
				inputDir := filepath.Join(tempDir, "input")
				err := os.MkdirAll(inputDir, 0755)
				require.NoError(t, err)

				// Create an invalid file in root
				err = os.WriteFile(filepath.Join(inputDir, "invalid.txt"), []byte("invalid"), 0644)
				require.NoError(t, err)

				return inputDir, filepath.Join(tempDir, "output")
			},
			packer: &Packer{
				name: "test-package",
			},
			expectedErr:  errors.ErrInvalidPath,
			expectOutput: false,
		},
		{
			name: "unreferenced hook script",
			setup: func(t *testing.T) (string, string) {
				tempDir := t.TempDir()
				inputDir := filepath.Join(tempDir, "input")
				err := os.MkdirAll(filepath.Join(inputDir, artifactMetaDir), 0755)
				require.NoError(t, err)

				// Create an unreferenced hook script
				err = os.WriteFile(filepath.Join(inputDir, artifactMetaDir, "unreferenced.tengo"), []byte("// unreferenced"), 0644)
				require.NoError(t, err)

				return inputDir, filepath.Join(tempDir, "output")
			},
			packer: &Packer{
				name:  "test-package",
				hooks: map[string]string{},
			},
			expectedErr:  errors.ErrInvalidPath,
			expectOutput: false,
		},
		{
			name: "metadata only package without data directory",
			setup: func(t *testing.T) (string, string) {
				tempDir := t.TempDir()
				inputDir := filepath.Join(tempDir, "input")
				outputDir := filepath.Join(tempDir, "output")

				// Only create meta directory, no data directory
				err := os.MkdirAll(filepath.Join(inputDir, artifactMetaDir), 0755)
				require.NoError(t, err)

				// Create a hook script
				err = os.WriteFile(
					filepath.Join(inputDir, artifactMetaDir, "pre-install.tengo"),
					[]byte("// pre-install hook"),
					0644,
				)
				require.NoError(t, err)

				return inputDir, outputDir
			},
			packer: &Packer{
				name:        "metadata-only-pkg",
				version:     "1.0.0",
				os:          "linux",
				arch:        "amd64",
				maintainer:  "test@example.com",
				description: "Metadata only test package",
				hooks:       map[string]string{"pre-install": "pre-install.tengo"},
			},
			expectedErr:  nil,
			expectOutput: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputDir, outputDir := tt.setup(t)

			// Create output directory
			err := os.MkdirAll(outputDir, 0755)
			require.NoError(t, err)

			// Set up packer with test directories
			p := tt.packer
			p.inputDir = inputDir
			p.outputDir = outputDir

			// Run the pack function
			err = p.Pack()

			// Check the error
			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			// Check if output file was created if expected
			outputFile := filepath.Join(outputDir, p.name+"_"+p.version+"_"+p.os+"_"+p.arch+".gotya")
			_, statErr := os.Stat(outputFile)
			if tt.expectOutput {
				assert.NoError(t, statErr, "expected output file to exist")
			} else {
				assert.ErrorIs(t, statErr, os.ErrNotExist)
			}
		})
	}
}

// TestPackSymlinks tests handling of symlinks during packaging
func TestPackSymlinks(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T) (string, string, string)
		expectErr  bool
		errMessage string
	}{
		{
			name: "valid relative symlink",
			setup: func(t *testing.T) (string, string, string) {
				tempDir := t.TempDir()
				inputDir := filepath.Join(tempDir, "input")
				outputDir := filepath.Join(tempDir, "output")

				// Create input directory structure
				err := os.MkdirAll(filepath.Join(inputDir, "data"), 0755)
				require.NoError(t, err)

				// Create a target file
				targetFile := filepath.Join(inputDir, "data", "target.txt")
				err = os.WriteFile(targetFile, []byte("target content"), 0644)
				require.NoError(t, err)

				// Create a relative symlink (target must be relative to current working directory because
				// copyInputDir resolves symlink targets using filepath.Abs(target))
				symlinkPath := filepath.Join(inputDir, "data", "link.txt")
				cwd, cwdErr := os.Getwd()
				require.NoError(t, cwdErr)
				relFromCwd, relErr := filepath.Rel(cwd, targetFile)
				require.NoError(t, relErr)
				err = os.Symlink(relFromCwd, symlinkPath)
				require.NoError(t, err)

				return inputDir, outputDir, symlinkPath
			},
			expectErr: false,
		},
		{
			name: "absolute symlink",
			setup: func(t *testing.T) (string, string, string) {
				tempDir := t.TempDir()
				inputDir := filepath.Join(tempDir, "input")
				outputDir := filepath.Join(tempDir, "output")

				// Create input directory structure
				err := os.MkdirAll(filepath.Join(inputDir, "data"), 0755)
				require.NoError(t, err)

				// Create a target file
				targetFile := filepath.Join(inputDir, "data", "target.txt")
				err = os.WriteFile(targetFile, []byte("target content"), 0644)
				require.NoError(t, err)

				// Create an absolute symlink (should fail)
				symlinkPath := filepath.Join(inputDir, "data", "link.txt")
				err = os.Symlink(targetFile, symlinkPath)
				require.NoError(t, err)

				return inputDir, outputDir, symlinkPath
			},
			expectErr:  true,
			errMessage: "is absolute",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputDir, outputDir, _ := tt.setup(t)

			p := &Packer{
				name:        "test-package",
				version:     "1.0.0",
				os:          "linux",
				arch:        "amd64",
				maintainer:  "test@example.com",
				description: "Test package",
				inputDir:    inputDir,
				outputDir:   outputDir,
			}

			// Ensure output directory exists
			require.NoError(t, os.MkdirAll(outputDir, 0o755))

			err := p.Pack()

			if tt.expectErr {
				assert.Error(t, err)
				// New error semantics: ensure it wraps ErrInvalidPath
				assert.ErrorIs(t, err, errors.ErrInvalidPath)
				// Keep a minimal message check to ensure context is present when provided
				if tt.errMessage != "" {
					assert.Contains(t, err.Error(), tt.errMessage)
				}
			} else {
				assert.NoError(t, err)

				// Verify the output file was created (underscored naming)
				outputFile := filepath.Join(outputDir, p.name+"_"+p.version+"_"+p.os+"_"+p.arch+".gotya")
				_, err := os.Stat(outputFile)
				assert.NoError(t, err, "output file should exist")
			}
		})
	}
}
