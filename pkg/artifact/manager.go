package artifact

import (
	"context"

	"github.com/cperrin88/gotya/pkg/http"
	"github.com/cperrin88/gotya/pkg/index"
)

type ManagerImpl struct {
	indexManager    index.Manager
	httpClient      http.Client
	os              string
	arch            string
	packageCacheDir string
}

func NewManager(indexManager index.Manager, httpClient http.Client, os, arch, packageCacheDir string) *ManagerImpl {
	return &ManagerImpl{
		indexManager:    indexManager,
		httpClient:      httpClient,
		os:              os,
		arch:            arch,
		packageCacheDir: packageCacheDir,
	}
}

func (m ManagerImpl) InstallArtifact(ctx context.Context, pkgName, version string, force bool) error {
	pkg, err := m.indexManager.ResolveArtifact(pkgName, version, m.os, m.arch)
	if err != nil {
		return err
	}
	if err := m.httpClient.DownloadArtifact(ctx, pkg.GetURL(), ""); err != nil {
		return err
	}

	return nil
}
