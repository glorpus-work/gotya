package pkg

import (
	"context"

	"github.com/cperrin88/gotya/pkg/config"
	"github.com/cperrin88/gotya/pkg/http"
	"github.com/cperrin88/gotya/pkg/index"
)

type ManagerImpl struct {
	indexManager index.Manager
	config       *config.Config
	httpClient   http.Client
}

func NewManager(indexManager index.Manager, config *config.Config) *ManagerImpl {
	return &ManagerImpl{
		indexManager: indexManager,
		config:       config,
		httpClient:   http.NewHTTPClient(config.Settings.HTTPTimeout),
	}
}

func (m ManagerImpl) InstallPackage(ctx context.Context, pkgName, version, os, arch string, force bool) error {
	pkg, err := m.indexManager.ResolvePackage(pkgName, version, os, arch)
	if err != nil {
		return err
	}
	if err := m.httpClient.DownloadPackage(ctx, pkg.GetUrl(), ""); err != nil {
		return err
	}

	return nil
}
