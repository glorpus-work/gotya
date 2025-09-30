package artifact

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cperrin88/gotya/pkg/artifact/database"
	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/fsutil"
	"github.com/cperrin88/gotya/pkg/model"
)

// installArtifactFiles handles the actual file operations for installing an artifact
// Returns an error if the installation fails
func (m ManagerImpl) installArtifactFiles(artifactName, extractDir string) error {
	metaSrcDir := filepath.Join(extractDir, artifactMetaDir)
	dataSrcDir := filepath.Join(extractDir, artifactDataDir)

	// Check if metadata directory exists
	if _, err := os.Stat(metaSrcDir); os.IsNotExist(err) {
		return fmt.Errorf("metadata directory not found in artifact: %w", errors.ErrFileNotFound)
	}

	err := os.MkdirAll(m.artifactMetaInstallDir, 0o755)
	if err != nil {
		return err
	}

	// Install the metadata directory
	metaPath := m.getArtifactMetaInstallPath(artifactName)
	if err := fsutil.Move(metaSrcDir, metaPath); err != nil {
		return fmt.Errorf("failed to install metadata: %w", err)
	}

	// Only install data directory if it exists
	if _, err := os.Stat(dataSrcDir); err == nil {
		err := os.MkdirAll(m.artifactDataInstallDir, 0o755)
		if err != nil {
			return err
		}
		dataPath := m.getArtifactDataInstallPath(artifactName)
		if err := fsutil.Move(dataSrcDir, dataPath); err != nil {
			// Clean up the metadata directory if data installation fails
			_ = os.RemoveAll(metaPath)
			return fmt.Errorf("failed to install data: %w", err)
		}
	}

	return nil
}

// addArtifactToDatabase adds an installed artifact to the database
// Returns the list of installed files if successful, or an error
func (m ManagerImpl) addArtifactToDatabase(db *database.InstalledManagerImpl, desc *model.IndexArtifactDescriptor, existingReverseDeps []string, reason model.InstallationReason) error {
	metaPath := m.getArtifactMetaInstallPath(desc.Name)

	// Read and parse the metadata file
	metadataFilePath := filepath.Join(metaPath, metadataFile)
	metadata, err := ParseMetadataFromPath(metadataFilePath)
	if err != nil {
		return fmt.Errorf("failed to parse metadata: %w", err)
	}

	metaFiles, dataFiles, err := buildInstalledFileEntries(metadata, metadataFilePath)
	if err != nil {
		return err
	}

	// Create and add the artifact to the database
	installedArtifact := &model.InstalledArtifact{
		Name:                desc.Name,
		Version:             desc.Version,
		Description:         desc.Description,
		OS:                  desc.OS,
		Arch:                desc.Arch,
		InstalledAt:         time.Now(),
		InstalledFrom:       desc.URL,
		ArtifactMetaDir:     metaPath,
		ArtifactDataDir:     m.getArtifactDataInstallPath(desc.Name),
		MetaFiles:           metaFiles,
		DataFiles:           dataFiles,
		ReverseDependencies: existingReverseDeps,
		Status:              model.StatusInstalled,
		Checksum:            desc.Checksum,
		InstallationReason:  reason,
	}

	recordReverseDependencies(db, desc)
	db.AddArtifact(installedArtifact)

	// Save the database
	if err := db.SaveDatabase(m.installedDBPath); err != nil {
		return fmt.Errorf("failed to save installed database: %w", err)
	}
	return nil
}

// buildInstalledFileEntries builds InstalledFile entries from metadata and metadata file path.
func buildInstalledFileEntries(metadata *Metadata, metadataFilePath string) ([]model.InstalledFile, []model.InstalledFile, error) {
	var metaFileEntries []model.InstalledFile
	var dataFileEntries []model.InstalledFile

	hash, err := calculateFileHash(metadataFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to calculate hash: %w", err)
	}
	metaFileEntries = append(metaFileEntries, model.InstalledFile{Path: metadataFile, Hash: hash})

	for relPath, h := range metadata.Hashes {
		if strings.HasPrefix(relPath, artifactDataDir+"/") {
			dataRelPath := strings.TrimPrefix(relPath, artifactDataDir+"/")
			dataFileEntries = append(dataFileEntries, model.InstalledFile{Path: dataRelPath, Hash: h})
		} else {
			metaRelPath := strings.TrimPrefix(relPath, artifactMetaDir+"/")
			metaFileEntries = append(metaFileEntries, model.InstalledFile{Path: metaRelPath, Hash: h})
		}
	}
	return metaFileEntries, dataFileEntries, nil
}

// recordReverseDependencies updates reverse dependency links (and dummy entries) in the DB.
func recordReverseDependencies(db *database.InstalledManagerImpl, desc *model.IndexArtifactDescriptor) {
	for _, dep := range desc.Dependencies {
		existingArtifact := db.FindArtifact(dep.Name)
		if existingArtifact != nil {
			if existingArtifact.Status == model.StatusMissing {
				existingArtifact.Status = model.StatusInstalled
			} else {
				existingArtifact.ReverseDependencies = append(existingArtifact.ReverseDependencies, desc.Name)
			}
			continue
		}
		// Create a dummy entry for missing dependency
		db.AddArtifact(&model.InstalledArtifact{
			Name:                dep.Name,
			Version:             "invalid",
			Description:         "invalid",
			OS:                  "",
			Arch:                "",
			InstalledAt:         time.Time{},
			InstalledFrom:       "invalid",
			ArtifactMetaDir:     "invalid",
			ArtifactDataDir:     "invalid",
			MetaFiles:           make([]model.InstalledFile, 0),
			DataFiles:           make([]model.InstalledFile, 0),
			ReverseDependencies: []string{desc.Name},
			Status:              model.StatusMissing,
			Checksum:            "invalid",
			InstallationReason:  model.InstallationReasonAutomatic,
		})
	}
}

// installRollback cleans up any partially installed files in case of an error
func (m ManagerImpl) installRollback(artifactName string) {
	metaPath := m.getArtifactMetaInstallPath(artifactName)
	_ = os.RemoveAll(metaPath)

	dataPath := m.getArtifactDataInstallPath(artifactName)
	_ = os.RemoveAll(dataPath)
}
