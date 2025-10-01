package artifact

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/glorpus-work/gotya/pkg/artifact/database"
	"github.com/glorpus-work/gotya/pkg/errors"
	"github.com/glorpus-work/gotya/pkg/model"
)

// cleanupReverseDependencies removes the artifact from all reverse dependency lists of dependent artifacts
func (m ManagerImpl) cleanupReverseDependencies(db *database.InstalledManagerImpl, artifact *model.InstalledArtifact) {
	for _, dependentName := range artifact.ReverseDependencies {
		if dependent := db.FindArtifact(dependentName); dependent != nil {
			// Remove this artifact from the dependent's reverse dependencies
			for i, revDep := range dependent.ReverseDependencies {
				if revDep == artifact.Name {
					dependent.ReverseDependencies = append(dependent.ReverseDependencies[:i], dependent.ReverseDependencies[i+1:]...)
					break
				}
			}
		}
	}
}

// removeArtifactFromDatabase removes an artifact from the database and saves the database
func (m ManagerImpl) removeArtifactFromDatabase(db *database.InstalledManagerImpl, artifact *model.InstalledArtifact) error {
	db.RemoveArtifact(artifact.Name)
	if err := db.SaveDatabase(m.installedDBPath); err != nil {
		return fmt.Errorf("failed to save database after removing artifact %s: %w", artifact.Name, err)
	}
	return nil
}

// uninstallWithPurge removes the entire artifact directories recursively
func (m ManagerImpl) uninstallWithPurge(_ context.Context, db *database.InstalledManagerImpl, artifact *model.InstalledArtifact) error {
	// Clean up reverse dependencies from other artifacts
	m.cleanupReverseDependencies(db, artifact)

	// Remove meta directory
	if err := os.RemoveAll(artifact.ArtifactMetaDir); err != nil {
		return fmt.Errorf("failed to remove meta directory %s: %w", artifact.ArtifactMetaDir, err)
	}

	// Remove data directory
	if err := os.RemoveAll(artifact.ArtifactDataDir); err != nil {
		return fmt.Errorf("failed to remove data directory %s: %w", artifact.ArtifactDataDir, err)
	}

	// Remove from database
	return m.removeArtifactFromDatabase(db, artifact)
}

// uninstallSelectively removes only the files listed in the database, tracking directories for cleanup
func (m ManagerImpl) uninstallSelectively(_ context.Context, db *database.InstalledManagerImpl, artifact *model.InstalledArtifact) error {
	// Clean up reverse dependencies from other artifacts
	m.cleanupReverseDependencies(db, artifact)

	dirsToCheck := make(map[string]bool)

	// Delete meta files
	for _, file := range artifact.MetaFiles {
		fullPath := filepath.Join(artifact.ArtifactMetaDir, file.Path)
		if err := m.deleteFile(fullPath, dirsToCheck); err != nil {
			log.Printf("Warning: failed to delete meta file %s: %v", fullPath, err)
		}
	}

	// Delete data files
	for _, file := range artifact.DataFiles {
		fullPath := filepath.Join(artifact.ArtifactDataDir, file.Path)
		if err := m.deleteFile(fullPath, dirsToCheck); err != nil {
			log.Printf("Warning: failed to delete data file %s: %v", fullPath, err)
		}
	}

	// Try to remove empty directories
	m.tryRemoveEmptyDirs(dirsToCheck)

	// Remove from database
	return m.removeArtifactFromDatabase(db, artifact)
}

// deleteFile deletes a single file and tracks its parent directory for cleanup
func (m ManagerImpl) deleteFile(path string, dirsToCheck map[string]bool) error {
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s: %w", path, errors.ErrFileNotFound)
		}
		return fmt.Errorf("failed to remove file %s: %w", path, err)
	}

	// Track the parent directory for cleanup
	parentDir := filepath.Dir(path)
	if parentDir != "." && parentDir != "/" {
		dirsToCheck[parentDir] = true
	}
	return nil
}

// tryRemoveEmptyDirs attempts to remove directories that might be empty after file deletion
func (m ManagerImpl) tryRemoveEmptyDirs(dirsToCheck map[string]bool) {
	// Process directories in a loop since removing a directory can make its parent empty
	processed := make(map[string]bool)

	for len(dirsToCheck) > 0 && len(processed) < len(dirsToCheck) {
		for dir := range dirsToCheck {
			if processed[dir] {
				continue
			}

			// Try to remove the directory
			if err := os.Remove(dir); err == nil {
				// If successful, check if parent directory should also be removed
				parent := filepath.Dir(dir)
				if parent != "." && parent != "/" && parent != dir {
					dirsToCheck[parent] = true
				}
				processed[dir] = true
				delete(dirsToCheck, dir)
				log.Printf("Info: removed empty directory %s", dir)
			} else {
				// Directory is not empty, mark as processed to avoid infinite loops
				processed[dir] = true
				log.Printf("Warning: could not remove directory %s (not empty or permission denied)", dir)
			}
		}
	}
}
