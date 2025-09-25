package artifact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cperrin88/gotya/pkg/artifact/database"
	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/model"
	"github.com/mholt/archives"
)

type ManagerImpl struct {
	os                     string
	arch                   string
	artifactCacheDir       string
	artifactDataInstallDir string
	artifactMetaInstallDir string
	installedDBPath        string
}

func NewManager(os, arch, artifactCacheDir, artifactInstallDir, artifactMetaInstallDir, installedDBPath string) *ManagerImpl {
	return &ManagerImpl{
		os:                     os,
		arch:                   arch,
		artifactCacheDir:       artifactCacheDir,
		artifactDataInstallDir: artifactInstallDir,
		artifactMetaInstallDir: artifactMetaInstallDir,
		installedDBPath:        installedDBPath,
	}
}

// InstallArtifact installs (verifies/stages) an artifact from a local file path, replacing the previous network-based install.
func (m ManagerImpl) InstallArtifact(ctx context.Context, desc *model.IndexArtifactDescriptor, localPath string) (err error) {
	// Set up rollback in case of failure
	var installed bool
	defer func() {
		if err != nil && installed {
			// If we installed files but then failed, clean them up
			m.installRollback(desc.Name)
		}
	}()

	if err = m.verifyArtifactFile(ctx, desc, localPath); err != nil {
		return err
	}

	// Load or create the installed database
	db := database.NewInstalledDatabase()
	if err = db.LoadDatabase(m.installedDBPath); err != nil {
		return fmt.Errorf("failed to load installed database: %w", err)
	}

	// Check if the artifact is already installed
	if db.IsArtifactInstalled(desc.Name) {
		return fmt.Errorf("artifact %s is already installed", desc.Name)
	}

	extractDir, err := os.MkdirTemp("", fmt.Sprintf("gotya-extract-%s", desc.Name))
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(extractDir)

	if err = m.extractArtifact(ctx, localPath, extractDir); err != nil {
		return fmt.Errorf("failed to extract artifact: %w", err)
	}

	//TODO pre-install hooks

	// Install the artifact files
	if err = m.installArtifactFiles(desc.Name, extractDir); err != nil {
		return fmt.Errorf("failed to install artifact files: %w", err)
	}
	installed = true // Mark that we've installed files that might need cleanup

	// Add the installed artifact to the database
	err = m.addArtifactToDatabase(db, desc)
	if err != nil {
		return fmt.Errorf("failed to update artifact database: %w", err)
	}

	//TODO post-install hooks

	return nil
}

func (m ManagerImpl) UninstallArtifact(ctx context.Context, artifactName string, purge bool) error {
	// Input validation
	if artifactName == "" {
		return fmt.Errorf("artifact name cannot be empty")
	}

	// Load the installed database
	db := database.NewInstalledDatabase()
	if err := db.LoadDatabase(m.installedDBPath); err != nil {
		return fmt.Errorf("failed to load installed database: %w", err)
	}

	// Check if the artifact is installed
	if !db.IsArtifactInstalled(artifactName) {
		return fmt.Errorf("artifact %s is not installed", artifactName)
	}

	artifact := db.FindArtifact(artifactName)
	if artifact == nil {
		return fmt.Errorf("artifact %s not found in database", artifactName)
	}

	// Handle purge mode
	if purge {
		return m.uninstallWithPurge(ctx, db, artifact)
	}

	// Handle selective mode
	return m.uninstallSelectively(ctx, db, artifact)
}

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

// installArtifactFiles handles the actual file operations for installing an artifact
// Returns an error if the installation fails
func (m ManagerImpl) installArtifactFiles(artifactName, extractDir string) error {
	metaSrcDir := filepath.Join(extractDir, artifactMetaDir)
	dataSrcDir := filepath.Join(extractDir, artifactDataDir)

	// Check if metadata directory exists
	if _, err := os.Stat(metaSrcDir); os.IsNotExist(err) {
		return fmt.Errorf("metadata directory not found in artifact")
	}

	err := os.MkdirAll(m.artifactMetaInstallDir, 0o755)
	if err != nil {
		return err
	}

	// Install the metadata directory
	metaPath := m.getArtifactMetaInstallPath(artifactName)
	if err := os.Rename(metaSrcDir, metaPath); err != nil {
		return fmt.Errorf("failed to install metadata: %w", err)
	}

	// Only install data directory if it exists
	if _, err := os.Stat(dataSrcDir); err == nil {
		err := os.MkdirAll(m.artifactDataInstallDir, 0o755)
		if err != nil {
			return err
		}
		dataPath := m.getArtifactDataInstallPath(artifactName)
		if err := os.Rename(dataSrcDir, dataPath); err != nil {
			// Clean up the metadata directory if data installation fails
			_ = os.RemoveAll(metaPath)
			return fmt.Errorf("failed to install data: %w", err)
		}
	}

	return nil
}

// getAllFilesInDir recursively gets all files in a directory
func getAllFilesInDir(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return files, nil
}

func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// addArtifactToDatabase adds an installed artifact to the database
// Returns the list of installed files if successful, or an error
func (m ManagerImpl) addArtifactToDatabase(db *database.InstalledManagerImpl, desc *model.IndexArtifactDescriptor) error {
	metaPath := m.getArtifactMetaInstallPath(desc.Name)

	// Read and parse the metadata file
	metadataFilePath := filepath.Join(metaPath, metadataFile)
	metadata, err := ParseMetadataFromPath(metadataFilePath)
	if err != nil {
		return fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Process all files from the hashes map in a single loop
	var (
		metaFileEntries []database.InstalledFile
		dataFileEntries []database.InstalledFile
	)

	hash, err := calculateFileHash(metadataFilePath)
	if err != nil {
		return fmt.Errorf("failed to calculate hash: %w", err)
	}

	entry := database.InstalledFile{
		Path: metadataFile,
		Hash: hash,
	}
	metaFileEntries = append(metaFileEntries, entry)

	// Process all other files
	for relPath, hash := range metadata.Hashes {
		// Check if it's a data file (starts with artifactDataDir/)
		if strings.HasPrefix(relPath, artifactDataDir+"/") {
			// Remove the artifactDataDir/ prefix
			dataRelPath := strings.TrimPrefix(relPath, artifactDataDir+"/")
			entry := database.InstalledFile{
				Path: dataRelPath,
				Hash: hash,
			}
			dataFileEntries = append(dataFileEntries, entry)
		} else {
			metaRelPath := strings.TrimPrefix(relPath, artifactMetaDir+"/")
			// It's a metadata file
			entry := database.InstalledFile{
				Path: metaRelPath,
				Hash: hash,
			}
			metaFileEntries = append(metaFileEntries, entry)
		}
	}

	// Create and add the artifact to the database
	installedArtifact := &database.InstalledArtifact{
		Name:            desc.Name,
		Version:         desc.Version,
		Description:     desc.Description,
		InstalledAt:     time.Now(),
		InstalledFrom:   desc.URL,
		ArtifactMetaDir: metaPath,
		ArtifactDataDir: m.getArtifactDataInstallPath(desc.Name),
		MetaFiles:       metaFileEntries,
		DataFiles:       dataFileEntries,
	}
	db.AddArtifact(installedArtifact)

	// Save the database
	if err := db.SaveDatabase(m.installedDBPath); err != nil {
		return fmt.Errorf("failed to save installed database: %w", err)
	}

	return nil
}

// verifyArtifactFile verifies an artifact from a local file path.
// TODO rewrite to use a local filepath instead of archives.FileSystem.
func (m ManagerImpl) verifyArtifactFile(ctx context.Context, artifact *model.IndexArtifactDescriptor, filePath string) error {
	if _, err := os.Stat(filePath); err != nil {
		return errors.ErrArtifactNotFound
	}

	fsys, err := archives.FileSystem(ctx, filePath, nil)
	if err != nil {
		return err
	}

	metadataFile, err := fsys.Open(filepath.Join(artifactMetaDir, metadataFile))
	if err != nil {
		return err
	}
	defer metadataFile.Close()

	metadata := &Metadata{}
	if err := json.NewDecoder(metadataFile).Decode(metadata); err != nil {
		return err
	}

	if metadata.Name != artifact.Name || metadata.Version != artifact.Version || metadata.GetOS() != artifact.GetOS() || metadata.GetArch() != artifact.GetArch() {
		return errors.Wrapf(errors.ErrArtifactInvalid, "metadata mismatch - expected Name: %s, Version: %s, OS: %s, Arch: %s but got Name: %s, Version: %s, OS: %s, Arch: %s",
			artifact.Name, artifact.Version, artifact.GetOS(), artifact.GetArch(),
			metadata.Name, metadata.Version, metadata.GetOS(), metadata.GetArch())
	}

	dataDir, err := fsys.Open(artifactDataDir)
	if err != nil {
		return nil
	}

	defer dataDir.Close()

	if dir, ok := dataDir.(fs.ReadDirFile); ok {
		entries, err := dir.ReadDir(0)
		if err != nil {
			return errors.Wrap(err, "failed to read data directory")
		}

		for _, entry := range entries {
			if !entry.Type().IsRegular() {
				continue
			}
			artifactFile := filepath.Join(artifactDataDir, entry.Name())
			val, ok := metadata.Hashes[artifactFile]
			if !ok {
				return errors.Wrapf(errors.ErrArtifactInvalid, "hash for file %s not found", artifactFile)
			}

			h := sha256.New()

			file, err := fsys.Open(artifactFile)
			if err != nil {
				return errors.Wrap(err, "failed to open file")
			}

			if _, err := io.Copy(h, file); err != nil {
				return errors.Wrap(err, "failed to copy file")
			}

			if err := file.Close(); err != nil {
				return errors.Wrap(err, "failed to close file")
			}

			if fmt.Sprintf("%x", h.Sum(nil)) != val {
				return errors.Wrapf(errors.ErrArtifactInvalid, "Hashsum mismatch %x, %s", h.Sum(nil), val)
			}
		}
	}

	return nil
}

// extractArtifact extracts an artifact from a local file path to a directory.
func (m ManagerImpl) extractArtifact(ctx context.Context, filePath, destDir string) error {
	// Open the artifact file
	fsys, err := archives.FileSystem(ctx, filePath, nil)
	if err != nil {
		return errors.Wrap(err, "failed to open artifact file")
	}

	// Ensure the destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return errors.Wrapf(err, "failed to create destination directory: %s", destDir)
	}

	// Walk through all files in the artifact and extract them
	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory
		if path == "." {
			return nil
		}

		targetPath := filepath.Join(destDir, path)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		// Handle regular files and symlinks
		info, err := d.Info()
		if err != nil {
			return errors.Wrapf(err, "failed to get file info for %s", path)
		}

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(filepath.Join(filePath, path))
			if err != nil {
				return errors.Wrapf(err, "failed to read symlink %s", path)
			}

			// Ensure the target directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return errors.Wrapf(err, "failed to create parent directory for symlink %s", path)
			}

			// Remove existing file/symlink if it exists
			_ = os.Remove(targetPath)

			return os.Symlink(linkTarget, targetPath)
		}

		// Handle regular files
		srcFile, err := fsys.Open(path)
		if err != nil {
			return errors.Wrapf(err, "failed to open source file %s", path)
		}
		defer srcFile.Close()

		// Ensure the target directory exists
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return errors.Wrapf(err, "failed to create parent directory for %s", path)
		}

		dstFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			return errors.Wrapf(err, "failed to create destination file %s", targetPath)
		}
		defer dstFile.Close()

		if _, err := io.Copy(dstFile, srcFile); err != nil {
			return errors.Wrapf(err, "failed to copy file %s", path)
		}

		// Preserve file permissions
		if err := os.Chmod(targetPath, info.Mode().Perm()); err != nil {
			return errors.Wrapf(err, "failed to set permissions for %s", targetPath)
		}

		// Preserve modification time if possible
		if err := os.Chtimes(targetPath, info.ModTime(), info.ModTime()); err != nil {
			return errors.Wrapf(err, "failed to set modification time for %s", targetPath)
		}

		return nil
	}

	// Start walking through the artifact files
	return fs.WalkDir(fsys, ".", walkFn)
}

func (m ManagerImpl) VerifyArtifact(ctx context.Context, artifact *model.IndexArtifactDescriptor) error {
	filePath := filepath.Join(m.artifactCacheDir, fmt.Sprintf("%s_%s_%s_%s.gotya", artifact.Name, artifact.Version, artifact.OS, artifact.Arch))
	return m.verifyArtifactFile(ctx, artifact, filePath)
}

func (m ManagerImpl) getArtifactMetaInstallPath(artifactName string) string {
	return filepath.Join(m.artifactMetaInstallDir, artifactName)
}

func (m ManagerImpl) getArtifactDataInstallPath(artifactName string) string {
	return filepath.Join(m.artifactDataInstallDir, artifactName)
}

// installRollback cleans up any partially installed files in case of an error
func (m ManagerImpl) installRollback(artifactName string) {
	metaPath := m.getArtifactMetaInstallPath(artifactName)
	_ = os.RemoveAll(metaPath)

	dataPath := m.getArtifactDataInstallPath(artifactName)
	_ = os.RemoveAll(dataPath)
}
