package artifact

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/glorpus-work/gotya/pkg/artifact/database"
	"github.com/glorpus-work/gotya/pkg/errors"
	"github.com/glorpus-work/gotya/pkg/fsutil"
	"github.com/glorpus-work/gotya/pkg/model"
)

// cleanupReverseDependencies removes the artifact from all reverse dependency lists of dependent artifacts
func (m *ManagerImpl) cleanupReverseDependencies(db database.InstalledManager, artifact *model.InstalledArtifact) {
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
func (m *ManagerImpl) removeArtifactFromDatabase(db database.InstalledManager, artifact *model.InstalledArtifact) error {
	db.RemoveArtifact(artifact.Name)
	if err := db.SaveDatabase(); err != nil {
		return fmt.Errorf("failed to save database after removing artifact %s: %w", artifact.Name, err)
	}
	return nil
}

// uninstallWithPurge removes the entire artifact directories recursively
func (m *ManagerImpl) uninstallWithPurge(_ context.Context, db database.InstalledManager, artifact *model.InstalledArtifact) error {
	// Execute pre-uninstall hook before removing files

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

// executePreUninstallHook executes the pre-uninstall hook for the artifact
func (m *ManagerImpl) executePreUninstallHook(artifact *model.InstalledArtifact, metadata *Metadata) error {
	preUninstallContext := &HookContext{
		ArtifactName:    artifact.Name,
		ArtifactVersion: artifact.Version,
		Operation:       "uninstall",
		MetaDir:         artifact.ArtifactMetaDir,
		DataDir:         artifact.ArtifactDataDir,
	}

	preUninstallHookPath := m.resolveHookPath(artifact.ArtifactMetaDir, "pre-uninstall", metadata)
	if preUninstallHookPath != "" {
		if err := m.hookExecutor.ExecuteHook(preUninstallHookPath, preUninstallContext); err != nil {
			return fmt.Errorf("pre-uninstall hook failed: %w", err)
		}
	}

	return nil
}

// executePostUninstallHook executes the post-uninstall hook for the artifact
func (m *ManagerImpl) executePostUninstallHook(artifact *model.InstalledArtifact, preservedScriptDir string) error {
	postUninstallContext := &HookContext{
		ArtifactName:    artifact.Name,
		ArtifactVersion: artifact.Version,
		Operation:       "uninstall",
		WasMetaDir:      artifact.ArtifactMetaDir,
		WasDataDir:      artifact.ArtifactDataDir,
	}

	if err := m.hookExecutor.ExecuteHook(preservedScriptDir, postUninstallContext); err != nil {
		return errors.Wrap(err, "failed to execute post-uninstall hook")
	}
	return nil
}

// deleteArtifactFiles deletes all files associated with an artifact
func (m *ManagerImpl) deleteArtifactFiles(artifact *model.InstalledArtifact) map[string]bool {
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

	return dirsToCheck
}

// uninstallSelectively removes only the files listed in the database, tracking directories for cleanup
func (m *ManagerImpl) uninstallSelectively(_ context.Context, db database.InstalledManager, artifact *model.InstalledArtifact) error {

	// Clean up reverse dependencies from other artifacts
	m.cleanupReverseDependencies(db, artifact)

	// Delete artifact files
	m.deleteArtifactFiles(artifact)

	// Remove from database
	return m.removeArtifactFromDatabase(db, artifact)
}

// deleteFile deletes a single file and tracks its parent directory for cleanup
func (m *ManagerImpl) deleteFile(path string, dirsToCheck map[string]bool) error {
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
func (m *ManagerImpl) tryRemoveEmptyDirs(dirsToCheck map[string]bool) {
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
			}
		}
	}
}

// preservePostUninstallHookScript preserves only hook scripts defined in metadata
func (m *ManagerImpl) preservePostUninstallHookScript(metaDir string, metadata *Metadata) (string, error) {
	if metadata == nil || metadata.Hooks == nil {
		return "", nil // No hooks to preserve
	}

	val, exists := metadata.Hooks["post-uninstall"]
	if !exists {
		return "", nil // No hooks to preserve
	}

	preservedScriptDir, err := os.MkdirTemp("", fmt.Sprintf("gotya-hooks-%s", "artifact.Name"))
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory for hook scripts: %w", err)
	}

	err = fsutil.Copy(filepath.Join(metaDir, val), filepath.Join(preservedScriptDir, val))
	if err != nil {
		return "", err
	}

	return preservedScriptDir, nil
}
