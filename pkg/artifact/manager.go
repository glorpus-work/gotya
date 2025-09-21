package artifact

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/http"
	"github.com/cperrin88/gotya/pkg/index"
	"github.com/mholt/archives"
)

type ManagerImpl struct {
	indexManager       index.Manager
	httpClient         http.Client
	os                 string
	arch               string
	artifactCacheDir   string
	artifactInstallDir string
}

func NewManager(indexManager index.Manager, httpClient http.Client, os, arch, packageCacheDir string) *ManagerImpl {
	return &ManagerImpl{
		indexManager:     indexManager,
		httpClient:       httpClient,
		os:               os,
		arch:             arch,
		artifactCacheDir: packageCacheDir,
	}
}

func (m ManagerImpl) InstallArtifact(ctx context.Context, pkgName, version string, force bool) error {
	artifact, err := m.indexManager.ResolveArtifact(pkgName, version, m.os, m.arch)
	if err != nil {
		return err
	}
	if err := m.DownloadArtifact(ctx, artifact); err != nil {
		return err
	}

	if err := m.VerifyArtifact(ctx, artifact); err != nil {
		return err
	}

	return nil
}

func (m ManagerImpl) DownloadArtifact(ctx context.Context, artifact *index.Artifact) error {
	if err := m.httpClient.DownloadArtifact(ctx, artifact.GetURL(), m.getArtifactCacheFilePath(artifact)); err != nil {
		return err
	}

	return nil
}

func (m ManagerImpl) VerifyArtifact(ctx context.Context, artifact *index.Artifact) error {
	filePath := m.getArtifactCacheFilePath(artifact)
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
		return errors.ErrArtifactInvalid
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

			val, ok := metadata.Hashes[filepath.Join(artifactDataDir, entry.Name())]
			if !ok {
				return errors.ErrArtifactInvalid
			}

			h := sha256.New()

			file, err := fsys.Open(filepath.Join(artifactDataDir, entry.Name()))
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

func (m ManagerImpl) getArtifactCacheFilePath(artifact *index.Artifact) string {
	return filepath.Join(m.artifactCacheDir, fmt.Sprintf("%s_%s_%s_%s.gotya", artifact.Name, artifact.Version, artifact.OS, artifact.Arch))
}
