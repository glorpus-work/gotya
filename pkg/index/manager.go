package index

import (
	"context"
	"maps"
	"os"
	"slices"
	"sort"
	"time"

	"github.com/cperrin88/gotya/pkg/config"
	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/http"
)

type Manager struct {
	config  *config.Config
	indexes map[string]*Index
}

func NewRepositoryManager(config *config.Config) *Manager {
	return &Manager{
		config:  config,
		indexes: make(map[string]*Index, len(config.Repositories)),
	}
}

func (rm *Manager) Sync(ctx context.Context, name string) error {
	repo := rm.config.GetRepository(name)
	if repo == nil {
		return errors.ErrRepositoryNotFound(name)
	}
	hc := http.NewHTTPClient(rm.config.Settings.HTTPTimeout)
	if err := hc.DownloadIndex(ctx, repo.URL, rm.config.GetIndexPath(name)); err != nil {
		return err
	}
	return nil
}

func (rm *Manager) SyncAll(ctx context.Context, name string) error {
	hc := http.NewHTTPClient(rm.config.Settings.HTTPTimeout)
	for _, repo := range rm.config.Repositories {
		if err := hc.DownloadIndex(ctx, repo.URL, rm.config.GetIndexPath(name)); err != nil {
			return err
		}
	}
	return nil
}

func (rm *Manager) IsCacheStale(name string) bool {
	cacheTTL := rm.config.Settings.CacheTTL
	repo := rm.config.GetRepository(name)
	if repo == nil {
		return true
	}

	indexPath := rm.config.GetIndexPath(name)

	stat, err := os.Stat(indexPath)
	if err != nil {
		return true
	}

	return stat.ModTime().Add(cacheTTL).Before(time.Now())
}

func (rm *Manager) GetCacheAge(name string) (time.Duration, error) {
	repo := rm.config.GetRepository(name)
	if repo == nil {
		return -1, errors.ErrRepositoryNotFound(name)
	}

	indexPath := rm.config.GetIndexPath(name)

	stat, err := os.Stat(indexPath)
	if err != nil {
		return -1, errors.Wrapf(err, "Cannot stat file %s", indexPath)
	}

	return time.Now().Sub(stat.ModTime()), nil
}

func (rm *Manager) FindPackages(name string) (map[string][]*Package, error) {
	indexes, err := rm.getIndexes()
	if err != nil {
		return nil, err
	}

	packages := make(map[string][]*Package, 10)

	for idxName, idx := range indexes {
		pkg := idx.FindPackages(name)
		if pkg != nil {
			if packages[idxName] != nil {
				packages[idxName] = make([]*Package, 0, 5)
			}
			packages[idxName] = pkg
		}
	}

	if len(packages) == 0 {
		return nil, errors.ErrPackageNotFound
	}
	return packages, nil
}

func (rm *Manager) ResolvePackage(name, version, os, arch string) (*Package, error) {
	repoPackages, err := rm.FindPackages(name)
	if err != nil {
		return nil, err
	}

	repoPrioPackages := make(map[int][]*Package)

	for idxName, pkgs := range repoPackages {
		for _, pkg := range pkgs {
			if version != "" && !pkg.MatchVersion(version) {
				continue
			}
			if os != "" && !pkg.MatchOs(os) {
				continue
			}
			if arch != "" && !pkg.MatchArch(arch) {
				continue
			}

			repo := rm.config.GetRepository(idxName)
			if repo == nil {
				return nil, errors.ErrRepositoryNotFound(idxName)
			}
			if repoPrioPackages[repo.Priority] == nil {
				repoPrioPackages[repo.Priority] = make([]*Package, 5)
			}
			repoPrioPackages[repo.Priority] = append(repoPrioPackages[repo.Priority], pkg)
		}
	}
	if len(repoPrioPackages) == 0 {
		return nil, ErrPackageNotFound
	}

	prios := slices.Collect(maps.Keys(repoPrioPackages))
	sort.Sort(sort.Reverse(sort.IntSlice(prios)))

	var finalPackage *Package
	for prio := range prios {
		for _, pkg := range repoPrioPackages[prio] {
			if finalPackage == nil || pkg.GetVersion().GreaterThanOrEqual(finalPackage.GetVersion()) {
				finalPackage = pkg
			}
		}
	}

	return finalPackage, nil
}

func (rm *Manager) getIndexes() (map[string]*Index, error) {
	if rm.indexes == nil {
		if err := rm.loadIndexes(); err != nil {
			return nil, err
		}
	}
	return rm.indexes, nil
}

func (rm *Manager) loadIndexes() error {
	for _, repo := range rm.config.Repositories {
		index, err := ParseIndexFromFile(rm.config.GetIndexPath(repo.Name))
		if err != nil {
			return err
		}
		rm.indexes[repo.Name] = index
	}
	return nil
}

func (rm *Manager) GetIndex(name string) (*Index, error) {
	index, err := ParseIndexFromFile(rm.config.GetIndexPath(name))
	if err != nil {
		return nil, err
	}
	return index, nil
}
