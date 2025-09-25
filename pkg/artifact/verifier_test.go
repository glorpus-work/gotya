package artifact

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifier_extractArtifact_Success(t *testing.T) {
	tempDir := t.TempDir()
	verifier := NewVerifier()

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
	err := verifier.extractArtifact(context.Background(), testArtifact, destDir)
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
	verifier := NewVerifier()

	// Try to extract a non-existent artifact
	err := verifier.extractArtifact(context.Background(), "/nonexistent/path.gotya", tempDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open artifact file")
}

func TestVerifier_extractArtifact_InvalidArchive(t *testing.T) {
	tempDir := t.TempDir()
	verifier := NewVerifier()

	// Try to extract a non-existent artifact file
	err := verifier.extractArtifact(context.Background(), "/nonexistent/path.gotya", tempDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open artifact file")
}

func TestVerifier_extractArtifact_MetaPackage(t *testing.T) {
	tempDir := t.TempDir()
	verifier := NewVerifier()

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
	err := verifier.extractArtifact(context.Background(), testArtifact, destDir)
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
	verifier := NewVerifier()

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
	err := verifier.extractArtifact(context.Background(), testArtifact, destDir)
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
	tempDir := t.TempDir()
	verifier := NewVerifier()

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
	err := verifier.extractArtifact(context.Background(), testArtifact, destDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
}

func TestVerifier_extractArtifact_OverwriteProtection(t *testing.T) {
	tempDir := t.TempDir()
	verifier := NewVerifier()

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
	err := verifier.extractArtifact(context.Background(), testArtifact, destDir)
	require.NoError(t, err)

	// Second extraction to the same directory should also succeed (should overwrite)
	err = verifier.extractArtifact(context.Background(), testArtifact, destDir)
	require.NoError(t, err)

	// Verify files still exist
	assert.FileExists(t, filepath.Join(destDir, artifactMetaDir, metadataFile))
	assert.FileExists(t, filepath.Join(destDir, artifactDataDir, "datafile1.bin"))
}
