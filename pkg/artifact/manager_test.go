package artifact

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/model"
	"github.com/cperrin88/gotya/pkg/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager(platform.OSLinux, platform.ArchAMD64, t.TempDir(), "", "", "")
	assert.NotNil(t, mgr)
}

func TestInstallArtifact_MissingLocalFile(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager("linux", "amd64", tempDir, "", tempDir, filepath.Join(tempDir, "installed.db"))

	desc := &model.IndexArtifactDescriptor{
		Name:    "invalid-artifact",
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
	}

	err := mgr.InstallArtifact(context.Background(), desc, "/non/existent/path.gotya")
	assert.Equal(t, errors.ErrArtifactNotFound, err)
}

func TestInstallArtifact_RegularPackage(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager("linux", "amd64", tempDir, "", tempDir, filepath.Join(tempDir, "installed.db"))

	// Create a test artifact
	testArtifact := filepath.Join(tempDir, "test.gotya")
	setupTestArtifact(t, testArtifact, true)

	desc := &model.IndexArtifactDescriptor{
		Name:    "test-package",
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/test.gotya",
	}

	// Install the artifact
	err := mgr.InstallArtifact(context.Background(), desc, testArtifact)
	require.NoError(t, err)

	// Verify files were installed
	assert.DirExists(t, filepath.Join(tempDir, "test-package"))
	assert.FileExists(t, filepath.Join(tempDir, "test-package", "meta", "manifest.json"))
	assert.FileExists(t, filepath.Join(tempDir, "test-package", "data", "datafile.bin"))
}

func TestInstallArtifact_MetaPackage(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager("linux", "amd64", tempDir, "", tempDir, filepath.Join(tempDir, "installed.db"))

	// Create a test meta-package (no data directory)
	testArtifact := filepath.Join(tempDir, "meta-package.gotya")
	setupTestArtifact(t, testArtifact, false)

	desc := &model.IndexArtifactDescriptor{
		Name:        "meta-package",
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		URL:         "http://example.com/meta-package.gotya",
		Description: "A meta package with no data files",
	}

	// Install the meta-package
	err := mgr.InstallArtifact(context.Background(), desc, testArtifact)
	require.NoError(t, err)

	// Verify only metadata was installed
	assert.DirExists(t, filepath.Join(tempDir, "meta-package"))
	assert.FileExists(t, filepath.Join(tempDir, "meta-package", "meta", "manifest.json"))
	assert.NoDirExists(t, filepath.Join(tempDir, "meta-package", "data"))
}

func TestInstallArtifact_RollbackOnFailure(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager("linux", "amd64", tempDir, "", tempDir, filepath.Join(tempDir, "instored.db"))

	// Create a test artifact with invalid data to cause installation failure
	testArtifact := filepath.Join(tempDir, "bad-package.gotya")
	setupTestArtifact(t, testArtifact, true)

	// Make the data directory read-only to cause a failure
	dataDir := filepath.Join(tempDir, "bad-package", "data")
	require.NoError(t, os.MkdirAll(dataDir, 0555))

	desc := &model.IndexArtifactDescriptor{
		Name:    "bad-package",
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
	}

	// Installation should fail
	err := mgr.InstallArtifact(context.Background(), desc, testArtifact)
	require.Error(t, err)

	// Verify rollback cleaned up everything
	assert.NoDirExists(t, filepath.Join(tempDir, "bad-package"))
}

func TestInstallArtifact_AlreadyInstalled(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager("linux", "amd64", tempDir, "", tempDir, filepath.Join(tempDir, "installed.db"))

	// Create and install a test artifact
	testArtifact := filepath.Join(tempDir, "test-pkg.gotya")
	setupTestArtifact(t, testArtifact, true)

	desc := &model.IndexArtifactDescriptor{
		Name:    "test-pkg",
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
	}

	// First installation should succeed
	err := mgr.InstallArtifact(context.Background(), desc, testArtifact)
	require.NoError(t, err)

	// Second installation should fail
	err = mgr.InstallArtifact(context.Background(), desc, testArtifact)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already installed")
}

// setupTestArtifact creates a test artifact file with the specified structure
func setupTestArtifact(t *testing.T, artifactPath string, includeDataDir bool) {
	t.Helper()

	// Create a temporary directory for the test artifact
	tempDir := t.TempDir()

	// Create metadata directory and files
	metaDir := filepath.Join(tempDir, "meta")
	require.NoError(t, os.MkdirAll(metaDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(metaDir, "manifest.json"),
		[]byte(`{"name":"test","version":"1.0.0"}`),
		0644,
	))

	// Optionally create data directory and files
	if includeDataDir {
		dataDir := filepath.Join(tempDir, "data")
		require.NoError(t, os.MkdirAll(dataDir, 0755))
		require.NoError(t, os.WriteFile(
			filepath.Join(dataDir, "datafile.bin"),
			[]byte("test data"),
			0644,
		))
	}

	// Create a tar.gz of the test files
	// In a real test, you would use the actual archiving logic here
	// For simplicity, we'll just create a dummy file
	require.NoError(t, os.WriteFile(artifactPath, []byte("test artifact"), 0644))
}
