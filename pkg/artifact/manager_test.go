package artifact

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cperrin88/gotya/pkg/artifact/database"
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
		OS:          DefaultArtifactOS,
		Arch:        DefaultArtifactArch,
		Maintainer:  "test@example.com",
		Description: "Test artifact for unit tests",
		Dependencies: []model.Dependency{
			{Name: "dep1"},
			{Name: "dep2"},
		},
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

	err := mgr.InstallArtifact(context.Background(), desc, "/non/existent/path.gotya", model.InstallationReasonManual)
	assert.Equal(t, errors.ErrArtifactNotFound, err)
}

func TestInstallArtifact_RegularPackage(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), dbPath)
	artifactName := "test-artifact"

	// Create a test artifact
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
	assert.Equal(t, artifactName, installedArtifact.Name, "artifact name in database doesn't match")
	assert.Equal(t, "1.0.0", installedArtifact.Version, "artifact version in database doesn't match")
	assert.Equal(t, "http://example.com/test.gotya", installedArtifact.InstalledFrom, "installed from URL doesn't match")
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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "artifact name cannot be empty")
}

// TestInstallArtifact_EmptyDescriptor tests installation with nil descriptor
func TestInstallArtifact_EmptyDescriptor(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), filepath.Join(tempDir, "installed.db"))

	err := mgr.InstallArtifact(context.Background(), nil, "/nonexistent/path.gotya", model.InstallationReasonManual)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "artifact descriptor cannot be nil")
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
	assert.Error(t, err)
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
	metaFiles := installedArtifact.MetaFiles
	if len(metaFiles) > 0 {
		testFile := filepath.Join(installedArtifact.ArtifactMetaDir, metaFiles[0].Path)
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
	desc := &model.IndexArtifactDescriptor{
		Name:    "nonexistent",
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/test.gotya",
	}

	err := mgr.UpdateArtifact(context.Background(), testArtifact, desc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not installed")
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
	assert.Error(t, err)
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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to verify new artifact")
}

// TestUpdateArtifact_EmptyArtifactName tests updating with empty artifact name
func TestUpdateArtifact_EmptyArtifactName(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), filepath.Join(tempDir, "installed.db"))

	// Create a descriptor with empty name to test validation
	desc := &model.IndexArtifactDescriptor{
		Name:    "", // Empty name
		Version: "2.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/v2.0.0.gotya",
	}

	err := mgr.UpdateArtifact(context.Background(), "/nonexistent/path.gotya", desc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "artifact name cannot be empty")
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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "new artifact path cannot be empty")
}

// TestUpdateArtifact_EmptyDescriptor tests updating with nil descriptor
func TestUpdateArtifact_EmptyDescriptor(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), filepath.Join(tempDir, "installed.db"))

	err := mgr.UpdateArtifact(context.Background(), "/path/to/artifact.gotya", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "new artifact descriptor cannot be nil")
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
	assert.Error(t, err)
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
	assert.Error(t, err)
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
	db := database.NewInstalledDatabase()
	err := db.LoadDatabase(dbPath)
	require.NoError(t, err, "failed to load installed database")
	return db
}

// setupTestDatabaseWithArtifacts creates a test database with the specified artifacts
func setupTestDatabaseWithArtifacts(t *testing.T, dbPath string, artifacts []*model.InstalledArtifact) {
	t.Helper()
	db := database.NewInstalledDatabase()
	for _, artifact := range artifacts {
		db.AddArtifact(artifact)
	}
	err := db.SaveDatabase(dbPath)
	require.NoError(t, err, "failed to save test database")
}

// createTestArtifact creates a basic test artifact for database testing
func createTestArtifact(name, version string, reverseDeps []string) *model.InstalledArtifact {
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
	dependency := createTestArtifact("dep1", "1.0.0", []string{})
	mainArtifact := createTestArtifact("main", "1.0.0", []string{"dep1"})

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
	core := createTestArtifact("core", "1.0.0", []string{})
	libA := createTestArtifact("libA", "1.0.0", []string{"core"})
	libB := createTestArtifact("libB", "1.0.0", []string{"core"})
	app := createTestArtifact("app", "1.0.0", []string{"libA", "libB"})
	tool := createTestArtifact("tool", "1.0.0", []string{"libA"})

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
	assert.Error(t, err)
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
	artifact := createTestArtifact("self", "1.0.0", []string{"self"})
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
	mainArtifact := createTestArtifact("main", "1.0.0", []string{})

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

// Helper function to test installation reason transitions and updates
// Parameters:
// - initialReason: The installation reason for the initial installation
// - updateToDifferentVersion: Whether to update to version 2.0.0 or keep 1.0.0
// - expectedFinalReason: The expected installation reason after the update
// - testName: Name suffix for the test artifact to avoid conflicts
func testInstallationReasonUpdate(t *testing.T, initialReason model.InstallationReason, updateToDifferentVersion bool, expectedFinalReason model.InstallationReason, testName string) {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), dbPath)
	artifactName := "test-artifact-" + testName

	// Create and install the initial version
	testArtifact := filepath.Join(tempDir, "test-artifact.gotya")
	metadata := &Metadata{
		Name:        artifactName,
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Test artifact for installation reason test",
	}
	setupTestArtifact(t, testArtifact, true, metadata)

	desc := &model.IndexArtifactDescriptor{
		Name:    artifactName,
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/test.gotya",
	}

	// Install with the specified reason
	err := mgr.InstallArtifact(context.Background(), desc, testArtifact, initialReason)
	require.NoError(t, err)

	// Verify it was installed with the correct reason
	db := loadInstalledDB(t, dbPath)
	installedArtifact := db.FindArtifact(artifactName)
	require.NotNil(t, installedArtifact)
	assert.Equal(t, initialReason, installedArtifact.InstallationReason, "should have correct initial reason")

	// Create the artifact for update
	var updateVersion string
	var updateURL string
	if updateToDifferentVersion {
		updateVersion = "2.0.0"
		updateURL = "http://example.com/test-v2.gotya"
	} else {
		updateVersion = "1.0.0" // Same version
		updateURL = "http://example.com/test.gotya"
	}

	testArtifactV2 := filepath.Join(tempDir, "test-artifact-v2.gotya")
	metadataV2 := &Metadata{
		Name:        artifactName,
		Version:     updateVersion,
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Updated test artifact",
	}
	setupTestArtifact(t, testArtifactV2, true, metadataV2)

	descV2 := &model.IndexArtifactDescriptor{
		Name:    artifactName,
		Version: updateVersion,
		OS:      "linux",
		Arch:    "amd64",
		URL:     updateURL,
	}

	// Update the artifact
	err = mgr.UpdateArtifact(context.Background(), testArtifactV2, descV2)
	require.NoError(t, err)

	// Verify the update was successful
	db = loadInstalledDB(t, dbPath)
	updatedArtifact := db.FindArtifact(artifactName)
	require.NotNil(t, updatedArtifact)
	assert.Equal(t, updateVersion, updatedArtifact.Version, "version should be updated")
	assert.Equal(t, expectedFinalReason, updatedArtifact.InstallationReason, "should have correct final reason")
}

// TestInstallArtifact_InstallationReason_Transitions tests installation reason transitions
func TestInstallArtifact_InstallationReason_Transitions(t *testing.T) {
	testInstallationReasonUpdate(t, model.InstallationReasonAutomatic, false, model.InstallationReasonManual, "transitions")
}

// TestInstallArtifact_InstallationReason_ManualUpdate tests updating a manually installed artifact to a new version
func TestInstallArtifact_InstallationReason_ManualUpdate(t *testing.T) {
	testInstallationReasonUpdate(t, model.InstallationReasonManual, true, model.InstallationReasonManual, "manual-update")
}

// TestInstallArtifact_InstallationReason_SameVersionUpgrade tests upgrading automatic to manual with same version
func TestInstallArtifact_InstallationReason_SameVersionUpgrade(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), dbPath)
	artifactName := "test-artifact"

	// Create and install an artifact with automatic reason first
	testArtifact := filepath.Join(tempDir, "test-artifact.gotya")
	metadata := &Metadata{
		Name:        artifactName,
		Version:     "1.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Maintainer:  "test@example.com",
		Description: "Test artifact for same version upgrade",
	}
	setupTestArtifact(t, testArtifact, true, metadata)

	desc := &model.IndexArtifactDescriptor{
		Name:    artifactName,
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/test.gotya",
	}

	// Install with automatic reason
	err := mgr.InstallArtifact(context.Background(), desc, testArtifact, model.InstallationReasonAutomatic)
	require.NoError(t, err)

	// Verify it was installed with automatic reason
	db := loadInstalledDB(t, dbPath)
	installedArtifact := db.FindArtifact(artifactName)
	require.NotNil(t, installedArtifact)
	assert.Equal(t, model.InstallationReasonAutomatic, installedArtifact.InstallationReason, "should have automatic reason")

	// Create a new artifact file (same version, same metadata) but install with manual reason
	testArtifactV2 := filepath.Join(tempDir, "test-artifact-v2.gotya")
	setupTestArtifact(t, testArtifactV2, true, metadata) // Same metadata

	// Try to update with manual reason (should succeed even with same version)
	err = mgr.UpdateArtifact(context.Background(), testArtifactV2, desc)
	require.NoError(t, err)

	// Verify it was upgraded to manual reason
	db = loadInstalledDB(t, dbPath)
	updatedArtifact := db.FindArtifact(artifactName)
	require.NotNil(t, updatedArtifact)
	assert.Equal(t, model.InstallationReasonManual, updatedArtifact.InstallationReason, "should be upgraded to manual reason")
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
	manualArtifact := createTestArtifact("manual", "1.0.0", []string{"dep1"})
	manualArtifact.InstallationReason = model.InstallationReasonManual

	automaticWithDeps := createTestArtifact("auto-with-deps", "1.0.0", []string{"dep1"})
	automaticWithDeps.InstallationReason = model.InstallationReasonAutomatic

	automaticNoDeps := createTestArtifact("auto-no-deps", "1.0.0", []string{})
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
	manualArtifact := createTestArtifact("manual", "1.0.0", []string{"dep1"})
	manualArtifact.InstallationReason = model.InstallationReasonManual

	automaticOrphaned1 := createTestArtifact("auto-orphan1", "1.0.0", []string{})
	automaticOrphaned1.InstallationReason = model.InstallationReasonAutomatic

	automaticOrphaned2 := createTestArtifact("auto-orphan2", "1.0.0", []string{})
	automaticOrphaned2.InstallationReason = model.InstallationReasonAutomatic

	automaticWithDeps := createTestArtifact("auto-with-deps", "1.0.0", []string{"dep1"})
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
	manualOrphaned := createTestArtifact("manual-orphan", "1.0.0", []string{})
	manualOrphaned.InstallationReason = model.InstallationReasonManual

	automaticNotOrphaned := createTestArtifact("auto-not-orphan", "1.0.0", []string{"dep1"})
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
	missingAutomatic := createTestArtifact("missing-auto", "1.0.0", []string{})
	missingAutomatic.InstallationReason = model.InstallationReasonAutomatic
	missingAutomatic.Status = model.StatusMissing

	installedAutomatic := createTestArtifact("installed-auto", "1.0.0", []string{})
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
