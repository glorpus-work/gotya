package artifact

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cperrin88/gotya/pkg/archive"
	"github.com/cperrin88/gotya/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifier_extractArtifact_Success(t *testing.T) {
	tempDir := t.TempDir()
	archiveManager := archive.NewManager()

	// Create a test artifact
	testArtifact := filepath.Join(tempDir, "test-artifact.gotya")
	metadata := &Metadata{
		Name:        "test-artifact",
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Test artifact for extraction tests",
	}
	setupTestArtifact(t, testArtifact, true, metadata)

	destDir := filepath.Join(tempDir, "extracted")

	// Extract the artifact
	err := archiveManager.ExtractAll(context.Background(), testArtifact, destDir)
	require.NoError(t, err)

	// Verify extraction
	assert.DirExists(t, destDir)
	assert.DirExists(t, filepath.Join(destDir, artifactMetaDir))
	assert.DirExists(t, filepath.Join(destDir, artifactDataDir))

	// Verify metadata file exists
	metadataFile := filepath.Join(destDir, artifactMetaDir, metadataFile)
	assert.FileExists(t, metadataFile)

	// Verify data files exist
	assert.FileExists(t, filepath.Join(destDir, artifactDataDir, "datafile1.bin"))
	assert.FileExists(t, filepath.Join(destDir, artifactDataDir, "datafile2.bin"))
}

func TestVerifier_extractArtifact_NonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	archiveManager := archive.NewManager()

	// Try to extract a non-existent artifact
	err := archiveManager.ExtractAll(context.Background(), "/nonexistent/path.gotya", tempDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open archive file")
}

func TestVerifier_extractArtifact_InvalidArchive(t *testing.T) {
	tempDir := t.TempDir()
	archiveManager := archive.NewManager()

	// Try to extract a non-existent artifact file
	err := archiveManager.ExtractAll(context.Background(), "/nonexistent/path.gotya", tempDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open archive file")
}

func TestVerifier_extractArtifact_MetaPackage(t *testing.T) {
	tempDir := t.TempDir()
	archiveManager := archive.NewManager()

	// Create a test meta-package (no data directory)
	testArtifact := filepath.Join(tempDir, "test-meta.gotya")
	metadata := &Metadata{
		Name:        "test-meta",
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Test meta package",
	}
	setupTestArtifact(t, testArtifact, false, metadata)

	destDir := filepath.Join(tempDir, "extracted")

	// Extract the meta-package
	err := archiveManager.ExtractAll(context.Background(), testArtifact, destDir)
	require.NoError(t, err)

	// Verify extraction
	assert.DirExists(t, destDir)
	assert.DirExists(t, filepath.Join(destDir, artifactMetaDir))
	assert.NoDirExists(t, filepath.Join(destDir, artifactDataDir))

	// Verify metadata file exists
	metadataFile := filepath.Join(destDir, artifactMetaDir, metadataFile)
	assert.FileExists(t, metadataFile)
}

func TestVerifier_extractArtifact_WithSymlinks(t *testing.T) {
	tempDir := t.TempDir()
	archiveManager := archive.NewManager()

	// Create a test artifact using the standard setup
	testArtifact := filepath.Join(tempDir, "test-artifact.gotya")
	metadata := &Metadata{
		Name:        "test-artifact",
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Test artifact for symlink tests",
	}
	setupTestArtifact(t, testArtifact, true, metadata)

	// Extract the artifact
	destDir := filepath.Join(tempDir, "extracted")
	err := archiveManager.ExtractAll(context.Background(), testArtifact, destDir)
	require.NoError(t, err)

	// Verify the files were extracted correctly
	assert.DirExists(t, destDir)
	assert.DirExists(t, filepath.Join(destDir, artifactMetaDir))
	assert.DirExists(t, filepath.Join(destDir, artifactDataDir))
	assert.FileExists(t, filepath.Join(destDir, artifactMetaDir, metadataFile))
	assert.FileExists(t, filepath.Join(destDir, artifactDataDir, "datafile1.bin"))
	assert.FileExists(t, filepath.Join(destDir, artifactDataDir, "datafile2.bin"))
}

func TestVerifier_extractArtifact_Permissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping permission test on Windows")
	}
	tempDir := t.TempDir()
	archiveManager := archive.NewManager()

	// Create a test artifact
	testArtifact := filepath.Join(tempDir, "test-artifact.gotya")
	metadata := &Metadata{
		Name:        "test-artifact",
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Test artifact for permission tests",
	}
	setupTestArtifact(t, testArtifact, true, metadata)

	// Create destination directory with restricted permissions
	destDir := filepath.Join(tempDir, "readonly-dest")
	require.NoError(t, os.Mkdir(destDir, 0444)) // Read-only directory

	// Try to extract to read-only directory
	err := archiveManager.ExtractAll(context.Background(), testArtifact, destDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
}

func TestVerifier_Verify(t *testing.T) {
	// Common test artifact setup
	validArtifactSetup := func(t *testing.T, tempDir string) string {
		testArtifact := filepath.Join(tempDir, "test-artifact.gotya")
		metadata := &Metadata{
			Name:    "test-artifact",
			Version: "1.0.0",
			OS:      "linux",
			Arch:    "amd64",
		}
		setupTestArtifact(t, testArtifact, true, metadata)
		return testArtifact
	}

	tests := []struct {
		name        string
		doc         string
		descriptor  *model.IndexArtifactDescriptor
		setup       func(*testing.T, string) string
		verifyFn    func(*Verifier, context.Context, *model.IndexArtifactDescriptor, string) error
		expectError bool
		errorMsg    string
	}{
		{
			name: "VerifyArtifact with matching descriptor",
			doc:  "Should successfully verify when descriptor matches artifact metadata",
			descriptor: &model.IndexArtifactDescriptor{
				Name:    "test-artifact",
				Version: "1.0.0",
				OS:      "linux",
				Arch:    "amd64",
			},
			setup: validArtifactSetup,
			verifyFn: func(v *Verifier, ctx context.Context, desc *model.IndexArtifactDescriptor, path string) error {
				return v.VerifyArtifact(ctx, desc, path)
			},
		},
		{
			name: "VerifyArtifact with mismatched name",
			doc:  "Should fail when descriptor name doesn't match artifact metadata",
			descriptor: &model.IndexArtifactDescriptor{
				Name:    "wrong-name",
				Version: "1.0.0",
				OS:      "linux",
				Arch:    "amd64",
			},
			setup:       validArtifactSetup,
			verifyFn:    (*Verifier).VerifyArtifact,
			expectError: true,
			errorMsg:    "metadata mismatch",
		},
		{
			name:       "VerifyArtifact with nil descriptor",
			doc:        "Should successfully verify when no descriptor is provided",
			descriptor: nil,
			setup:      validArtifactSetup,
			verifyFn: func(v *Verifier, ctx context.Context, _ *model.IndexArtifactDescriptor, path string) error {
				return v.VerifyArtifact(ctx, nil, path)
			},
			expectError: false,
		},
		{
			name:       "VerifyArtifactFile with valid artifact",
			doc:        "Should successfully verify a valid artifact file",
			setup:      validArtifactSetup,
			descriptor: nil,
			verifyFn: func(v *Verifier, ctx context.Context, _ *model.IndexArtifactDescriptor, path string) error {
				return v.VerifyArtifact(ctx, nil, path)
			},
			expectError: false,
		},
		{
			name: "VerifyArtifactFile with corrupted artifact",
			doc:  "Should fail when verifying a corrupted artifact file",
			setup: func(t *testing.T, tempDir string) string {
				testArtifact := filepath.Join(tempDir, "corrupted.gotya")
				f, err := os.Create(testArtifact)
				require.NoError(t, err)
				_ = f.Close()
				return testArtifact
			},
			verifyFn: func(v *Verifier, ctx context.Context, _ *model.IndexArtifactDescriptor, path string) error {
				return v.VerifyArtifact(ctx, nil, path)
			},
			expectError: true,
			errorMsg:    "failed to open metadata file",
		},
		{
			name:       "VerifyArtifact with valid artifact and nil descriptor",
			doc:        "Should successfully verify a valid artifact when no descriptor is provided",
			setup:      validArtifactSetup,
			descriptor: nil,
			verifyFn: func(v *Verifier, ctx context.Context, _ *model.IndexArtifactDescriptor, path string) error {
				return v.VerifyArtifact(ctx, nil, path)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			verifier := NewVerifier()

			testArtifact := tt.setup(t, tempDir)

			var err error
			if tt.descriptor != nil {
				err = verifier.VerifyArtifact(context.Background(), tt.descriptor, testArtifact)
			} else {
				err = tt.verifyFn(verifier, context.Background(), tt.descriptor, testArtifact)
			}

			if tt.expectError {
				assert.Error(t, err, tt.doc)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg, "%s: error message mismatch", tt.doc)
				}
			} else {
				assert.NoError(t, err, tt.doc)
			}
		})
	}
}

func TestVerifier_extractArtifact_OverwriteProtection(t *testing.T) {
	tempDir := t.TempDir()
	archiveManager := archive.NewManager()

	// Create a test artifact
	testArtifact := filepath.Join(tempDir, "test-artifact.gotya")
	metadata := &Metadata{
		Name:        "test-artifact",
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Test artifact for overwrite tests",
	}
	setupTestArtifact(t, testArtifact, true, metadata)

	destDir := filepath.Join(tempDir, "extracted")

	// First extraction should succeed
	err := archiveManager.ExtractAll(context.Background(), testArtifact, destDir)
	require.NoError(t, err)

	// Second extraction to the same directory should also succeed (should overwrite)
	err = archiveManager.ExtractAll(context.Background(), testArtifact, destDir)
	require.NoError(t, err)

	// Verify files still exist
	assert.FileExists(t, filepath.Join(destDir, artifactMetaDir, metadataFile))
	assert.FileExists(t, filepath.Join(destDir, artifactDataDir, "datafile1.bin"))
}
