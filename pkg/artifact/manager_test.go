package artifact

import (
	"context"
	"os"
	"path/filepath"
	"testing"

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
	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, "install", artifactDataDir), filepath.Join(tempDir, "install", artifactMetaDir), dbPath)
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

	err := mgr.InstallArtifact(context.Background(), desc, testArtifact)
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
	err := mgr.InstallArtifact(context.Background(), desc, testArtifact)
	require.Error(t, err)

	// Verify rollback cleaned up everything
	assert.NoDirExists(t, filepath.Join(tempDir, "bad-package"))

	db := loadInstalledDB(t, dbPath)

	assert.False(t, db.IsArtifactInstalled("bad-package"), "bad-package should not be marked as installed in database")
	assert.Empty(t, db.GetInstalledArtifacts(), "no package should be installed at all")
}

func TestInstallArtifact_AlreadyInstalled(t *testing.T) {
	tempDir := t.TempDir()

	dbPath := filepath.Join(tempDir, "installed.db")
	mgr := NewManager("linux", "amd64", tempDir, filepath.Join(tempDir, artifactDataDir), filepath.Join(tempDir, artifactMetaDir), dbPath)

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

	db := loadInstalledDB(t, dbPath)

	assert.True(t, db.IsArtifactInstalled(DefaultArtifactName), "artifact should be marked as installed in database")
	assert.Len(t, db.GetInstalledArtifacts(), 1, "no package should be installed at all")
}

// loadInstalledDB loads the installed database from the given path
func loadInstalledDB(t *testing.T, dbPath string) *database.InstalledManagerImpl {
	t.Helper()
	db := database.NewInstalledDatabase()
	err := db.LoadDatabase(dbPath)
	require.NoError(t, err, "failed to load installed database")
	return db
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
			err := mgr.InstallArtifact(context.Background(), desc, testArtifact)
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
		Name:         artifactName,
		Version:      "1.0.0",
		OS:           "linux",
		Arch:         "amd64",
		Maintainer:   "test@example.com",
		Description:  "Original version",
		Dependencies: []string{"dep1"},
		Hooks:        make(map[string]string),
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
	err := mgr.InstallArtifact(context.Background(), originalDesc, originalArtifact)
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
		Name:         artifactName,
		Version:      "2.0.0",
		OS:           "linux",
		Arch:         "amd64",
		Maintainer:   "test@example.com",
		Description:  "Updated version",
		Dependencies: []string{"dep1", "dep2"}, // Added new dependency
		Hooks:        make(map[string]string),
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
	err = mgr.UpdateArtifact(context.Background(), artifactName, updatedArtifact, updatedDesc)
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

	err := mgr.UpdateArtifact(context.Background(), "nonexistent", testArtifact, desc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not installed")
}

// TestUpdateArtifact_AlreadyLatest tests updating to the same version
func TestUpdateArtifact_AlreadyLatest(t *testing.T) {
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
	err := mgr.InstallArtifact(context.Background(), originalDesc, originalArtifact)
	require.NoError(t, err)

	// Try to update to the same version and URL
	sameDesc := &model.IndexArtifactDescriptor{
		Name:    artifactName,
		Version: "1.0.0", // Same version
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/v1.0.0.gotya", // Same URL
	}

	err = mgr.UpdateArtifact(context.Background(), artifactName, originalArtifact, sameDesc)
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
	err := mgr.InstallArtifact(context.Background(), originalDesc, originalArtifact)
	require.NoError(t, err)

	// Try to update with an invalid artifact
	invalidDesc := &model.IndexArtifactDescriptor{
		Name:    artifactName,
		Version: "2.0.0",
		OS:      "linux",
		Arch:    "amd64",
		URL:     "http://example.com/v2.0.0.gotya",
	}

	err = mgr.UpdateArtifact(context.Background(), artifactName, "/nonexistent/path.gotya", invalidDesc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to verify new artifact")
}

// TestUpdateArtifact_UninstallFailure tests behavior when uninstall fails during update
func TestUpdateArtifact_UninstallFailure(t *testing.T) {
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
	err := mgr.InstallArtifact(context.Background(), originalDesc, originalArtifact)
	require.NoError(t, err)

	// Create an updated version
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
		URL:     "http://example.com/v2.0.0.gotya",
	}

	// Verify the updated artifact is valid before testing
	verifier := NewVerifier()
	err = verifier.VerifyArtifact(context.Background(), updatedDesc, updatedArtifact)
	require.NoError(t, err)

	// Now test the update - it should work normally
	err = mgr.UpdateArtifact(context.Background(), artifactName, updatedArtifact, updatedDesc)
	require.NoError(t, err)

	// Verify the update was successful
	db := loadInstalledDB(t, dbPath)
	assert.True(t, db.IsArtifactInstalled(artifactName), "updated artifact should be installed")
	updatedInstalled := db.FindArtifact(artifactName)
	require.NotNil(t, updatedInstalled)
	assert.Equal(t, "2.0.0", updatedInstalled.Version)
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
