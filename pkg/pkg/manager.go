package pkg

import (
	"context"

	"github.com/cperrin88/gotya/pkg/config"
	"github.com/cperrin88/gotya/pkg/http"
	"github.com/cperrin88/gotya/pkg/index"
)

type Manager struct {
	indexManager *index.Manager
	config       *config.Config
}

func New(indexManager *index.Manager, config *config.Config) *Manager {
	return &Manager{
		indexManager: indexManager,
		config:       config,
	}
}

func (m Manager) InstallPackage(ctx context.Context, pkgName, version, os, arch string, force bool) error {
	pkg, err := m.indexManager.ResolvePackage(pkgName, version, os, arch)
	if err != nil {
		return err
	}
	hc := http.NewHTTPClient(m.config.Settings.HTTPTimeout)
	hc.DownloadPackage(ctx, pkg.URL, m.config.GetIndexPath())
}
