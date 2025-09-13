package index

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/http"
	"github.com/cperrin88/gotya/pkg/repository"
)

type UintSlice []uint

func (x UintSlice) Len() int           { return len(x) }
func (x UintSlice) Less(i, j int) bool { return x[i] < x[j] }
func (x UintSlice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

type ManagerImpl struct {
	httpClient   http.Client
	repositories []*repository.Repository
	indexPath    string
	cacheTTL     time.Duration
	indexes      map[string]*Index
}

func NewManager(
	httpClient http.Client,
	repositories []*repository.Repository,
	indexPath string,
	cacheTTL time.Duration,
) *ManagerImpl {
	return &ManagerImpl{
		httpClient:   httpClient,
		repositories: repositories,
		indexPath:    indexPath,
		cacheTTL:     cacheTTL,
		indexes:      make(map[string]*Index, len(repositories)),
	}
}

func (rm *ManagerImpl) Sync(ctx context.Context, name string) error {
	repo, err := rm.getRepository(name)
	if err != nil {
		return errors.ErrRepositoryNotFound(name)
	}
	if err := rm.httpClient.DownloadIndex(ctx, repo.URL, rm.indexPath); err != nil {
		return err
	}
	return nil
}

func (rm *ManagerImpl) SyncAll(ctx context.Context) error {
	for _, repo := range rm.repositories {
		if err := rm.httpClient.DownloadIndex(ctx, repo.URL, rm.getIndexPath(repo.Name)); err != nil {
			return err
		}
	}
	return nil
}

func (rm *ManagerImpl) IsCacheStale(name string) bool {
	age, err := rm.GetCacheAge(name)
	if err != nil {
		return true
	}
	return age > rm.cacheTTL
}

func (rm *ManagerImpl) GetCacheAge(name string) (time.Duration, error) {
	repo, err := rm.getRepository(name)
	if err != nil {
		return -1, errors.ErrRepositoryNotFound(name)
	}

	indexPath := rm.getIndexPath(repo.Name)

	stat, err := os.Stat(indexPath)
	if err != nil {
		return -1, errors.Wrapf(err, "Cannot stat file %s", indexPath)
	}

	return time.Now().Sub(stat.ModTime()), nil
}

func (rm *ManagerImpl) FindPackages(name string) (map[string][]*Package, error) {
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

func (rm *ManagerImpl) ResolvePackage(name, version, os, arch string) (*Package, error) {
	repoPackages, err := rm.FindPackages(name)
	if err != nil {
		return nil, err
	}

	repoPrioPackages := make(map[uint][]*Package)

	for idxName, pkgs := range repoPackages {
		for _, pkg := range pkgs {
			if !pkg.MatchVersion(version) {
				continue
			}
			if !pkg.MatchOs(os) {
				continue
			}
			if !pkg.MatchArch(arch) {
				continue
			}

			repo, err := rm.getRepository(idxName)
			if err != nil {
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
	sort.Sort(sort.Reverse(UintSlice(prios)))

	var finalPackage *Package
	for _, prio := range prios {
		for _, pkg := range repoPrioPackages[prio] {
			if finalPackage == nil || pkg.GetVersion().GreaterThanOrEqual(finalPackage.GetVersion()) {
				finalPackage = pkg
			}
		}
	}

	return finalPackage, nil
}

func (rm *ManagerImpl) GetIndex(name string) (*Index, error) {
	index, err := ParseIndexFromFile(rm.indexPath)
	if err != nil {
		return nil, err
	}
	return index, nil
}

func (rm *ManagerImpl) getIndexes() (map[string]*Index, error) {
	if len(rm.indexes) == 0 {
		if err := rm.loadIndexes(); err != nil {
			return nil, err
		}
	}
	return rm.indexes, nil
}

func (rm *ManagerImpl) loadIndexes() error {
	for _, repo := range rm.repositories {
		index, err := ParseIndexFromFile(rm.getIndexPath(repo.Name))
		if err != nil {
			return err
		}
		rm.indexes[repo.Name] = index
	}
	return nil
}

func (rm *ManagerImpl) ListRepositories() []*repository.Repository {
	return rm.repositories
}

func (rm *ManagerImpl) getRepository(name string) (*repository.Repository, error) {
	idx := slices.IndexFunc(rm.repositories, func(r *repository.Repository) bool {
		return r.Name == name
	})
	if idx == -1 {
		return nil, errors.ErrRepositoryNotFound(name)
	}
	return rm.repositories[idx], nil
}

func (rm *ManagerImpl) getIndexPath(repoName string) string {
	return filepath.Join(rm.indexPath, fmt.Sprintf("%s.json", repoName))
}
