package artifact

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glorpus-work/gotya/pkg/errutils"
	"github.com/glorpus-work/gotya/pkg/fsutil"
	"github.com/glorpus-work/gotya/pkg/model"
)

// installArtifactFiles handles the actual file operations for installing an artifact
// Returns an error if the installation fails
func (m *ManagerImpl) installArtifactFiles(artifactName, extractDir string) error {
	metaSrcDir := filepath.Join(extractDir, artifactMetaDir)
	dataSrcDir := filepath.Join(extractDir, artifactDataDir)

	// Check if metadata directory exists
	if _, err := os.Stat(metaSrcDir); os.IsNotExist(err) {
		return fmt.Errorf("metadata directory not found in artifact: %w", errutils.ErrFileNotFound)
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
func (m *ManagerImpl) addArtifactToDatabase(desc *model.IndexArtifactDescriptor, existingReverseDeps []string, reason model.InstallationReason) error {
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

	m.recordReverseDependencies(desc)

	m.installDB.AddArtifact(installedArtifact)

	// Save the database
	if err := m.installDB.SaveDatabase(); err != nil {
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
func (m *ManagerImpl) recordReverseDependencies(desc *model.IndexArtifactDescriptor) {
	for _, dep := range desc.Dependencies {
		artifact := m.installDB.FindArtifact(dep.Name)
		if artifact == nil {
			// Create a dummy entry for missing dependency
			artifact = &model.InstalledArtifact{
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
				ReverseDependencies: []string{},
				Status:              model.StatusMissing,
				Checksum:            "invalid",
				InstallationReason:  model.InstallationReasonAutomatic,
			}

			m.installDB.AddArtifact(artifact)
		}
		artifact.ReverseDependencies = append(artifact.ReverseDependencies, desc.Name)
	}
}

// installRollback cleans up any partially installed files in case of an error
func (m *ManagerImpl) installRollback(artifactName string) {
	metaPath := m.getArtifactMetaInstallPath(artifactName)
	_ = os.RemoveAll(metaPath)

	dataPath := m.getArtifactDataInstallPath(artifactName)
	_ = os.RemoveAll(dataPath)
}

// performInstallation contains the core installation logic
func (m *ManagerImpl) performInstallation(extractDir string, desc *model.IndexArtifactDescriptor, reason model.InstallationReason, existingReverseDeps []string) error {
	if err := m.installArtifactFiles(desc.Name, extractDir); err != nil {
		return fmt.Errorf("failed to install artifact files: %w", err)
	}

	// Add the installed artifact to the database
	err := m.addArtifactToDatabase(desc, existingReverseDeps, reason)
	if err != nil {
		return fmt.Errorf("failed to update artifact database: %w", err)
	}

	return nil
}
