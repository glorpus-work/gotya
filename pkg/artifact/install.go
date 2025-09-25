package artifact

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cperrin88/gotya/pkg/artifact/database"
	"github.com/cperrin88/gotya/pkg/model"
)

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
		Name:                desc.Name,
		Version:             desc.Version,
		Description:         desc.Description,
		InstalledAt:         time.Now(),
		InstalledFrom:       desc.URL,
		ArtifactMetaDir:     metaPath,
		ArtifactDataDir:     m.getArtifactDataInstallPath(desc.Name),
		MetaFiles:           metaFileEntries,
		DataFiles:           dataFileEntries,
		ReverseDependencies: make([]string, 0),
		Status:              database.StatusInstalled,
		Checksum:            "", // Assuming checksum is handled elsewhere or set later
	}

	// Record reverse dependencies for each dependency
	for _, dep := range desc.Dependencies {
		existingArtifact := db.FindArtifact(dep.Name)
		if existingArtifact != nil {
			// Add current artifact to existing dependency's reverse dependencies
			existingArtifact.ReverseDependencies = append(existingArtifact.ReverseDependencies, desc.Name)
		} else {
			// Create a dummy entry for missing dependency
			dummyArtifact := &database.InstalledArtifact{
				Name:                dep.Name,
				Version:             "invalid",
				Description:         "invalid",
				InstalledAt:         time.Time{},
				InstalledFrom:       "invalid",
				ArtifactMetaDir:     "invalid",
				ArtifactDataDir:     "invalid",
				MetaFiles:           make([]database.InstalledFile, 0),
				DataFiles:           make([]database.InstalledFile, 0),
				ReverseDependencies: []string{desc.Name},
				Status:              database.StatusMissing,
				Checksum:            "invalid",
			}
			db.AddArtifact(dummyArtifact)
		}
	}

	db.AddArtifact(installedArtifact)

	// Save the database
	if err := db.SaveDatabase(m.installedDBPath); err != nil {
		return fmt.Errorf("failed to save installed database: %w", err)
	}

	return nil
}
