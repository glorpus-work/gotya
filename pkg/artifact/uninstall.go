package artifact

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/cperrin88/gotya/pkg/artifact/database"
)

// uninstallWithPurge removes the entire artifact directories recursively
func (m ManagerImpl) uninstallWithPurge(ctx context.Context, db *database.InstalledManagerImpl, artifact *database.InstalledArtifact) error {
	// Remove meta directory
	if err := os.RemoveAll(artifact.ArtifactMetaDir); err != nil {
		return fmt.Errorf("failed to remove meta directory %s: %w", artifact.ArtifactMetaDir, err)
	}

	// Remove data directory
	if err := os.RemoveAll(artifact.ArtifactDataDir); err != nil {
		return fmt.Errorf("failed to remove data directory %s: %w", artifact.ArtifactDataDir, err)
	}

	// Remove from database
	db.RemoveArtifact(artifact.Name)
	if err := db.SaveDatabase(m.installedDBPath); err != nil {
		return fmt.Errorf("failed to save database after purge: %w", err)
	}

	return nil
}

// uninstallSelectively removes only the files listed in the database, tracking directories for cleanup
func (m ManagerImpl) uninstallSelectively(ctx context.Context, db *database.InstalledManagerImpl, artifact *database.InstalledArtifact) error {
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
	db.RemoveArtifact(artifact.Name)
	if err := db.SaveDatabase(m.installedDBPath); err != nil {
		return fmt.Errorf("failed to save database after selective uninstall: %w", err)
	}

	return nil
}

// deleteFile deletes a single file and tracks its parent directory for cleanup
func (m ManagerImpl) deleteFile(path string, dirsToCheck map[string]bool) error {
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", path)
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
