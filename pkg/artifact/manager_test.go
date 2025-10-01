package artifact

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glorpus-work/gotya/pkg/artifact/database"
	"github.com/glorpus-work/gotya/pkg/errors"
	"github.com/glorpus-work/gotya/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var (
	DefaultArtifactName    = "test-artifact"
	DefaultArtifactVersion = "1.0.0"
	DefaultArtifactOS      = "linux"
	DefaultArtifactArch    = "amd64"
	DefaultArtifactURL     = "http://example.com/test.gotya"
	DefaultMetadata        = &Metadata{
		Name:         DefaultArtifactName,
		Version:      DefaultArtifactVersion,
		OS:           DefaultArtifactOS,
		Arch:         DefaultArtifactArch,
		Maintainer:   "test@example.com",
		Description:  "Test artifact for unit tests",
		Dependencies: []model.Dependency{},
		Hooks:        map[string]string{},
	}
	DefaultIndexArtifactDescriptor = &model.IndexArtifactDescriptor{
		Name:    DefaultMetadata.Name,
		Version: DefaultMetadata.Version,
		OS:      DefaultMetadata.OS,
		Arch:    DefaultMetadata.Arch,
		URL:     DefaultArtifactURL,
	}
	DefaultInstalledArtifact = &model.InstalledArtifact{
		Name:    DefaultMetadata.Name,
		Version: DefaultMetadata.Version,
		OS:      DefaultMetadata.OS,
		Arch:    DefaultMetadata.Arch,
	}
)

func TestNewManager(t *testing.T) {
	mgr := NewManager("linux", "amd64", t.TempDir(), "", "", "")
	assert.NotNil(t, mgr)
}

func TestInstallArtifact_MissingLocalFile(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager("linux", "amd64", tempDir, "", tempDir, filepath.Join(tempDir, "installed.db"))

	desc := &model.IndexArtifactDescriptor{
		Name:    "invalid-artifact",
		Version: "1.0.0",
		URL:     "http://example.com/invalid-artifact.gotya",
	}

	err := mgr.InstallArtifact(context.Background(), desc, "/non/existent/path.gotya", model.InstallationReasonManual)
	var pathError *os.PathError
	assert.ErrorAs(t, err, &pathError)
}

func TestInstallArtifact_RegularPackage(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), dbPath)
	artifactName := "test-artifact"

	// Create a test artifact
	testArtifact := filepath.Join(tempDir, "test-artifact.gotya")

	setupTestArtifact(t, testArtifact, true, DefaultMetadata)

	desc := &model.IndexArtifactDescriptor{
		Name:    artifactName,
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/test.gotya",
	}

	// Install the artifact
	err := mgr.InstallArtifact(context.Background(), desc, testArtifact, model.InstallationReasonManual)
	require.NoError(t, err)

	// Verify files were installed
	assert.DirExists(t, filepath.Join(tempDir, "install"))
	assert.FileExists(t, filepath.Join(tempDir, "install", "meta", artifactName, "artifact.json"))
	assert.FileExists(t, filepath.Join(tempDir, "install", "data", artifactName, "datafile1.bin"))
	assert.FileExists(t, filepath.Join(tempDir, "install", "data", artifactName, "datafile2.bin"))

	db := loadInstalledDB(t, dbPath)

	// Check if artifact is marked as installed
	assert.True(t, db.IsArtifactInstalled(artifactName), "artifact should be marked as installed in database")

	// Get the installed artifact details
	installedArtifact := db.FindArtifact(artifactName)
	require.NotNil(t, installedArtifact, "installed artifact not found in database")

	// Verify installed artifact details
	assert.Equal(t, DefaultArtifactName, installedArtifact.Name, "artifact name in database doesn't match")
	assert.Equal(t, DefaultArtifactVersion, installedArtifact.Version, "artifact version in database doesn't match")
	assert.Equal(t, DefaultArtifactURL, installedArtifact.InstalledFrom, "installed from URL doesn't match")
	assert.NotEmpty(t, installedArtifact.InstalledAt, "installed at timestamp should be set")

	// Verify installed files in database
	expectedMetaFiles := []string{
		filepath.Join(tempDir, "install", "meta", artifactName, "artifact.json"),
	}
	expectedDataFiles := []string{
		filepath.Join(tempDir, "install", "data", artifactName, "datafile1.bin"),
		filepath.Join(tempDir, "install", "data", artifactName, "datafile2.bin"),
	}

	// Convert MetaFiles and DataFiles to full paths for comparison
	var actualMetaFiles []string
	for _, f := range installedArtifact.MetaFiles {
		actualMetaFiles = append(actualMetaFiles, filepath.Join(installedArtifact.ArtifactMetaDir, f.Path))
	}

	var actualDataFiles []string
	for _, f := range installedArtifact.DataFiles {
		actualDataFiles = append(actualDataFiles, filepath.Join(installedArtifact.ArtifactDataDir, f.Path))
	}

	assert.ElementsMatch(t, expectedMetaFiles, actualMetaFiles, "meta files in database don't match")
	assert.ElementsMatch(t, expectedDataFiles, actualDataFiles, "data files in database don't match")
}

func TestInstallArtifact_MetaPackage(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), dbPath)

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

	desc := &model.IndexArtifactDescriptor{
		Name:    "test-meta",
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/meta.gotya",
	}

	err := mgr.InstallArtifact(context.Background(), desc, testArtifact, model.InstallationReasonManual)
	require.NoError(t, err)

	// Verify only metadata was installed
	assert.FileExists(t, filepath.Join(tempDir, "install", "meta", "test-meta", "artifact.json"))
	assert.NoDirExists(t, filepath.Join(tempDir, "install", "data", "test-meta"))

	db := loadInstalledDB(t, dbPath)

	// Check if meta-package is marked as installed
	assert.True(t, db.IsArtifactInstalled("test-meta"), "meta-package should be marked as installed in database")

	// Get the installed meta-package details
	installedArtifact := db.FindArtifact("test-meta")
	require.NotNil(t, installedArtifact, "installed meta-package not found in database")

	// Verify installed meta-package details
	assert.Equal(t, "test-meta", installedArtifact.Name, "meta-package name in database doesn't match")
	assert.Equal(t, "1.0.0", installedArtifact.Version, "meta-package version in database doesn't match")
	assert.Equal(t, "http://example.com/meta.gotya", installedArtifact.InstalledFrom, "installed from URL doesn't match")
	assert.NotEmpty(t, installedArtifact.InstalledAt, "installed at timestamp should be set")

	// Verify only metadata file is recorded in database
	expectedMetaFiles := []string{
		filepath.Join(tempDir, "install", "meta", "test-meta", "artifact.json"),
	}

	// Convert MetaFiles to full paths for comparison
	var actualMetaFiles []string
	for _, f := range installedArtifact.MetaFiles {
		actualMetaFiles = append(actualMetaFiles, filepath.Join(installedArtifact.ArtifactMetaDir, f.Path))
	}

	assert.ElementsMatch(t, expectedMetaFiles, actualMetaFiles, "meta files in database don't match")
	assert.Empty(t, installedArtifact.DataFiles, "data files should be empty for meta-package")
}

func TestInstallArtifact_RollbackOnFailure(t *testing.T) {
	tempDir := t.TempDir()

	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactDataDir), dbPath)

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
	err := mgr.InstallArtifact(context.Background(), desc, testArtifact, model.InstallationReasonManual)
	require.Error(t, err)

	// Verify rollback cleaned up everything
	assert.NoDirExists(t, filepath.Join(tempDir, "bad-package"))

	db := loadInstalledDB(t, dbPath)

	assert.False(t, db.IsArtifactInstalled("bad-package"), "bad-package should not be marked as installed in database")
	assert.Empty(t, db.GetInstalledArtifacts(), "no package should be installed at all")
}

func TestInstallArtifact_InstallFailure(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactDataDir), dbPath)

	// Create a test artifact
	testArtifact := filepath.Join(tempDir, "test-artifact.gotya")
	metadata := &Metadata{
		Name:        "test-artifact",
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Test artifact for rollback tests",
	}
	setupTestArtifact(t, testArtifact, true, metadata)

	desc := &model.IndexArtifactDescriptor{
		Name:    "test-artifact",
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/test.gotya",
	}

	// Create parent directories with write permissions
	parentDir := filepath.Join(tempDir, artifactDataDir)
	require.NoError(t, os.MkdirAll(parentDir, 0755))

	// Create the final directory as read-only to cause installation failure
	dataDir := filepath.Join(parentDir, "test-artifact")
	require.NoError(t, os.Mkdir(dataDir, 0555)) // Read-only directory

	// Installation should fail
	err := mgr.InstallArtifact(context.Background(), desc, testArtifact, model.InstallationReasonManual)
	require.Error(t, err)

	// Verify rollback cleaned up everything
	assert.NoDirExists(t, filepath.Join(tempDir, "test-artifact"))

	db := loadInstalledDB(t, dbPath)

	assert.False(t, db.IsArtifactInstalled("test-artifact"), "artifact should not be marked as installed in database")
	assert.Empty(t, db.GetInstalledArtifacts(), "no package should be installed at all")
}

func TestInstallArtifact_DatabaseFailure(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "readonly.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), dbPath)

	// Create a test artifact
	testArtifact := filepath.Join(tempDir, "test-artifact.gotya")
	metadata := &Metadata{
		Name:        "test-artifact",
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Test artifact for database failure tests",
	}
	setupTestArtifact(t, testArtifact, true, metadata)

	desc := &model.IndexArtifactDescriptor{
		Name:    "test-artifact",
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/test.gotya",
	}

	// Make database file read-only to cause save failure
	require.NoError(t, os.WriteFile(dbPath, []byte("test"), 0444)) // Read-only file

	// Installation should fail
	err := mgr.InstallArtifact(context.Background(), desc, testArtifact, model.InstallationReasonManual)
	require.Error(t, err)

	// Verify rollback cleaned up installed files
	assert.NoDirExists(t, filepath.Join(tempDir, "test-artifact"))

	// Database should be unchanged (still read-only)
	// Note: We can't load the database since it's read-only, so we skip this check
}

// TestInstallArtifact_EmptyArtifactName tests installation with empty artifact name
func TestInstallArtifact_EmptyArtifactName(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), filepath.Join(tempDir, "installed.db"))

	desc := &model.IndexArtifactDescriptor{
		Name:    "", // Empty name
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
	}

	err := mgr.InstallArtifact(context.Background(), desc, "/nonexistent/path.gotya", model.InstallationReasonManual)
	require.Error(t, err)
	assert.ErrorIs(t, err, errors.ErrValidation)
}

// TestInstallArtifact_EmptyDescriptor tests installation with nil descriptor
func TestInstallArtifact_EmptyDescriptor(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), filepath.Join(tempDir, "installed.db"))

	err := mgr.InstallArtifact(context.Background(), nil, "/nonexistent/path.gotya", model.InstallationReasonManual)
	require.Error(t, err)
	assert.ErrorContains(t, err, "artifact descriptor cannot be nil")
}

func TestInstallArtifact_ReverseDependenciesSet(t *testing.T) {
	tempDir := t.TempDir()

	dependencyName := "dep1"
	DefaultMetadata.Dependencies = []model.Dependency{
		{Name: dependencyName},
	}
	DefaultIndexArtifactDescriptor.Dependencies = []model.Dependency{
		{Name: dependencyName},
	}

	metadata := &Metadata{
		Name:         dependencyName,
		Version:      "1.0.0",
		OS:           "linux",
		Arch:         "amd64",
		Maintainer:   "test@example.com",
		Description:  "Test dependency for reverse dependencies tests",
		Dependencies: []model.Dependency{},
		Hooks:        map[string]string{},
	}

	desc := &model.IndexArtifactDescriptor{
		Name:         dependencyName,
		Version:      "1.0.0",
		OS:           "linux",
		Arch:         "amd64",
		URL:          "http://example.com/dep1.gotya",
		Dependencies: []model.Dependency{},
	}

	setupTestArtifact(t, filepath.Join(tempDir, "artifact.gotya"), true, DefaultMetadata)
	setupTestArtifact(t, filepath.Join(tempDir, "dep1.gotya"), false, metadata)

	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), dbPath)

	require.NoError(t, mgr.InstallArtifact(context.Background(), DefaultIndexArtifactDescriptor, filepath.Join(tempDir, "artifact.gotya"), model.InstallationReasonManual))

	db := loadInstalledDB(t, dbPath)
	assert.True(t, db.IsArtifactInstalled(DefaultArtifactName))
	assert.True(t, db.IsArtifactInstalled(dependencyName))
	artifact := db.FindArtifact(dependencyName)
	require.NotNil(t, artifact)
	assert.Equal(t, model.StatusMissing, artifact.Status)
	assert.Equal(t, DefaultArtifactName, artifact.ReverseDependencies[0])

	require.NoError(t, mgr.InstallArtifact(context.Background(), desc, filepath.Join(tempDir, "dep1.gotya"), model.InstallationReasonManual))

	db = loadInstalledDB(t, dbPath)
	artifact = db.FindArtifact(dependencyName)
	require.NotNil(t, artifact)
	assert.Equal(t, model.StatusInstalled, artifact.Status)
	assert.Equal(t, DefaultArtifactName, artifact.ReverseDependencies[0])

}

// TestUninstallArtifact_UpdatesReverseDependencies tests that reverse dependencies are cleaned up when artifacts are uninstalled
func TestUninstallArtifact_UpdatesReverseDependencies(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), dbPath)
	artifactName := "test-artifact"
	depName := "test-dependency"

	// Step 1: Install an artifact with missing dependencies (creates dummy entries)
	mainArtifact := filepath.Join(tempDir, "test-artifact.gotya")
	mainMetadata := &Metadata{
		Name:        artifactName,
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Test artifact with dependencies",
		Dependencies: []model.Dependency{
			{Name: depName},
		},
		Hooks: make(map[string]string),
	}
	setupTestArtifact(t, mainArtifact, true, mainMetadata)

	mainDesc := &model.IndexArtifactDescriptor{
		Name:    artifactName,
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/test-artifact.gotya",
		Dependencies: []model.Dependency{
			{Name: depName, VersionConstraint: "1.0.0"},
		},
	}

	err := mgr.InstallArtifact(context.Background(), mainDesc, mainArtifact, model.InstallationReasonManual)
	require.NoError(t, err)

	// Verify dummy entry was created for the dependency
	db := loadInstalledDB(t, dbPath)
	depDummy := db.FindArtifact(depName)
	require.NotNil(t, depDummy, "dummy entry for dependency should exist")
	assert.Equal(t, model.StatusMissing, depDummy.Status, "dependency should have missing status")
	assert.Contains(t, depDummy.ReverseDependencies, artifactName, "dependency should have main artifact as reverse dependency")

	// Step 2: Install the dependency (this establishes the reverse dependency relationship)
	depArtifact := filepath.Join(tempDir, "test-dependency.gotya")
	depMetadata := &Metadata{
		Name:         depName,
		Version:      "1.0.0",
		OS:           "linux",
		Arch:         "amd64",
		Maintainer:   "test@example.com",
		Description:  "Test dependency",
		Dependencies: []model.Dependency{},
		Hooks:        make(map[string]string),
	}
	setupTestArtifact(t, depArtifact, true, depMetadata)

	depDesc := &model.IndexArtifactDescriptor{
		Name:         depName,
		Version:      "1.0.0",
		OS:           "linux",
		Arch:         "amd64",
		URL:          "http://example.com/test-dependency.gotya",
		Dependencies: []model.Dependency{},
	}

	err = mgr.InstallArtifact(context.Background(), depDesc, depArtifact, model.InstallationReasonManual)
	require.NoError(t, err)

	// Verify the dependency is now installed and has the main artifact as reverse dependency
	db = loadInstalledDB(t, dbPath)
	installedDep := db.FindArtifact(depName)
	require.NotNil(t, installedDep, "dependency should exist")
	assert.Equal(t, model.StatusInstalled, installedDep.Status, "dependency should have installed status")
	assert.Contains(t, installedDep.ReverseDependencies, artifactName, "dependency should have main artifact as reverse dependency")

	// Step 3: Uninstall the dependency and verify reverse dependencies are cleaned up
	err = mgr.UninstallArtifact(context.Background(), depName, false)
	require.NoError(t, err)

	// Verify the dependency was removed from the database
	db = loadInstalledDB(t, dbPath)
	assert.False(t, db.IsArtifactInstalled(depName), "dependency should not be installed")
	removedDep := db.FindArtifact(depName)
	assert.Nil(t, removedDep, "dependency should not exist in database")

	// Verify the main artifact still exists but no longer has the dependency as a reverse dependency
	installedMainArtifact := db.FindArtifact(artifactName)
	require.NotNil(t, installedMainArtifact, "main artifact should still exist")
	assert.Equal(t, model.StatusInstalled, installedMainArtifact.Status, "main artifact should still have installed status")
	assert.Empty(t, installedMainArtifact.ReverseDependencies, "main artifact should have no reverse dependencies")
}

// TestUninstallArtifact_NonExistent tests uninstalling a non-existent artifact
func TestUninstallArtifact_NonExistent(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), filepath.Join(tempDir, "installed.db"))

	// Try to uninstall a non-existent artifact
	err := mgr.UninstallArtifact(context.Background(), "non-existent-artifact", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not installed")
}

// TestUninstallArtifact_PurgeMode tests uninstalling an artifact with purge=true
func TestUninstallArtifact_PurgeMode(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), dbPath)
	artifactName := "test-artifact"

	// Create and install a test artifact
	testArtifact := filepath.Join(tempDir, "test-artifact.gotya")
	metadata := &Metadata{
		Name:        artifactName,
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Test artifact for unit tests",
		Dependencies: []model.Dependency{
			{Name: "dep1"},
			{Name: "dep2"},
		},
		Hooks: make(map[string]string),
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
	err := mgr.InstallArtifact(context.Background(), desc, testArtifact, model.InstallationReasonManual)
	require.NoError(t, err)

	// Verify it was installed
	db := loadInstalledDB(t, dbPath)
	assert.True(t, db.IsArtifactInstalled(artifactName), "artifact should be installed")

	// Uninstall with purge=true
	err = mgr.UninstallArtifact(context.Background(), artifactName, true)
	require.NoError(t, err)

	// Verify complete removal
	assert.NoDirExists(t, filepath.Join(tempDir, "install", "meta", artifactName))
	assert.NoDirExists(t, filepath.Join(tempDir, "install", "data", artifactName))

	// Verify database is clean
	db = loadInstalledDB(t, dbPath)
	assert.False(t, db.IsArtifactInstalled(artifactName), "artifact should be removed from database")
	assert.Empty(t, db.GetInstalledArtifacts(), "no artifacts should remain in database")
}

// TestUninstallArtifact_SelectiveMode tests uninstalling an artifact with purge=false
func TestUninstallArtifact_SelectiveMode(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), dbPath)
	artifactName := "test-artifact"

	// Create and install a test artifact
	testArtifact := filepath.Join(tempDir, "test-artifact.gotya")
	metadata := &Metadata{
		Name:        artifactName,
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Test artifact for unit tests",
		Dependencies: []model.Dependency{
			{Name: "dep1"},
			{Name: "dep2"},
		},
		Hooks: make(map[string]string),
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
	err := mgr.InstallArtifact(context.Background(), desc, testArtifact, model.InstallationReasonManual)
	require.NoError(t, err)

	// Verify it was installed
	db := loadInstalledDB(t, dbPath)
	assert.True(t, db.IsArtifactInstalled(artifactName), "artifact should be installed")

	installedArtifact := db.FindArtifact(artifactName)
	require.NotNil(t, installedArtifact)

	// Get paths before uninstall
	metaDir := installedArtifact.ArtifactMetaDir
	dataDir := installedArtifact.ArtifactDataDir

	// Uninstall with purge=false
	err = mgr.UninstallArtifact(context.Background(), artifactName, false)
	require.NoError(t, err)

	// Verify files were removed
	for _, file := range installedArtifact.MetaFiles {
		assert.NoFileExists(t, filepath.Join(metaDir, file.Path))
	}
	for _, file := range installedArtifact.DataFiles {
		assert.NoFileExists(t, filepath.Join(dataDir, file.Path))
	}

	// Verify directories are gone (should be cleaned up)
	assert.NoDirExists(t, metaDir)
	assert.NoDirExists(t, dataDir)

	// Verify database is clean
	db = loadInstalledDB(t, dbPath)
	assert.False(t, db.IsArtifactInstalled(artifactName), "artifact should be removed from database")
	assert.Empty(t, db.GetInstalledArtifacts(), "no artifacts should remain in database")
}

// TestUninstallArtifact_SelectiveMode_MissingFiles tests selective uninstall when some files are missing
func TestUninstallArtifact_SelectiveMode_MissingFiles(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), dbPath)
	artifactName := "test-artifact"

	// Create and install a test artifact
	testArtifact := filepath.Join(tempDir, "test-artifact.gotya")
	metadata := &Metadata{
		Name:        artifactName,
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Test artifact for unit tests",
		Dependencies: []model.Dependency{
			{Name: "dep1"},
			{Name: "dep2"},
		},
		Hooks: make(map[string]string),
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
	err := mgr.InstallArtifact(context.Background(), desc, testArtifact, model.InstallationReasonManual)
	require.NoError(t, err)

	// Verify it was installed
	db := loadInstalledDB(t, dbPath)
	assert.True(t, db.IsArtifactInstalled(artifactName), "artifact should be installed")

	installedArtifact := db.FindArtifact(artifactName)
	require.NotNil(t, installedArtifact)

	// Remove one file manually to simulate missing file
	dataFiles := installedArtifact.DataFiles
	if len(dataFiles) > 0 {
		testFile := filepath.Join(installedArtifact.ArtifactDataDir, dataFiles[0].Path)
		err := os.Remove(testFile)
		require.NoError(t, err)
	}

	// Uninstall with purge=false - should succeed despite missing file
	err = mgr.UninstallArtifact(context.Background(), artifactName, false)
	require.NoError(t, err)

	// Verify database is clean
	db = loadInstalledDB(t, dbPath)
	assert.False(t, db.IsArtifactInstalled(artifactName), "artifact should be removed from database")
	assert.Empty(t, db.GetInstalledArtifacts(), "no artifacts should remain in database")
}

// TestUninstallArtifact_MetaPackage tests uninstalling a meta-package (no data files)
func TestUninstallArtifact_MetaPackage(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), dbPath)

	// Test both purge and selective modes
	testCases := []struct {
		name         string
		purge        bool
		artifactName string
	}{
		{"purge mode", true, "test-meta-purge"},
		{"selective mode", false, "test-meta-selective"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test meta-package (no data directory)
			testArtifact := filepath.Join(tempDir, tc.artifactName+".gotya")
			metadata := &Metadata{
				Name:        tc.artifactName,
				Version:     "1.0.0",
				OS:          "linux",
				Arch:        "amd64",
				Maintainer:  "test@example.com",
				Description: "Test meta package",
			}
			setupTestArtifact(t, testArtifact, false, metadata)

			desc := &model.IndexArtifactDescriptor{
				Name:    tc.artifactName,
				Version: "1.0.0",
				OS:      "linux",
				Arch:    "amd64",
				URL:     "http://example.com/meta.gotya",
			}

			// Install the meta-package
			err := mgr.InstallArtifact(context.Background(), desc, testArtifact, model.InstallationReasonManual)
			require.NoError(t, err)

			// Verify it was installed
			db := loadInstalledDB(t, dbPath)
			assert.True(t, db.IsArtifactInstalled(tc.artifactName), "meta-package should be installed")

			// Uninstall
			err = mgr.UninstallArtifact(context.Background(), tc.artifactName, tc.purge)
			require.NoError(t, err)

			// Verify removal
			db = loadInstalledDB(t, dbPath)
			assert.False(t, db.IsArtifactInstalled(tc.artifactName), "meta-package should be removed from database")
		})
	}
}

// TestUpdateArtifact_Successful tests successful artifact update
func TestUpdateArtifact_Successful(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), dbPath)
	artifactName := "test-artifact"

	// Create and install the original version
	originalArtifact := filepath.Join(tempDir, "original.gotya")
	originalMetadata := &Metadata{
		Name:        artifactName,
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Original version",
		Dependencies: []model.Dependency{
			{Name: "dep1"},
		},
		Hooks: make(map[string]string),
	}
	setupTestArtifact(t, originalArtifact, true, originalMetadata)

	originalDesc := &model.IndexArtifactDescriptor{
		Name:    artifactName,
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/v1.0.0.gotya",
	}

	// Install the original version
	err := mgr.InstallArtifact(context.Background(), originalDesc, originalArtifact, model.InstallationReasonManual)
	require.NoError(t, err)

	// Verify original version is installed
	db := loadInstalledDB(t, dbPath)
	assert.True(t, db.IsArtifactInstalled(artifactName), "original artifact should be installed")
	originalInstalled := db.FindArtifact(artifactName)
	require.NotNil(t, originalInstalled)
	assert.Equal(t, "1.0.0", originalInstalled.Version)

	// Create the updated version
	updatedArtifact := filepath.Join(tempDir, "updated.gotya")
	updatedMetadata := &Metadata{
		Name:        artifactName,
		Version:     "2.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Updated version",
		Dependencies: []model.Dependency{
			{Name: "dep1"},
			{Name: "dep2"},
		}, // Added new dependency
		Hooks: make(map[string]string),
	}
	setupTestArtifact(t, updatedArtifact, true, updatedMetadata)

	updatedDesc := &model.IndexArtifactDescriptor{
		Name:    artifactName,
		Version: "2.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/v2.0.0.gotya",
	}

	// Update the artifact
	err = mgr.UpdateArtifact(context.Background(), updatedArtifact, updatedDesc)
	require.NoError(t, err)

	// Verify the update was successful
	db = loadInstalledDB(t, dbPath)
	assert.True(t, db.IsArtifactInstalled(artifactName), "updated artifact should be installed")
	updatedInstalled := db.FindArtifact(artifactName)
	require.NotNil(t, updatedInstalled)
	assert.Equal(t, "2.0.0", updatedInstalled.Version)
	assert.Equal(t, "http://example.com/v2.0.0.gotya", updatedInstalled.InstalledFrom)
}

// TestUpdateArtifact_NotInstalled tests updating a non-existent artifact
func TestUpdateArtifact_NotInstalled(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), filepath.Join(tempDir, "installed.db"))

	// Try to update a non-existent artifact
	testArtifact := filepath.Join(tempDir, "nonexistent.gotya")

	setupTestArtifact(t, testArtifact, true, DefaultMetadata)

	err := mgr.UpdateArtifact(context.Background(), testArtifact, DefaultIndexArtifactDescriptor)
	require.Error(t, err)
	assert.ErrorContains(t, err, "not installed")
}

// Helper function to test updating to the same version and URL (should fail)
// Parameters:
// - testName: Name suffix for the test artifact to avoid conflicts
// - description: Description for the artifact metadata
func testUpdateToSameVersion(t *testing.T, testName, description string) {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), dbPath)
	artifactName := "test-artifact-" + testName

	// Create and install the original version
	originalArtifact := filepath.Join(tempDir, "original.gotya")
	originalMetadata := &Metadata{
		Name:        artifactName,
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: description,
	}
	setupTestArtifact(t, originalArtifact, true, originalMetadata)

	originalDesc := &model.IndexArtifactDescriptor{
		Name:    artifactName,
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/v1.0.0.gotya",
	}

	// Install the original version
	err := mgr.InstallArtifact(context.Background(), originalDesc, originalArtifact, model.InstallationReasonManual)
	require.NoError(t, err)

	// Try to update to the same version and URL
	sameDesc := &model.IndexArtifactDescriptor{
		Name:    artifactName,
		Version: "1.0.0", // Same version
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/v1.0.0.gotya", // Same URL
	}

	err = mgr.UpdateArtifact(context.Background(), originalArtifact, sameDesc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already at the latest version")
}

// TestUpdateArtifact_InvalidNewArtifact tests updating with an invalid new artifact
func TestUpdateArtifact_InvalidNewArtifact(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), dbPath)
	artifactName := "test-artifact"

	// Create and install the original version
	originalArtifact := filepath.Join(tempDir, "original.gotya")
	originalMetadata := &Metadata{
		Name:        artifactName,
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Test version",
	}
	setupTestArtifact(t, originalArtifact, true, originalMetadata)

	originalDesc := &model.IndexArtifactDescriptor{
		Name:    artifactName,
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/v1.0.0.gotya",
	}

	// Install the original version
	err := mgr.InstallArtifact(context.Background(), originalDesc, originalArtifact, model.InstallationReasonManual)
	require.NoError(t, err)

	// Try to update with an invalid artifact
	invalidDesc := &model.IndexArtifactDescriptor{
		Name:    artifactName,
		Version: "2.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/v2.0.0.gotya",
	}

	err = mgr.UpdateArtifact(context.Background(), "/nonexistent/path.gotya", invalidDesc)
	require.Error(t, err)
	var pathError *os.PathError
	assert.ErrorAs(t, err, &pathError)
}

// TestUpdateArtifact_EmptyNewArtifactPath tests updating with empty new artifact path
func TestUpdateArtifact_EmptyNewArtifactPath(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), filepath.Join(tempDir, "installed.db"))

	desc := &model.IndexArtifactDescriptor{
		Name:    "test-artifact",
		Version: "2.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/v2.0.0.gotya",
	}

	err := mgr.UpdateArtifact(context.Background(), "", desc)
	require.Error(t, err)
	assert.ErrorIs(t, err, errors.ErrValidation)
}

// TestUpdateArtifact_EmptyDescriptor tests updating with nil descriptor
func TestUpdateArtifact_EmptyDescriptor(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), filepath.Join(tempDir, "installed.db"))

	err := mgr.UpdateArtifact(context.Background(), "/path/to/artifact.gotya", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, errors.ErrValidation)
}

// TestUpdateArtifact_DifferentArtifactName tests updating with mismatched artifact name
func TestUpdateArtifact_DifferentArtifactName(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), dbPath)
	artifactName := "test-artifact"

	// Create and install the original version
	originalArtifact := filepath.Join(tempDir, "original.gotya")
	originalMetadata := &Metadata{
		Name:        artifactName,
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Original version",
	}
	setupTestArtifact(t, originalArtifact, true, originalMetadata)

	originalDesc := &model.IndexArtifactDescriptor{
		Name:    artifactName,
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/v1.0.0.gotya",
	}

	// Install the original version
	err := mgr.InstallArtifact(context.Background(), originalDesc, originalArtifact, model.InstallationReasonManual)
	require.NoError(t, err)

	// Create an updated version with different name (should fail)
	updatedArtifact := filepath.Join(tempDir, "updated.gotya")
	updatedMetadata := &Metadata{
		Name:        "different-artifact", // Different name
		Version:     "2.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Updated version",
	}
	setupTestArtifact(t, updatedArtifact, true, updatedMetadata)

	updatedDesc := &model.IndexArtifactDescriptor{
		Name:    "different-artifact", // Different name
		Version: "2.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/v2.0.0.gotya",
	}

	// Update should fail because the artifact is not installed
	err = mgr.UpdateArtifact(context.Background(), updatedArtifact, updatedDesc)
	if err == nil {
		t.Errorf("Expected error because artifact is not installed but got nil")
		return
	}
	assert.Contains(t, err.Error(), "artifact different-artifact is not installed")
}

// TestUpdateArtifact_DowngradeVersion tests updating to a lower version
func TestUpdateArtifact_DowngradeVersion(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), dbPath)
	artifactName := "test-artifact"

	// Create and install version 2.0.0 first
	originalArtifact := filepath.Join(tempDir, "original.gotya")
	originalMetadata := &Metadata{
		Name:        artifactName,
		Version:     "2.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Original version",
	}
	setupTestArtifact(t, originalArtifact, true, originalMetadata)

	originalDesc := &model.IndexArtifactDescriptor{
		Name:    artifactName,
		Version: "2.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/v2.0.0.gotya",
	}

	// Install the original version
	err := mgr.InstallArtifact(context.Background(), originalDesc, originalArtifact, model.InstallationReasonManual)
	require.NoError(t, err)

	// Create a "downgrade" version (1.0.0)
	downgradeArtifact := filepath.Join(tempDir, "downgrade.gotya")
	downgradeMetadata := &Metadata{
		Name:        artifactName,
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Downgrade version",
	}
	setupTestArtifact(t, downgradeArtifact, true, downgradeMetadata)

	downgradeDesc := &model.IndexArtifactDescriptor{
		Name:    artifactName,
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/v1.0.0.gotya",
	}

	// Update to lower version should succeed (no version comparison logic)
	err = mgr.UpdateArtifact(context.Background(), downgradeArtifact, downgradeDesc)
	require.NoError(t, err)

	// Verify the downgrade was successful
	db := loadInstalledDB(t, dbPath)
	assert.True(t, db.IsArtifactInstalled(artifactName), "downgraded artifact should be installed")
	updatedInstalled := db.FindArtifact(artifactName)
	require.NotNil(t, updatedInstalled)
	assert.Equal(t, "1.0.0", updatedInstalled.Version)
}

// TestUpdateArtifact_SameURLDifferentVersion tests updating with same URL but different version
func TestUpdateArtifact_SameURLDifferentVersion(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), dbPath)
	artifactName := "test-artifact"

	// Create and install the original version
	originalArtifact := filepath.Join(tempDir, "original.gotya")
	originalMetadata := &Metadata{
		Name:        artifactName,
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Original version",
	}
	setupTestArtifact(t, originalArtifact, true, originalMetadata)

	originalDesc := &model.IndexArtifactDescriptor{
		Name:    artifactName,
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/v1.0.0.gotya",
	}

	// Install the original version
	err := mgr.InstallArtifact(context.Background(), originalDesc, originalArtifact, model.InstallationReasonManual)
	require.NoError(t, err)

	// Create an updated version with same URL but different version
	updatedArtifact := filepath.Join(tempDir, "updated.gotya")
	updatedMetadata := &Metadata{
		Name:        artifactName,
		Version:     "2.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Updated version",
	}
	setupTestArtifact(t, updatedArtifact, true, updatedMetadata)

	updatedDesc := &model.IndexArtifactDescriptor{
		Name:    artifactName,
		Version: "2.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/v1.0.0.gotya", // Same URL
	}

	// Update should succeed
	err = mgr.UpdateArtifact(context.Background(), updatedArtifact, updatedDesc)
	require.NoError(t, err)

	// Verify the update was successful
	db := loadInstalledDB(t, dbPath)
	assert.True(t, db.IsArtifactInstalled(artifactName), "updated artifact should be installed")
	updatedInstalled := db.FindArtifact(artifactName)
	require.NotNil(t, updatedInstalled)
	assert.Equal(t, "2.0.0", updatedInstalled.Version)
	assert.Equal(t, "http://example.com/v1.0.0.gotya", updatedInstalled.InstalledFrom)
}

// TestUpdateArtifact_ForceReinstall tests updating to the exact same version and URL
func TestUpdateArtifact_ForceReinstall(t *testing.T) {
	testUpdateToSameVersion(t, "force-reinstall", "Original version")
}

// TestVerifyArtifact_CachedFile tests verifying an artifact from cache directory
func TestVerifyArtifact_CachedFile(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))
	mgr := NewManager("linux", "amd64", cacheDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), filepath.Join(tempDir, "installed.db"))

	// Create a test artifact
	testArtifact := filepath.Join(tempDir, "test-artifact.gotya")
	metadata := &Metadata{
		Name:        "test-artifact",
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Test artifact for verification tests",
	}
	setupTestArtifact(t, testArtifact, true, metadata)

	// Copy the artifact to the cache directory
	cacheArtifact := filepath.Join(cacheDir, "test-artifact_1.0.0_linux_amd64.gotya")
	require.NoError(t, os.Rename(testArtifact, cacheArtifact))

	desc := &model.IndexArtifactDescriptor{
		Name:    "test-artifact",
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
	}

	// Verify the artifact
	err := mgr.VerifyArtifact(context.Background(), desc)
	require.NoError(t, err)
}

// TestVerifyArtifact_NonExistentFile tests verifying a non-existent cached artifact
func TestVerifyArtifact_NonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), filepath.Join(tempDir, "installed.db"))

	desc := &model.IndexArtifactDescriptor{
		Name:    "non-existent-artifact",
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
	}

	// Try to verify a non-existent artifact
	err := mgr.VerifyArtifact(context.Background(), desc)
	require.Error(t, err)
	assert.Equal(t, errors.ErrArtifactNotFound, err)
}

// TestVerifyArtifact_InvalidMetadata tests verifying an artifact with mismatched metadata
func TestVerifyArtifact_InvalidMetadata(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0755))
	mgr := NewManager("linux", "amd64", cacheDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), filepath.Join(tempDir, "installed.db"))

	// Create a test artifact with specific metadata
	testArtifact := filepath.Join(tempDir, "test-artifact.gotya")
	metadata := &Metadata{
		Name:        "test-artifact",
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Test artifact for verification tests",
	}
	setupTestArtifact(t, testArtifact, true, metadata)

	// Copy the artifact to the cache directory with the name that the mismatched descriptor expects
	cacheArtifact := filepath.Join(cacheDir, "different-artifact_1.0.0_linux_amd64.gotya")
	require.NoError(t, os.Rename(testArtifact, cacheArtifact))

	// Try to verify with mismatched metadata (different name)
	desc := &model.IndexArtifactDescriptor{
		Name:    "different-artifact", // Different name - should cause metadata mismatch
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
	}

	err := mgr.VerifyArtifact(context.Background(), desc)
	require.Error(t, err)
	assert.ErrorContains(t, err, "metadata mismatch")
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
	outputDir := filepath.Join(tempDir, "output")
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

// loadInstalledDB loads the installed database from the given path
func loadInstalledDB(t *testing.T, dbPath string) *database.InstalledManagerImpl {
	t.Helper()
	db := database.NewInstalledManger()
	err := db.LoadDatabaseFrom(dbPath)
	require.NoError(t, err, "failed to load installed database")
	return db
}

// setupTestDatabaseWithArtifacts creates a test database with the specified artifacts
func setupTestDatabaseWithArtifacts(t *testing.T, dbPath string, artifacts []*model.InstalledArtifact) {
	t.Helper()
	db := database.NewInstalledManger()
	for _, artifact := range artifacts {
		db.AddArtifact(artifact)
	}
	err := db.SaveDatabaseTo(dbPath)
	require.NoError(t, err, "failed to save test database")
}

// createTestInstalledArtifact creates a basic test artifact for database testing
func createTestInstalledArtifact(t *testing.T, name, version string, reverseDeps []string) *model.InstalledArtifact {
	t.Helper()
	return &model.InstalledArtifact{
		Name:                name,
		Version:             version,
		Description:         fmt.Sprintf("Test artifact %s", name),
		InstalledAt:         time.Now(),
		InstalledFrom:       "http://example.com/test.gotya",
		ArtifactMetaDir:     "/test/meta",
		ArtifactDataDir:     "/test/data",
		MetaFiles:           []model.InstalledFile{{Path: "artifact.json", Hash: "abc123"}},
		DataFiles:           []model.InstalledFile{{Path: "data.bin", Hash: "def456"}},
		ReverseDependencies: reverseDeps,
		Status:              model.StatusInstalled,
		Checksum:            "checksum123",
		InstallationReason:  model.InstallationReasonManual,
	}
}

// TestReverseResolve_Basic tests basic reverse dependency resolution
func TestReverseResolve_Basic(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), dbPath)

	// Create test artifacts with reverse dependencies
	dependency := createTestInstalledArtifact(t, "dep1", "1.0.0", []string{})
	mainArtifact := createTestInstalledArtifact(t, "main", "1.0.0", []string{"dep1"})

	setupTestDatabaseWithArtifacts(t, dbPath, []*model.InstalledArtifact{dependency, mainArtifact})

	// Test resolving reverse dependencies for dep1
	req := model.ResolveRequest{
		Name:              "dep1",
		VersionConstraint: "1.0.0",
		OS:                "linux",
		Arch:              "amd64",
	}

	result, err := mgr.ReverseResolve(context.Background(), req)
	require.NoError(t, err)

	// Should find main artifact as reverse dependency
	require.Len(t, result.Artifacts, 1)
	assert.Equal(t, "main", result.Artifacts[0].Name)
	assert.Equal(t, "1.0.0", result.Artifacts[0].Version)
	assert.Equal(t, "linux", result.Artifacts[0].OS)
	assert.Equal(t, "amd64", result.Artifacts[0].Arch)
	assert.Equal(t, "checksum123", result.Artifacts[0].Checksum)
}

// TestReverseResolve_ComplexDependencies tests reverse dependency resolution with complex dependency graph
func TestReverseResolve_ComplexDependencies(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), dbPath)

	// Create a complex dependency graph:
	// app -> libA -> core
	// app -> libB -> core
	// tool -> libA
	core := createTestInstalledArtifact(t, "core", "1.0.0", []string{})
	libA := createTestInstalledArtifact(t, "libA", "1.0.0", []string{"core"})
	libB := createTestInstalledArtifact(t, "libB", "1.0.0", []string{"core"})
	app := createTestInstalledArtifact(t, "app", "1.0.0", []string{"libA", "libB"})
	tool := createTestInstalledArtifact(t, "tool", "1.0.0", []string{"libA"})

	setupTestDatabaseWithArtifacts(t, dbPath, []*model.InstalledArtifact{core, libA, libB, app, tool})

	// Test resolving reverse dependencies for core
	req := model.ResolveRequest{
		Name:              "core",
		VersionConstraint: "1.0.0",
		OS:                "linux",
		Arch:              "amd64",
	}

	result, err := mgr.ReverseResolve(context.Background(), req)
	require.NoError(t, err)

	// Should find libA, libB, app, tool as reverse dependencies (no specific order guaranteed)
	require.Len(t, result.Artifacts, 4)

	artifactNames := make(map[string]bool)
	for _, artifact := range result.Artifacts {
		artifactNames[artifact.Name] = true
	}

	expectedNames := []string{"libA", "libB", "app", "tool"}
	for _, expected := range expectedNames {
		assert.True(t, artifactNames[expected], "expected artifact %s not found in reverse dependencies", expected)
	}
}

// TestReverseResolve_NonExistentArtifact tests reverse dependency resolution for non-existent artifact
func TestReverseResolve_NonExistentArtifact(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), dbPath)

	// Create empty database
	setupTestDatabaseWithArtifacts(t, dbPath, []*model.InstalledArtifact{})

	// Test resolving reverse dependencies for non-existent artifact
	req := model.ResolveRequest{
		Name:              "nonexistent",
		VersionConstraint: "1.0.0",
		OS:                "linux",
		Arch:              "amd64",
	}

	result, err := mgr.ReverseResolve(context.Background(), req)
	require.NoError(t, err)

	// Should return empty result
	assert.Empty(t, result.Artifacts)
}

// TestReverseResolve_DatabaseLoadError tests error handling when database loading fails
func TestReverseResolve_DatabaseLoadError(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "corrupted.db")

	// Create a corrupted database file (invalid JSON)
	require.NoError(t, os.WriteFile(dbPath, []byte("invalid json content"), 0644))

	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), dbPath)

	// Test resolving reverse dependencies when database is corrupted
	req := model.ResolveRequest{
		Name:              "test",
		VersionConstraint: "1.0.0",
		OS:                "linux",
		Arch:              "amd64",
	}

	_, err := mgr.ReverseResolve(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load installed database")
}

// TestReverseResolve_EmptyDatabase tests reverse dependency resolution with empty database
func TestReverseResolve_EmptyDatabase(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "empty.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), dbPath)

	// Create empty database
	setupTestDatabaseWithArtifacts(t, dbPath, []*model.InstalledArtifact{})

	// Test resolving reverse dependencies
	req := model.ResolveRequest{
		Name:              "any-artifact",
		VersionConstraint: "1.0.0",
		OS:                "linux",
		Arch:              "amd64",
	}

	result, err := mgr.ReverseResolve(context.Background(), req)
	require.NoError(t, err)

	// Should return empty result
	assert.Empty(t, result.Artifacts)
}

// TestReverseResolve_SelfDependency tests reverse dependency resolution when artifact has itself as dependency
func TestReverseResolve_SelfDependency(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), dbPath)

	// Create artifact with self-dependency (edge case)
	artifact := createTestInstalledArtifact(t, "self", "1.0.0", []string{"self"})
	setupTestDatabaseWithArtifacts(t, dbPath, []*model.InstalledArtifact{artifact})

	// Test resolving reverse dependencies for self
	req := model.ResolveRequest{
		Name:              "self",
		VersionConstraint: "1.0.0",
		OS:                "linux",
		Arch:              "amd64",
	}

	result, err := mgr.ReverseResolve(context.Background(), req)
	require.NoError(t, err)

	// Should find self as reverse dependency (due to DFS cycle detection)
	require.Len(t, result.Artifacts, 1)
	assert.Equal(t, "self", result.Artifacts[0].Name)
}

// TestReverseResolve_MissingStatusArtifact tests reverse dependency resolution for artifact with missing status
func TestReverseResolve_MissingStatusArtifact(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), dbPath)

	// Create artifact with missing status
	missingArtifact := &model.InstalledArtifact{
		Name:                "missing",
		Version:             "1.0.0",
		Description:         "Missing artifact",
		InstalledAt:         time.Now(),
		InstalledFrom:       "http://example.com/missing.gotya",
		ArtifactMetaDir:     "/test/meta",
		ArtifactDataDir:     "/test/data",
		MetaFiles:           []model.InstalledFile{},
		DataFiles:           []model.InstalledFile{},
		ReverseDependencies: []string{"main"},
		Status:              model.StatusMissing,
		Checksum:            "checksum123",
		InstallationReason:  model.InstallationReasonAutomatic,
	}
	mainArtifact := createTestInstalledArtifact(t, "main", "1.0.0", []string{})

	setupTestDatabaseWithArtifacts(t, dbPath, []*model.InstalledArtifact{missingArtifact, mainArtifact})

	// Test resolving reverse dependencies for missing artifact
	req := model.ResolveRequest{
		Name:              "missing",
		VersionConstraint: "1.0.0",
		OS:                "linux",
		Arch:              "amd64",
	}

	result, err := mgr.ReverseResolve(context.Background(), req)
	require.NoError(t, err)

	// Should return empty result since missing artifacts are not included
	assert.Empty(t, result.Artifacts)
}

func TestInstallArtifact_InstallationReasonTransitions(t *testing.T) {
	tests := []struct {
		name           string
		reason         model.InstallationReason
		previousReason model.InstallationReason
		expectedReason model.InstallationReason
	}{{
		name:           "New to manual",
		reason:         model.InstallationReasonManual,
		expectedReason: model.InstallationReasonManual,
	}, {
		name:           "New to automatic",
		reason:         model.InstallationReasonAutomatic,
		expectedReason: model.InstallationReasonAutomatic,
	}, {
		name:           "Manual to automatic",
		reason:         model.InstallationReasonAutomatic,
		previousReason: model.InstallationReasonManual,
		expectedReason: model.InstallationReasonManual,
	}, {
		name:           "Automatic to manual",
		reason:         model.InstallationReasonManual,
		previousReason: model.InstallationReasonAutomatic,
		expectedReason: model.InstallationReasonManual,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			artifact := &model.IndexArtifactDescriptor{
				Name:    DefaultMetadata.Name,
				Version: DefaultMetadata.Version,
				URL:     "http://example.com/test.gotya",
				OS:      DefaultArtifactOS,
				Arch:    DefaultArtifactArch,
			}

			dbPath := filepath.Join(tempDir, "installed.db")
			if tt.previousReason != "" {
				db := loadInstalledDB(t, dbPath)
				DefaultInstalledArtifact.InstallationReason = tt.previousReason
				DefaultInstalledArtifact.Status = model.StatusInstalled
				db.AddArtifact(DefaultInstalledArtifact)
				require.NoError(t, db.SaveDatabaseTo(dbPath))
			}

			setupTestArtifact(t, filepath.Join(tempDir, "test-artifact.gotya"), false, DefaultMetadata)
			mgr := NewManager("linux", "amd64", filepath.Join(tempDir, "install"), filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), dbPath)

			require.NoError(t, mgr.InstallArtifact(context.Background(), artifact, filepath.Join(tempDir, "test-artifact.gotya"), tt.reason))

			artifacts, err := mgr.GetInstalledArtifacts()
			require.NoError(t, err)

			require.Len(t, artifacts, 1)
			require.Equal(t, tt.expectedReason, artifacts[0].InstallationReason)
		})
	}
}

// TestInstallArtifact_InstallationReason_DatabasePersistence tests that installation reason is persisted in database
func TestInstallArtifact_InstallationReason_DatabasePersistence(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), dbPath)
	artifactName := "test-artifact"

	// Create and install an artifact
	testArtifact := filepath.Join(tempDir, "test-artifact.gotya")
	metadata := &Metadata{
		Name:        artifactName,
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Test artifact for database persistence",
	}
	setupTestArtifact(t, testArtifact, true, metadata)

	desc := &model.IndexArtifactDescriptor{
		Name:    artifactName,
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/test.gotya",
	}

	// Install with manual reason
	err := mgr.InstallArtifact(context.Background(), desc, testArtifact, model.InstallationReasonManual)
	require.NoError(t, err)

	// Verify it was installed with manual reason
	db := loadInstalledDB(t, dbPath)
	installedArtifact := db.FindArtifact(artifactName)
	require.NotNil(t, installedArtifact)
	assert.Equal(t, model.InstallationReasonManual, installedArtifact.InstallationReason, "should have manual reason in memory")

	// Load database directly to test persistence
	db2 := loadInstalledDB(t, dbPath) // This will load from the saved database
	reloadedArtifact := db2.FindArtifact(artifactName)
	require.NotNil(t, reloadedArtifact)
	assert.Equal(t, model.InstallationReasonManual, reloadedArtifact.InstallationReason, "should have manual reason persisted in database")
}

// TestGetOrphanedAutomaticArtifacts_NoOrphaned tests when there are no orphaned automatic artifacts
func TestGetOrphanedAutomaticArtifacts_NoOrphaned(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), dbPath)

	// Create test artifacts with different scenarios
	manualArtifact := createTestInstalledArtifact(t, "manual", "1.0.0", []string{"dep1"})
	manualArtifact.InstallationReason = model.InstallationReasonManual

	automaticWithDeps := createTestInstalledArtifact(t, "auto-with-deps", "1.0.0", []string{"dep1"})
	automaticWithDeps.InstallationReason = model.InstallationReasonAutomatic

	automaticNoDeps := createTestInstalledArtifact(t, "auto-no-deps", "1.0.0", []string{})
	automaticNoDeps.InstallationReason = model.InstallationReasonAutomatic

	setupTestDatabaseWithArtifacts(t, dbPath, []*model.InstalledArtifact{manualArtifact, automaticWithDeps, automaticNoDeps})

	// Get orphaned artifacts
	orphaned, err := mgr.GetOrphanedAutomaticArtifacts()
	require.NoError(t, err)

	// Should find the orphaned automatic artifact (auto-no-deps has no reverse dependencies)
	require.Len(t, orphaned, 1)
	assert.Contains(t, orphaned, "auto-no-deps")
}

// TestGetOrphanedAutomaticArtifacts_WithOrphaned tests finding orphaned automatic artifacts
func TestGetOrphanedAutomaticArtifacts_WithOrphaned(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), dbPath)

	// Create test artifacts
	manualArtifact := createTestInstalledArtifact(t, "manual", "1.0.0", []string{"dep1"})
	manualArtifact.InstallationReason = model.InstallationReasonManual

	automaticOrphaned1 := createTestInstalledArtifact(t, "auto-orphan1", "1.0.0", []string{})
	automaticOrphaned1.InstallationReason = model.InstallationReasonAutomatic

	automaticOrphaned2 := createTestInstalledArtifact(t, "auto-orphan2", "1.0.0", []string{})
	automaticOrphaned2.InstallationReason = model.InstallationReasonAutomatic

	automaticWithDeps := createTestInstalledArtifact(t, "auto-with-deps", "1.0.0", []string{"dep1"})
	automaticWithDeps.InstallationReason = model.InstallationReasonAutomatic

	setupTestDatabaseWithArtifacts(t, dbPath, []*model.InstalledArtifact{manualArtifact, automaticOrphaned1, automaticOrphaned2, automaticWithDeps})

	// Get orphaned artifacts
	orphaned, err := mgr.GetOrphanedAutomaticArtifacts()
	require.NoError(t, err)

	// Should find the two orphaned automatic artifacts
	require.Len(t, orphaned, 2)
	assert.Contains(t, orphaned, "auto-orphan1")
	assert.Contains(t, orphaned, "auto-orphan2")
	assert.NotContains(t, orphaned, "manual")
	assert.NotContains(t, orphaned, "auto-with-deps")
}

// TestGetOrphanedAutomaticArtifacts_OnlyAutomatic tests that only automatic artifacts are considered
func TestGetOrphanedAutomaticArtifacts_OnlyAutomatic(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), dbPath)

	// Create test artifacts
	manualOrphaned := createTestInstalledArtifact(t, "manual-orphan", "1.0.0", []string{})
	manualOrphaned.InstallationReason = model.InstallationReasonManual

	automaticNotOrphaned := createTestInstalledArtifact(t, "auto-not-orphan", "1.0.0", []string{"dep1"})
	automaticNotOrphaned.InstallationReason = model.InstallationReasonAutomatic

	setupTestDatabaseWithArtifacts(t, dbPath, []*model.InstalledArtifact{manualOrphaned, automaticNotOrphaned})

	// Get orphaned artifacts
	orphaned, err := mgr.GetOrphanedAutomaticArtifacts()
	require.NoError(t, err)

	// Should be empty since manual artifacts are ignored and automatic artifact has dependencies
	assert.Empty(t, orphaned)
}

// TestGetOrphanedAutomaticArtifacts_MissingStatus tests that missing artifacts are ignored
func TestGetOrphanedAutomaticArtifacts_MissingStatus(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), dbPath)

	// Create test artifacts
	missingAutomatic := createTestInstalledArtifact(t, "missing-auto", "1.0.0", []string{})
	missingAutomatic.InstallationReason = model.InstallationReasonAutomatic
	missingAutomatic.Status = model.StatusMissing

	installedAutomatic := createTestInstalledArtifact(t, "installed-auto", "1.0.0", []string{})
	installedAutomatic.InstallationReason = model.InstallationReasonAutomatic
	installedAutomatic.Status = model.StatusInstalled

	setupTestDatabaseWithArtifacts(t, dbPath, []*model.InstalledArtifact{missingAutomatic, installedAutomatic})

	// Get orphaned artifacts
	orphaned, err := mgr.GetOrphanedAutomaticArtifacts()
	require.NoError(t, err)

	// Should only find the installed automatic artifact
	require.Len(t, orphaned, 1)
	assert.Contains(t, orphaned, "installed-auto")
	assert.NotContains(t, orphaned, "missing-auto")
}

// TestGetOrphanedAutomaticArtifacts_DatabaseLoadError tests error when database cannot be loaded
func TestGetOrphanedAutomaticArtifacts_DatabaseLoadError(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "corrupted.db")

	// Create a corrupted database file
	require.NoError(t, os.WriteFile(dbPath, []byte("invalid json"), 0644))

	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), dbPath)

	// Try to get orphaned artifacts
	orphaned, err := mgr.GetOrphanedAutomaticArtifacts()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load installed database")
	require.Nil(t, orphaned)
}

// TestGetOrphanedAutomaticArtifacts_EmptyDatabase tests with an empty database
func TestGetOrphanedAutomaticArtifacts_EmptyDatabase(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "empty.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), dbPath)

	// Create empty database
	setupTestDatabaseWithArtifacts(t, dbPath, []*model.InstalledArtifact{})

	// Get orphaned artifacts
	orphaned, err := mgr.GetOrphanedAutomaticArtifacts()
	require.NoError(t, err)

	// Should be empty
	assert.Empty(t, orphaned)
}

func TestUpdateArtifact_HookBehavior(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Test that UpdateArtifact only triggers update hooks, not install/uninstall hooks
	tempDir := t.TempDir()
	metaDir := filepath.Join(tempDir, "install", artifactMetaDir)
	dataDir := filepath.Join(tempDir, "install", artifactDataDir)

	// Create directories
	err := os.MkdirAll(metaDir, 0o755)
	require.NoError(t, err)

	// Create hook scripts
	hooksDir := metaDir
	err = os.MkdirAll(hooksDir, 0o755)
	require.NoError(t, err)

	// Create all hook scripts
	hookScripts := []string{"pre-update", "post-update"}
	for _, hookName := range hookScripts {
		hookPath := filepath.Join(hooksDir, hookName+".tengo")
		err := os.WriteFile(hookPath, []byte(`// Tengo script`), 0o644)
		require.NoError(t, err)
	}

	// Create mock hook executor to track calls
	mockHookExecutor := NewMockHookExecutor(ctrl)

	// Set up expectations - UpdateArtifact should only call update hooks
	preUpdateContext := &HookContext{
		ArtifactName:    DefaultArtifactName,
		ArtifactVersion: "2.0.0", // New version, not old version
		Operation:       "update",
		MetaDir:         filepath.Join(metaDir, "test-artifact"), // Uses installed artifact's MetaDir
		DataDir:         filepath.Join(dataDir, "test-artifact"), // Uses installed artifact's DataDir (should match getArtifactDataInstallPath)
		OldVersion:      DefaultArtifactVersion,
	}
	postUpdateContext := &HookContext{
		ArtifactName:    DefaultArtifactName,
		ArtifactVersion: "2.0.0", // New version
		Operation:       "update",
		MetaDir:         filepath.Join(metaDir, "test-artifact"), // Uses new artifact's MetaDir (same as old in this case)
		DataDir:         filepath.Join(dataDir, "test-artifact"), // Uses new artifact's DataDir
		OldVersion:      DefaultArtifactVersion,
	}

	// Expect pre-update hook call (before uninstall)
	mockHookExecutor.EXPECT().
		ExecuteHook(filepath.Join(metaDir, "test-artifact", "pre-update.tengo"), preUpdateContext).
		Return(nil)

	// Expect post-update hook call (after install)
	mockHookExecutor.EXPECT().
		ExecuteHook(filepath.Join(metaDir, "test-artifact", "post-update.tengo"), postUpdateContext).
		Return(nil)
	// Create manager with mock hook executor
	mgr := NewManager("linux", "amd64", tempDir, dataDir, metaDir, filepath.Join(tempDir, "installed.db"))
	mgr.hookExecutor = mockHookExecutor

	// Setup test database with an installed artifact
	dbPath := filepath.Join(tempDir, "installed.db")
	installedArtifact := &model.InstalledArtifact{
		Name:                DefaultArtifactName,
		Version:             DefaultArtifactVersion,
		OS:                  DefaultArtifactOS,
		Arch:                DefaultArtifactArch,
		ArtifactMetaDir:     filepath.Join(metaDir, "test-artifact"),
		ArtifactDataDir:     filepath.Join(dataDir, "test-artifact"), // Should match getArtifactDataInstallPath
		Status:              model.StatusInstalled,
		InstallationReason:  model.InstallationReasonManual,
		InstalledAt:         time.Now(),
		InstalledFrom:       "test://test",
		Checksum:            "test-checksum",
		MetaFiles:           []model.InstalledFile{},
		DataFiles:           []model.InstalledFile{},
		ReverseDependencies: []string{},
	}
	setupTestDatabaseWithArtifacts(t, dbPath, []*model.InstalledArtifact{installedArtifact})

	DefaultMetadata.Hooks["pre-update"] = "pre-update.tengo"

	writeMetadata(t, filepath.Join(metaDir, "test-artifact"), DefaultMetadata)

	// Create new artifact descriptor for update
	newDesc := &model.IndexArtifactDescriptor{
		Name:    DefaultArtifactName,
		Version: "2.0.0", // Different version
		OS:      DefaultArtifactOS,
		Arch:    DefaultArtifactArch,
		URL:     "test://test-v2",
	}

	// Create a proper artifact file for update
	artifactPath := filepath.Join(tempDir, "test-artifact_2.0.0_linux_amd64.gotya")
	metadata := &Metadata{
		Name:        DefaultArtifactName,
		Version:     "2.0.0",
		OS:          DefaultArtifactOS,
		Arch:        DefaultArtifactArch,
		Maintainer:  "test@example.com",
		Description: "Updated test artifact",
		Hooks: map[string]string{
			"post-update": "post-update.tengo",
		},
	}
	setupTestArtifact(t, artifactPath, true, metadata)

	// Perform update - this should only trigger update hooks
	err = mgr.UpdateArtifact(context.Background(), artifactPath, newDesc)
	require.NoError(t, err)

	// Verify that the mock expectations were met
	ctrl.Finish()
}

// TestUninstallArtifact_HookBehavior verifies that UninstallArtifact only calls uninstall hooks
func TestUninstallArtifact_HookBehavior(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create temp directories for test
	tempDir := t.TempDir()
	metaDir := filepath.Join(tempDir, "meta")
	dataDir := filepath.Join(tempDir, "data")

	// Create hook directories and scripts
	hookScripts := []string{"pre-uninstall", "post-uninstall"}
	for _, hookName := range hookScripts {
		hookPath := filepath.Join(metaDir, "test-artifact", hookName+".tengo")
		err := os.MkdirAll(filepath.Dir(hookPath), 0o755)
		require.NoError(t, err)
		err = os.WriteFile(hookPath, []byte(`true`), 0o644)
		require.NoError(t, err)
	}

	// Create mock hook executor to track calls
	mockHookExecutor := NewMockHookExecutor(ctrl)

	// Set up expectations - UninstallArtifact should only call uninstall hooks
	preUninstallContext := &HookContext{
		ArtifactName:    DefaultArtifactName,
		ArtifactVersion: DefaultArtifactVersion,
		Operation:       "uninstall",
		MetaDir:         filepath.Join(metaDir, "test-artifact"),
		DataDir:         filepath.Join(dataDir, "test-artifact"),
	}
	postUninstallContext := &HookContext{
		ArtifactName:    DefaultArtifactName,
		ArtifactVersion: DefaultArtifactVersion,
		Operation:       "uninstall",
		WasMetaDir:      filepath.Join(metaDir, "test-artifact"),
		WasDataDir:      filepath.Join(dataDir, "test-artifact"),
	}

	DefaultMetadata.Hooks["pre-uninstall"] = "pre-uninstall.tengo"
	DefaultMetadata.Hooks["post-uninstall"] = "post-uninstall.tengo"

	writeMetadata(t, filepath.Join(metaDir, "test-artifact"), DefaultMetadata)

	file, err := os.Create(filepath.Join(metaDir, "test-artifact", "post-uninstall.tengo"))
	require.NoError(t, err)
	_, err = file.Write([]byte(`true`))
	require.NoError(t, err)
	require.NoError(t, file.Close())

	// Expect pre-uninstall hook call
	gomock.InOrder(
		mockHookExecutor.EXPECT().
			ExecuteHook(filepath.Join(metaDir, "test-artifact", "pre-uninstall.tengo"), preUninstallContext).
			Return(nil),

		mockHookExecutor.EXPECT().
			ExecuteHook(gomock.Any(), postUninstallContext).
			Return(nil),
	)

	// Create manager with mock hook executor
	mgr := NewManager("linux", "amd64", tempDir, dataDir, metaDir, filepath.Join(tempDir, "installed.db"))
	mgr.hookExecutor = mockHookExecutor

	// Setup test database with an installed artifact
	dbPath := filepath.Join(tempDir, "installed.db")
	installedArtifact := &model.InstalledArtifact{
		Name:                DefaultArtifactName,
		Version:             DefaultArtifactVersion,
		OS:                  DefaultArtifactOS,
		Arch:                DefaultArtifactArch,
		ArtifactMetaDir:     filepath.Join(metaDir, "test-artifact"),
		ArtifactDataDir:     filepath.Join(dataDir, "test-artifact"),
		Status:              model.StatusInstalled,
		InstallationReason:  model.InstallationReasonManual,
		InstalledAt:         time.Now(),
		InstalledFrom:       "test://test",
		Checksum:            "test-checksum",
		MetaFiles:           []model.InstalledFile{},
		DataFiles:           []model.InstalledFile{},
		ReverseDependencies: []string{},
	}
	setupTestDatabaseWithArtifacts(t, dbPath, []*model.InstalledArtifact{installedArtifact})

	// Perform uninstall - this should only trigger uninstall hooks
	require.NoError(t, mgr.UninstallArtifact(context.Background(), DefaultArtifactName, true))

	// Verify that the mock expectations were met
	ctrl.Finish()
}

// TestUpdateArtifact_FailingPreUpdateHook verifies that UpdateArtifact fails when pre-update hook fails
func TestUpdateArtifact_FailingPreUpdateHook(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create temp directories for test
	tempDir := t.TempDir()
	metaDir := filepath.Join(tempDir, "meta")
	dataDir := filepath.Join(tempDir, "data")

	// Create hook directories and failing script
	hookPath := filepath.Join(metaDir, "test-artifact", "pre-update.tengo")
	err := os.MkdirAll(filepath.Dir(hookPath), 0o755)
	require.NoError(t, err)
	// Create a script that will fail
	err = os.WriteFile(hookPath, []byte(`invalid tengo syntax !!!`), 0o644)
	require.NoError(t, err)

	DefaultMetadata.Hooks["pre-update"] = "pre-update.tengo"
	writeMetadata(t, filepath.Join(metaDir, "test-artifact"), DefaultMetadata)

	// Create mock hook executor to track calls
	mockHookExecutor := NewMockHookExecutor(ctrl)

	// Set up expectations - pre-update hook should fail
	preUpdateContext := &HookContext{
		ArtifactName:    DefaultArtifactName,
		ArtifactVersion: "2.0.0",
		Operation:       "update",
		MetaDir:         filepath.Join(metaDir, "test-artifact"),
		DataDir:         filepath.Join(dataDir, "test-artifact"),
		OldVersion:      DefaultArtifactVersion,
	}

	mockHookExecutor.EXPECT().
		ExecuteHook(filepath.Join(metaDir, "test-artifact", "pre-update.tengo"), preUpdateContext).
		Return(fmt.Errorf("hook script execution failed"))

	// Create manager with mock hook executor
	mgr := NewManager("linux", "amd64", tempDir, dataDir, metaDir, filepath.Join(tempDir, "installed.db"))
	mgr.hookExecutor = mockHookExecutor

	// Setup test database with an installed artifact
	dbPath := filepath.Join(tempDir, "installed.db")
	installedArtifact := &model.InstalledArtifact{
		Name:                DefaultArtifactName,
		Version:             DefaultArtifactVersion,
		OS:                  DefaultArtifactOS,
		Arch:                DefaultArtifactArch,
		ArtifactMetaDir:     filepath.Join(metaDir, "test-artifact"),
		ArtifactDataDir:     filepath.Join(dataDir, "test-artifact"),
		Status:              model.StatusInstalled,
		InstallationReason:  model.InstallationReasonManual,
		InstalledAt:         time.Now(),
		InstalledFrom:       "test://test",
		Checksum:            "test-checksum",
		MetaFiles:           []model.InstalledFile{},
		DataFiles:           []model.InstalledFile{},
		ReverseDependencies: []string{},
	}
	setupTestDatabaseWithArtifacts(t, dbPath, []*model.InstalledArtifact{installedArtifact})

	// Create new artifact descriptor for update
	newDesc := &model.IndexArtifactDescriptor{
		Name:    DefaultArtifactName,
		Version: "2.0.0",
		OS:      DefaultArtifactOS,
		Arch:    DefaultArtifactArch,
		URL:     "test://test-v2",
	}

	// Create a proper artifact file for update
	artifactPath := filepath.Join(tempDir, "test-artifact_2.0.0_linux_amd64.gotya")
	metadata := &Metadata{
		Name:        DefaultArtifactName,
		Version:     "2.0.0",
		OS:          DefaultArtifactOS,
		Arch:        DefaultArtifactArch,
		Maintainer:  "test@example.com",
		Description: "Updated test artifact",
	}
	setupTestArtifact(t, artifactPath, true, metadata)

	// Perform update - this should fail due to failing pre-update hook
	err = mgr.UpdateArtifact(context.Background(), artifactPath, newDesc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pre-update hook failed")

	// Verify that the mock expectations were met
	ctrl.Finish()
}

// TestInstallArtifact_MetadataDrivenHooks verifies that hooks are resolved using metadata
func TestInstallArtifact_MetadataDrivenHooks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create unique temp directories for this test
	tempDir := t.TempDir()
	installTempDir := filepath.Join(tempDir, "install")
	metaDir := filepath.Join(installTempDir, artifactMetaDir)
	dataDir := filepath.Join(installTempDir, artifactDataDir)

	// Create hook scripts with names that differ from hook types
	hookScripts := map[string]string{
		"pre-install":  "before_install.tengo", // Hook type "pre-install" maps to file "before_install.tengo"
		"post-install": "after_install.tengo",  // Hook type "post-install" maps to file "after_install.tengo"
	}

	metadata := &Metadata{
		Name:        "test-artifact",
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Test artifact with metadata-driven hooks",
		Hooks:       hookScripts,
	}

	// Create hook files in extracted directory (for pre-install hook execution)
	err := os.MkdirAll(metaDir, 0o755)
	require.NoError(t, err)

	for hookType, filename := range hookScripts {
		hookPath := filepath.Join(metaDir, filename)
		err := os.WriteFile(hookPath, []byte(`// Tengo script for `+hookType), 0o644)
		require.NoError(t, err)
	}

	// Create mock hook executor to track calls
	mockHookExecutor := NewMockHookExecutor(ctrl)

	// Set up expectations - should call hooks using metadata-resolved paths
	postInstallContext := &HookContext{
		ArtifactName:    "test-artifact",
		ArtifactVersion: "1.0.0",
		Operation:       "install",
		MetaDir:         filepath.Join(metaDir, "test-artifact"),
		DataDir:         filepath.Join(dataDir, "test-artifact"),
	}

	// Expect pre-install hook call using metadata-resolved path from extracted directory
	gomock.InOrder(
		mockHookExecutor.EXPECT().
			ExecuteHook(
				gomock.Cond(func(x string) bool { return strings.HasSuffix(x, "before_install.tengo") }),
				gomock.Cond(func(x *HookContext) bool {
					return x.ArtifactName == "test-artifact" &&
						x.ArtifactVersion == "1.0.0" &&
						x.Operation == "install" &&
						x.FinalMetaDir == filepath.Join(metaDir, "test-artifact") &&
						x.FinalDataDir == filepath.Join(dataDir, "test-artifact")
				})).
			Return(nil),

		mockHookExecutor.EXPECT().
			ExecuteHook(filepath.Join(metaDir, "test-artifact", "after_install.tengo"), postInstallContext).
			Return(nil),
	)

	// Create manager with mock hook executor
	mgr := NewManager("linux", "amd64", installTempDir, dataDir, metaDir, filepath.Join(tempDir, "installed.db"))
	mgr.hookExecutor = mockHookExecutor

	// Create test artifact descriptor
	desc := &model.IndexArtifactDescriptor{
		Name:    "test-artifact",
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "test://test",
	}

	// Create a proper artifact file
	artifactPath := filepath.Join(tempDir, "test-artifact_1.0.0_linux_amd64.gotya")
	setupTestArtifact(t, artifactPath, true, metadata)

	// Perform install - this should use metadata-driven hook resolution
	err = mgr.InstallArtifact(context.Background(), desc, artifactPath, model.InstallationReasonManual)
	require.NoError(t, err)

	// Verify that the mock expectations were met
	ctrl.Finish()
}

func writeMetadata(t *testing.T, metaDir string, metadata *Metadata) {
	t.Helper()

	require.NoError(t, os.MkdirAll(metaDir, 0o755))
	metaFile, err := os.Create(filepath.Join(metaDir, metadataFile))
	require.NoError(t, err)
	require.NoError(t, json.NewEncoder(metaFile).Encode(metadata))
	require.NoError(t, metaFile.Close())
}
