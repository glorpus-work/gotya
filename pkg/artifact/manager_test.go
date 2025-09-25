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

var (
	DefaultArtifactName    = "test-artifact"
	DefaultArtifactVersion = "1.0.0"
	DefaultArtifactOS      = "linux"
	DefaultArtifactArch    = "amd64"

	DefaultMetadata = &Metadata{
		Name:        DefaultArtifactName,
		Version:     DefaultArtifactVersion,
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Test artifact for unit tests",
	}
	DefaultIndexArtifactDescriptor = &model.IndexArtifactDescriptor{
		Name:    DefaultArtifactName,
		Version: DefaultArtifactVersion,
		OS:      DefaultArtifactOS,
		Arch:    DefaultArtifactArch,
	}
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
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), filepath.Join(tempDir, "installed.db"))
	artifactName := "test-artifact"

	// Create a test artifact
	testArtifact := filepath.Join(tempDir, "test-artifact.gotya")
	metadata := &Metadata{
		Name:         artifactName,
		Version:      "1.0.0",
		OS:           "linux",
		Arch:         "amd64",
		Maintainer:   "test@example.com",
		Description:  "Test artifact for unit tests",
		Dependencies: []string{"dep1", "dep2"},
		Hooks:        make(map[string]string),
	}
	setupTestArtifact(t, testArtifact, true, metadata)

	desc := &model.IndexArtifactDescriptor{
		Name:    artifactName,
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/test.gotya",
	}

	// Install the artifact
	err := mgr.InstallArtifact(context.Background(), desc, testArtifact)
	require.NoError(t, err)

	// Verify files were installed
	assert.DirExists(t, filepath.Join(tempDir, "install"))
	assert.FileExists(t, filepath.Join(tempDir, "install", "meta", artifactName, "artifact.json"))
	assert.FileExists(t, filepath.Join(tempDir, "install", "data", artifactName, "datafile1.bin"))
	assert.FileExists(t, filepath.Join(tempDir, "install", "data", artifactName, "datafile2.bin"))
}

func TestInstallArtifact_MetaPackage(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), filepath.Join(tempDir, "installed.db"))

	// Create a test meta-package (no data directory)
	testArtifact := filepath.Join(tempDir, "meta-package.gotya")
	setupTestArtifact(t, testArtifact, false, DefaultMetadata)

	// Install the meta-package
	err := mgr.InstallArtifact(context.Background(), DefaultIndexArtifactDescriptor, testArtifact)
	require.NoError(t, err)

	// Verify only metadata was installed
	assert.DirExists(t, filepath.Join(tempDir, "install"))
	assert.FileExists(t, filepath.Join(tempDir, "install", "meta", DefaultArtifactName, "artifact.json"))
	assert.NoDirExists(t, filepath.Join(tempDir, "install", "data", DefaultArtifactName))
}

func TestInstallArtifact_RollbackOnFailure(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, "instored.db"))

	// Create a test artifact with invalid data to cause installation failure
	testArtifact := filepath.Join(tempDir, "bad-package.gotya")
	setupTestArtifact(t, testArtifact, true, DefaultMetadata)

	// Create parent directories with write permissions
	parentDir := filepath.Join(tempDir, "install", artifactDataDir)
	require.NoError(t, os.MkdirAll(parentDir, 0755))

	// Create the final directory as read-only to cause installation failure
	dataDir := filepath.Join(parentDir, "bad-package")
	require.NoError(t, os.Mkdir(dataDir, 0555))

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
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), filepath.Join(tempDir, "installed.db"))

	// Create and install a test artifact
	testArtifact := filepath.Join(tempDir, "test-pkg.gotya")
	setupTestArtifact(t, testArtifact, true, DefaultMetadata)

	desc := DefaultIndexArtifactDescriptor

	// First installation should succeed
	err := mgr.InstallArtifact(context.Background(), desc, testArtifact)
	require.NoError(t, err)

	// Second installation should fail
	err = mgr.InstallArtifact(context.Background(), desc, testArtifact)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already installed")
}

// setupTestArtifact creates a test artifact file with the specified structure and metadata
// If metadata is nil, default test metadata will be used
func setupTestArtifact(t *testing.T, artifactPath string, includeDataDir bool, metadata *Metadata) {
	t.Helper()

	// Create a temporary directory for the test artifact
	tempDir := t.TempDir()

	// Create input directory structure
	inputDir := filepath.Join(tempDir, "input")
	require.NoError(t, os.MkdirAll(inputDir, 0755))

	// Create data directory and files if needed
	if includeDataDir {
		dataDir := filepath.Join(inputDir, "data")
		require.NoError(t, os.MkdirAll(dataDir, 0755))

		// Create test data files
		dataFiles := map[string][]byte{
			"datafile1.bin": []byte("test data 1"),
			"datafile2.bin": []byte("test data 2"),
		}

		for filename, content := range dataFiles {
			filePath := filepath.Join(dataDir, filename)
			require.NoError(t, os.WriteFile(filePath, content, 0644))
		}
	}

	// Create output directory
	outputDir := filepath.Join(tempDir, "input")
	require.NoError(t, os.MkdirAll(outputDir, 0755))

	// Initialize the packer with the provided or default metadata
	packer := NewPacker(
		metadata.Name,
		metadata.Version,
		metadata.OS,
		metadata.Arch,
		metadata.Maintainer,
		metadata.Description,
		metadata.Dependencies,
		metadata.Hooks,
		inputDir,
		outputDir,
	)

	// Create the artifact using the packer
	outputFile, err := packer.Pack()
	require.NoError(t, err)

	// Copy the artifact to the specified path
	require.NoError(t, os.Rename(outputFile, artifactPath))
}
