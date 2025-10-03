package database

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/glorpus-work/gotya/pkg/model"
)

func TestInstalledManager(t *testing.T) {
	t.Run("NewInstalledManger", func(t *testing.T) {
		db := NewInstalledManger()
		assert.NotNil(t, db)
		assert.Equal(t, "1", db.FormatVersion)
		assert.WithinDuration(t, time.Now(), db.LastUpdate, time.Second)
		assert.Empty(t, db.Artifacts)
	})

	t.Run("AddAndFindArtifact", func(t *testing.T) {
		db := NewInstalledManger()
		artifact := &model.InstalledArtifact{
			Name:    "test-artifact",
			Version: "1.0.0",
		}

		t.Run("AddArtifact", func(t *testing.T) {
			db.AddArtifact(artifact)
			assert.Len(t, db.Artifacts, 1)
			assert.Equal(t, artifact, db.Artifacts[0])
		})

		t.Run("FindArtifact", func(t *testing.T) {
			found := db.FindArtifact("test-artifact")
			require.NotNil(t, found)
			assert.Equal(t, "test-artifact", found.Name)
			assert.Equal(t, "1.0.0", found.Version)

			nilArtifact := db.FindArtifact("non-existent")
			assert.Nil(t, nilArtifact)
		})

		t.Run("IsArtifactInstalled", func(t *testing.T) {
			assert.True(t, db.IsArtifactInstalled("test-artifact"))
			assert.False(t, db.IsArtifactInstalled("non-existent"))
		})

		t.Run("GetInstalledArtifacts", func(t *testing.T) {
			artifacts := db.GetInstalledArtifacts()
			require.Len(t, artifacts, 1)
			assert.Equal(t, "test-artifact", artifacts[0].Name)
		})

		t.Run("RemoveArtifact", func(t *testing.T) {
			// Remove existing artifact
			removed := db.RemoveArtifact("test-artifact")
			assert.True(t, removed)
			assert.Empty(t, db.Artifacts)

			// Try to remove non-existent artifact
			removed = db.RemoveArtifact("non-existent")
			assert.False(t, removed)
		})
	})

	t.Run("LoadAndSaveDatabase", func(t *testing.T) {
		tempDir := t.TempDir()
		dbPath := filepath.Join(tempDir, "installed.json")

		db := NewInstalledManger()
		artifact := &model.InstalledArtifact{
			Name:    "test-save",
			Version: "2.0.0",
		}
		db.AddArtifact(artifact)

		t.Run("SaveDatabaseTo", func(t *testing.T) {
			err := db.SaveDatabaseTo(dbPath)
			require.NoError(t, err)
			_, err = os.Stat(dbPath)
			assert.False(t, os.IsNotExist(err), "database file should exist")
		})

		t.Run("LoadDatabaseFrom", func(t *testing.T) {
			newDB := NewInstalledManger()
			err := newDB.LoadDatabaseFrom(dbPath)
			require.NoError(t, err)

			artifacts := newDB.GetInstalledArtifacts()
			require.Len(t, artifacts, 1)
			assert.Equal(t, "test-save", artifacts[0].Name)
			assert.Equal(t, "2.0.0", artifacts[0].Version)
		})

		t.Run("LoadNonExistentDatabase", func(t *testing.T) {
			newDB := NewInstalledManger()
			nonExistentPath := filepath.Join(tempDir, "nonexistent.json")
			err := newDB.LoadDatabaseFrom(nonExistentPath)
			require.NoError(t, err)
			assert.Empty(t, newDB.Artifacts)
		})

		t.Run("LoadInvalidDatabase", func(t *testing.T) {
			err := os.WriteFile(dbPath, []byte("invalid json"), 0644)
			require.NoError(t, err)

			newDB := NewInstalledManger()
			err = newDB.LoadDatabaseFrom(dbPath)
			require.Error(t, err)
		})

		t.Run("SaveInvalidPath", func(t *testing.T) {
			invalidPath := "/nonexistent/path/installed.json"
			err := db.SaveDatabaseTo(invalidPath)
			require.Error(t, err)
		})
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		db := NewInstalledManger()
		const numGoroutines = 10
		done := make(chan bool, numGoroutines)

		// Start multiple goroutines to add artifacts
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				artifact := &model.InstalledArtifact{
					Name:    fmt.Sprintf("artifact-%d", id), // Use unique names
					Version: "1.0.0",
				}
				db.AddArtifact(artifact)
				done <- true
			}(i)
		}

		// Wait for all goroutines to finish
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// Check that all artifacts were added
		artifacts := db.GetInstalledArtifacts()
		assert.Len(t, artifacts, numGoroutines, "Expected %d artifacts, got %d", numGoroutines, len(artifacts))
	})
}

func TestInstalledManager_InstallationReason(t *testing.T) {
	t.Run("InstallationReasonPersistence", func(t *testing.T) {
		tempDir := t.TempDir()
		dbPath := filepath.Join(tempDir, "installed.json")

		// Create database with artifacts having different installation reasons
		db := NewInstalledManger()

		manualArtifact := &model.InstalledArtifact{
			Name:               "manual-artifact",
			Version:            "1.0.0",
			InstallationReason: model.InstallationReasonManual,
		}
		db.AddArtifact(manualArtifact)

		automaticArtifact := &model.InstalledArtifact{
			Name:               "automatic-artifact",
			Version:            "1.0.0",
			InstallationReason: model.InstallationReasonAutomatic,
		}
		db.AddArtifact(automaticArtifact)

		// Save database
		err := db.SaveDatabaseTo(dbPath)
		require.NoError(t, err)

		// Load database in new instance
		newDB := NewInstalledManger()
		err = newDB.LoadDatabaseFrom(dbPath)
		require.NoError(t, err)

		// Verify installation reasons are preserved
		manualFound := newDB.FindArtifact("manual-artifact")
		require.NotNil(t, manualFound)
		assert.Equal(t, model.InstallationReasonManual, manualFound.InstallationReason, "manual artifact should preserve installation reason")

		automaticFound := newDB.FindArtifact("automatic-artifact")
		require.NotNil(t, automaticFound)
		assert.Equal(t, model.InstallationReasonAutomatic, automaticFound.InstallationReason, "automatic artifact should preserve installation reason")
	})

	t.Run("DefaultInstallationReason", func(t *testing.T) {
		// Test that new artifacts get a default installation reason
		artifact := &model.InstalledArtifact{
			Name:    "new-artifact",
			Version: "1.0.0",
			// No InstallationReason set explicitly
		}

		db := NewInstalledManger()
		db.AddArtifact(artifact)

		found := db.FindArtifact("new-artifact")
		require.NotNil(t, found)
		// Default should be empty string (zero value)
		assert.Equal(t, model.InstallationReason(""), found.InstallationReason, "new artifact should have empty installation reason by default")
	})
}

func TestFilteredArtifacts(t *testing.T) {
	db := NewInstalledManger()

	// Add test artifacts with different names
	artifacts := []*model.InstalledArtifact{
		{Name: "package-a", Version: "1.0.0"},
		{Name: "package-b", Version: "2.0.0"},
		{Name: "other-package", Version: "1.5.0"},
		{Name: "another-app", Version: "3.0.0"},
	}

	for _, artifact := range artifacts {
		db.AddArtifact(artifact)
	}

	t.Run("FilterByExactName", func(t *testing.T) {
		filtered := db.FilteredArtifacts("package-a")
		assert.Len(t, filtered, 1)
		assert.Equal(t, "package-a", filtered[0].Name)
	})

	t.Run("FilterByPartialName", func(t *testing.T) {
		filtered := db.FilteredArtifacts("package")
		assert.Len(t, filtered, 3) // package-a, package-b, other-package
		expectedNames := []string{"package-a", "package-b", "other-package"}
		for i, artifact := range filtered {
			assert.Equal(t, expectedNames[i], artifact.Name)
		}
	})

	t.Run("FilterByNonExistentName", func(t *testing.T) {
		filtered := db.FilteredArtifacts("nonexistent")
		assert.Empty(t, filtered)
	})

	t.Run("FilterByEmptyString", func(t *testing.T) {
		filtered := db.FilteredArtifacts("")
		assert.Len(t, filtered, 4) // Should return all artifacts
	})
}

func TestSetInstallationReason(t *testing.T) {
	db := NewInstalledManger()

	artifact := &model.InstalledArtifact{
		Name:    "test-package",
		Version: "1.0.0",
	}
	db.AddArtifact(artifact)

	t.Run("SetValidInstallationReason", func(t *testing.T) {
		err := db.SetInstallationReason("test-package", model.InstallationReasonManual)
		require.NoError(t, err)

		found := db.FindArtifact("test-package")
		require.NotNil(t, found)
		assert.Equal(t, model.InstallationReasonManual, found.InstallationReason)
	})

	t.Run("SetInstallationReasonForNonExistentArtifact", func(t *testing.T) {
		err := db.SetInstallationReason("nonexistent", model.InstallationReasonAutomatic)
		assert.Error(t, err)
	})

	t.Run("SetInstallationReasonToEmpty", func(t *testing.T) {
		err := db.SetInstallationReason("test-package", model.InstallationReason(""))
		require.NoError(t, err)

		found := db.FindArtifact("test-package")
		require.NotNil(t, found)
		assert.Equal(t, model.InstallationReason(""), found.InstallationReason)
	})
}
