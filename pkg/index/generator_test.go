package index

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cperrin88/gotya/pkg/artifact"
	"github.com/cperrin88/gotya/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestArtifact creates a test artifact file with the given metadata using the packer
func createTestArtifact(t *testing.T, path string, md *artifact.Metadata) {
	t.Helper()

	// Create a temporary directory for the packer input
	tempDir := t.TempDir()

	// Create empty meta and data directories (packer will create the metadata file)
	err := os.MkdirAll(filepath.Join(tempDir, "meta"), 0o755)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(tempDir, "data"), 0o755)
	require.NoError(t, err)

	// Use the packer to create the artifact
	packer := artifact.NewPacker(
		md.Name,
		md.Version,
		md.OS,
		md.Arch,
		md.Maintainer,
		md.Description,
		md.Dependencies,
		md.Hooks,
		tempDir,
		filepath.Dir(path),
	)

	outputPath, err := packer.Pack()
	require.NoError(t, err)

	// Move the created artifact to the desired path
	err = os.Rename(outputPath, path)
	require.NoError(t, err)
}

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
	tempDir := t.TempDir()

	// Create a test artifact
	artifactsDir := filepath.Join(tempDir, "artifacts")
	err := os.MkdirAll(artifactsDir, 0o755)
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
		setup      func() (*Generator, string)
		wantErr    bool
		assertions func(t *testing.T, index *Index)
	}{
		{
			name: "successful generation",
			setup: func() (*Generator, string) {
				outputPath := filepath.Join(tempDir, "index.json")
				return &Generator{
					Dir:        artifactsDir,
					OutputPath: outputPath,
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
			setup: func() (*Generator, string) {
				outputPath := filepath.Join(tempDir, "index-with-base.json")
				return &Generator{
					Dir:            artifactsDir,
					OutputPath:     outputPath,
					BasePath:       "packages",
					ForceOverwrite: true, // Allow overwrite for test
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
			setup: func() (*Generator, string) {
				outputPath := filepath.Join(tempDir, "index.json")
				return &Generator{
					Dir:        "/non/existent/directory",
					OutputPath: outputPath,
				}, outputPath
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator, outputPath := tt.setup()

			err := generator.Generate(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify the index file was created
			_, err = os.Stat(outputPath)
			require.NoError(t, err)

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
	generator := &Generator{
		OutputPath: filepath.Join(os.TempDir(), "index.json"),
	}

	tests := []struct {
		name    string
		setup   func() string
		wantErr bool
	}{
		{
			name: "valid artifact",
			setup: func() string {
				path := filepath.Join(os.TempDir(), "valid-artifact.gotya")
				// Create a valid test artifact
				createTestArtifact(t, path, &artifact.Metadata{
					Name:        "valid-artifact",
					Version:     "1.0.0",
					OS:          "linux",
					Arch:        "amd64",
					Description: "Valid test artifact",
				})
				return path
			},
			wantErr: false,
		},
		{
			name: "non-existent file",
			setup: func() string {
				return "/non/existent/file.gotya"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			_, err := generator.describeArtifact(context.Background(), path)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGenerator_WithBaseline(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()

	// Create a test artifact in the temp directory
	testArtifactPath := filepath.Join(tempDir, "test.gotya")
	createSimpleTestArtifact(t, testArtifactPath, "test-artifact", "1.0.0")

	// Create a baseline index file
	baselineIndexPath := filepath.Join(tempDir, "baseline.json")
	createTestIndex(t, baselineIndexPath, []*model.IndexArtifactDescriptor{
		{
			Name:        "existing-artifact",
			Version:     "1.0.0",
			Description: "Existing artifact in baseline",
			URL:         "existing-artifact-1.0.0.gotya",
			Checksum:    "abc123",
			Size:        1024,
		},
	})

	// Create a generator with baseline
	outputPath := filepath.Join(tempDir, "output.json")
	generator := NewGenerator(tempDir, outputPath).WithBaseline(baselineIndexPath)

	// Generate the index
	err := generator.Generate(context.Background())
	require.NoError(t, err)

	// Verify the output file exists
	_, err = os.Stat(outputPath)
	require.NoError(t, err)

	// Load and verify the generated index
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	var result Index
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	// Should contain both the baseline artifact and the new one
	assert.Len(t, result.Artifacts, 2, "should contain both baseline and new artifact")

	// Verify the artifacts are present
	foundExisting := false
	foundNew := false
	for _, a := range result.Artifacts {
		if a.Name == "existing-artifact" && a.Version == "1.0.0" {
			foundExisting = true
			assert.Equal(t, "Existing artifact in baseline", a.Description)
		}
		if a.Name == "test-artifact" && a.Version == "1.0.0" {
			foundNew = true
		}
	}

	assert.True(t, foundExisting, "should contain the baseline artifact")
	assert.True(t, foundNew, "should contain the new artifact")
}

func TestGenerator_WithBaseline_Conflict(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()

	// Create a baseline index file with an artifact
	baselineIndexPath := filepath.Join(tempDir, "baseline.json")
	baselineArtifact := &model.IndexArtifactDescriptor{
		Name:        "conflict-artifact",
		Version:     "1.0.0",
		Description: "Test artifact conflict-artifact",
		URL:         "conflict-artifact-1.0.0.gotya",
		OS:          "linux",
		Arch:        "amd64",
	}

	// Create the test artifact with different metadata to cause a conflict
	testArtifactPath := filepath.Join(tempDir, "conflict.gotya")
	createTestArtifact(t, testArtifactPath, &artifact.Metadata{
		Name:        baselineArtifact.Name,
		Version:     baselineArtifact.Version,
		Description: "Different description to cause conflict", // This is different from baseline
		OS:          baselineArtifact.OS,
		Arch:        baselineArtifact.Arch,
	})

	// Get the actual checksum and size of the created artifact
	fileInfo, err := os.Stat(testArtifactPath)
	require.NoError(t, err)
	baselineArtifact.Size = fileInfo.Size()

	checksum, err := sha256File(testArtifactPath)
	require.NoError(t, err)
	baselineArtifact.Checksum = checksum

	// Create the baseline index with the updated artifact
	createTestIndex(t, baselineIndexPath, []*model.IndexArtifactDescriptor{baselineArtifact})

	// Create a generator with baseline
	outputPath := filepath.Join(tempDir, "output.json")
	generator := NewGenerator(tempDir, outputPath).WithBaseline(baselineIndexPath)

	// Generate the index - should fail with conflict
	err = generator.Generate(context.Background())
	require.Error(t, err)
	// Check if the error contains the expected error message
	assert.Contains(t, err.Error(), "conflict for artifact", "should return index conflict error")
}

func TestGenerator_WithBaseline_NoNewArtifacts(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()

	// Create a baseline index file
	baselineIndexPath := filepath.Join(tempDir, "baseline.json")
	createTestIndex(t, baselineIndexPath, []*model.IndexArtifactDescriptor{
		{
			Name:        "baseline-only",
			Version:     "1.0.0",
			Description: "Artifact only in baseline",
			URL:         "baseline-only-1.0.0.gotya",
			Checksum:    "abc123",
			Size:        1024,
		},
	})

	// Create an empty directory for the generator
	tempDir2 := t.TempDir()

	// Create a generator with baseline but no new artifacts
	outputPath := filepath.Join(tempDir, "output.json")
	generator := NewGenerator(tempDir2, outputPath).WithBaseline(baselineIndexPath)

	// Generate the index - should succeed with just the baseline artifacts
	err := generator.Generate(context.Background())
	require.NoError(t, err)

	// Verify the output file exists
	_, err = os.Stat(outputPath)
	require.NoError(t, err)

	// Load and verify the generated index
	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	var result Index
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	// Should contain only the baseline artifact
	assert.Len(t, result.Artifacts, 1, "should contain only the baseline artifact")
	assert.Equal(t, "baseline-only", result.Artifacts[0].Name)
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
			want: "test.gotya",
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
			want: "packages/test.gotya",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator, artifactPath := tt.setup(t)
			got, err := generator.makeURL(artifactPath)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
func createTestIndex(t *testing.T, path string, artifacts []*model.IndexArtifactDescriptor) {
	index := Index{
		FormatVersion: CurrentFormatVersion,
		LastUpdate:    time.Now(),
		Artifacts:     artifacts,
	}

	data, err := json.MarshalIndent(index, "", "  ")
	require.NoError(t, err)

	err = os.WriteFile(path, data, 0o644)
	require.NoError(t, err)
}

// createSimpleTestArtifact creates a simple test artifact with the given name and version
func createSimpleTestArtifact(t *testing.T, path, name, version string) {
	t.Helper()
	createTestArtifact(t, path, &artifact.Metadata{
		Name:    name,
		Version: version,
		OS:      "linux",
		Arch:    "amd64",
	})
}
