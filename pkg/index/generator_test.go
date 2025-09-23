package index

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cperrin88/gotya/pkg/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LoadIndex is a helper function to load an index from a file
func LoadIndex(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var index Index
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, err
	}
	return &index, nil
}

func TestGenerator_Generate(t *testing.T) {
	// Create a temporary directory for test artifacts
	tempDir, err := os.MkdirTemp("", "gotya-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a test artifact
	artifactsDir := filepath.Join(tempDir, "artifacts")
	err = os.MkdirAll(artifactsDir, 0o755)
	require.NoError(t, err)

	// Create a test artifact file
	artifactPath := filepath.Join(artifactsDir, "test-artifact.gotya")
	createTestArtifact(t, artifactPath, &artifact.Metadata{
		Name:        "test-tool",
		Version:     "1.0.0",
		Description: "A test tool",
		OS:          "linux",
		Arch:        "amd64",
	})

	tests := []struct {
		name       string
		setup      func(t *testing.T) (*Generator, string)
		wantErr    bool
		assertions func(t *testing.T, index *Index)
	}{
		{
			name: "successful generation",
			setup: func(t *testing.T) (*Generator, string) {
				outputPath := filepath.Join(tempDir, "index.json")
				return &Generator{
					Dir:           artifactsDir,
					OutputPath:    outputPath,
					FormatVersion: "1",
				}, outputPath
			},
			wantErr: false,
			assertions: func(t *testing.T, index *Index) {
				assert.Equal(t, "1", index.FormatVersion)
				require.Len(t, index.Artifacts, 1)
				art := index.Artifacts[0]
				assert.Equal(t, "test-tool", art.Name)
				assert.Equal(t, "1.0.0", art.Version)
				assert.Equal(t, "A test tool", art.Description)
				assert.Equal(t, "linux", art.OS)
				assert.Equal(t, "amd64", art.Arch)
				assert.NotEmpty(t, art.URL)
				assert.NotZero(t, art.Size)
				assert.NotEmpty(t, art.Checksum)
			},
		},
		{
			name: "with base path",
			setup: func(t *testing.T) (*Generator, string) {
				outputPath := filepath.Join(tempDir, "index.json")
				return &Generator{
					Dir:           artifactsDir,
					OutputPath:    outputPath,
					FormatVersion: "1",
					BasePath:      "packages",
				}, outputPath
			},
			wantErr: false,
			assertions: func(t *testing.T, index *Index) {
				require.Len(t, index.Artifacts, 1)
				assert.Contains(t, index.Artifacts[0].URL, "packages/")
			},
		},
		{
			name: "non-existent directory",
			setup: func(t *testing.T) (*Generator, string) {
				outputPath := filepath.Join(tempDir, "index.json")
				return &Generator{
					Dir:           "/non/existent/directory",
					OutputPath:    outputPath,
					FormatVersion: "1",
				}, outputPath
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator, outputPath := tt.setup(t)

			err := generator.Generate(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify the index file was created
			_, err = os.Stat(outputPath)
			assert.NoError(t, err)

			// Load and verify the index
			index, err := LoadIndex(outputPath)
			require.NoError(t, err)

			if tt.assertions != nil {
				tt.assertions(t, index)
			}
		})
	}
}

func TestGenerator_describeArtifact(t *testing.T) {
	tempFile, err := os.CreateTemp("", "test-artifact-*.gotya")
	require.NoError(t, err)
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	generator := &Generator{
		OutputPath: filepath.Join(os.TempDir(), "index.json"),
	}

	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantErr bool
	}{
		{
			name: "valid artifact",
			setup: func(t *testing.T) string {
				path := filepath.Join(os.TempDir(), "valid-artifact.gotya")
				createTestArtifact(t, path, &artifact.Metadata{
					Name:    "test-tool",
					Version: "1.0.0",
				})
				return path
			},
			wantErr: false,
		},
		{
			name: "non-existent file",
			setup: func(t *testing.T) string {
				return "/non/existent/file.gotya"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			_, err := generator.describeArtifact(context.Background(), path)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGenerator_makeURL(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) (*Generator, string)
		want    string
		wantErr bool
	}{
		{
			name: "relative path without base",
			setup: func(t *testing.T) (*Generator, string) {
				tempDir := t.TempDir()
				outputPath := filepath.Join(tempDir, "index.json")
				return &Generator{
					OutputPath: outputPath,
				}, filepath.Join(tempDir, "artifacts", "test.gotya")
			},
			want: "artifacts/test.gotya",
		},
		{
			name: "with base path",
			setup: func(t *testing.T) (*Generator, string) {
				tempDir := t.TempDir()
				outputPath := filepath.Join(tempDir, "index.json")
				return &Generator{
					OutputPath: outputPath,
					BasePath:   "packages",
				}, filepath.Join(tempDir, "artifacts", "test.gotya")
			},
			want: "packages/artifacts/test.gotya",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator, artifactPath := tt.setup(t)
			got, err := generator.makeURL(artifactPath)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// createTestArtifact creates a test artifact file with the given metadata
func createTestArtifact(t *testing.T, path string, md *artifact.Metadata) {
	tempDir := t.TempDir()

	// Create meta directory and artifact.json
	metaDir := filepath.Join(tempDir, "meta")
	err := os.MkdirAll(metaDir, 0o755)
	require.NoError(t, err)

	metaFile, err := os.Create(filepath.Join(metaDir, "artifact.json"))
	require.NoError(t, err)
	err = json.NewEncoder(metaFile).Encode(md)
	require.NoError(t, err)
	metaFile.Close()

	// Create a simple file in the archive
	contentFile, err := os.Create(filepath.Join(tempDir, "content.txt"))
	require.NoError(t, err)
	_, err = contentFile.WriteString("test content")
	require.NoError(t, err)
	contentFile.Close()

	// Create the output directory if it doesn't exist
	if filepath.Dir(path) != "." {
		err = os.MkdirAll(filepath.Dir(path), 0o755)
		require.NoError(t, err)
	}

	// Create a zip archive
	zipFile, err := os.Create(path)
	require.NoError(t, err)
	defer zipFile.Close()

	// Create a zip writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Walk the temp directory and add files to the zip
	err = filepath.Walk(tempDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get the relative path from tempDir
		relPath, err := filepath.Rel(tempDir, filePath)
		if err != nil {
			return err
		}

		// Create a new file in the zip
		zipEntry, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		// Copy the file content
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		_, err = zipEntry.Write(fileContent)
		return err
	})
	require.NoError(t, err)
}
