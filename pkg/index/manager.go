package index

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"sort"

	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/model"
)

// UintSlice is a slice of uint values that implements sort.Interface for sorting by value.
type UintSlice []uint

// ManagerImpl provides artifact management functionality for repositories and indexes.
type ManagerImpl struct {
	repositories []*Repository
	indexPath    string
	indexes      map[string]*Index
}

func (x UintSlice) Len() int           { return len(x) }
func (x UintSlice) Less(i, j int) bool { return x[i] < x[j] }
func (x UintSlice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

// NewManager creates a new ManagerImpl instance with the given repositories and index path.
func NewManager(
	repositories []*Repository,
	indexPath string,
) *ManagerImpl {
	return &ManagerImpl{
		repositories: repositories,
		indexPath:    indexPath,
		indexes:      make(map[string]*Index, len(repositories)),
	}
}

// FuzzySearchArtifacts performs a fuzzy search for artifacts matching the given query across all repositories.
func (rm *ManagerImpl) FuzzySearchArtifacts(query string) (map[string][]*model.IndexArtifactDescriptor, error) {
	indexes, err := rm.getIndexes()
	if err != nil {
		return nil, err
	}

	packages := make(map[string][]*model.IndexArtifactDescriptor, 10)

	for idxName, idx := range indexes {
		matches := idx.FuzzySearchArtifacts(query)
		if len(matches) > 0 {
			packages[idxName] = matches
		}
	}

	return packages, nil
}

// FindArtifacts searches for artifacts with the exact name across all repositories.
func (rm *ManagerImpl) FindArtifacts(name string) (map[string][]*model.IndexArtifactDescriptor, error) {
	indexes, err := rm.getIndexes()
	if err != nil {
		return nil, err
	}

	packages := make(map[string][]*model.IndexArtifactDescriptor, 10)

	for idxName, idx := range indexes {
		pkg := idx.FindArtifacts(name)
		if pkg != nil {
			if packages[idxName] != nil {
				packages[idxName] = make([]*model.IndexArtifactDescriptor, 0, 5)
			}
			packages[idxName] = pkg
		}
	}

	if len(packages) == 0 {
		return nil, errors.ErrArtifactNotFound
	}
	return packages, nil
}

// ResolveArtifact finds the best matching artifact for the given name, version, OS, and architecture constraints.
func (rm *ManagerImpl) ResolveArtifact(name, version, os, arch string) (*model.IndexArtifactDescriptor, error) {
	repoArtifacts, err := rm.FindArtifacts(name)
	if err != nil {
		return nil, err
	}

	repoPrioArtifacts, err := rm.filterAndGroupByPriority(repoArtifacts, version, os, arch)
	if err != nil {
		return nil, err
	}
	if len(repoPrioArtifacts) == 0 {
		// Artifact exists but no version matches the constraints
		availableVersions := availableVersionsForPlatform(repoArtifacts, os, arch)
		if len(availableVersions) == 0 {
			return nil, fmt.Errorf("artifact %s not found for %s/%s in any repository: %w", name, os, arch, ErrArtifactNotFound)
		}
		return nil, fmt.Errorf("artifact %s not found with version constraint %s (available versions: %v, os: %s, arch: %s): %w", name, version, availableVersions, os, arch, ErrArtifactNotFound)
	}

	finalArtifact := selectBestByPriorityAndVersion(repoPrioArtifacts)
	if finalArtifact == nil {
		return nil, ErrArtifactNotFound
	}

	desc := &model.IndexArtifactDescriptor{
		Name:         finalArtifact.Name,
		Version:      finalArtifact.Version,
		Description:  finalArtifact.Description,
		URL:          finalArtifact.URL,
		Checksum:     finalArtifact.Checksum,
		Size:         finalArtifact.Size,
		OS:           finalArtifact.GetOS(),
		Arch:         finalArtifact.GetArch(),
		Dependencies: finalArtifact.Dependencies,
	}
	return desc, nil
}

// filterAndGroupByPriority filters artifacts by constraints and groups them by repository priority.
func (rm *ManagerImpl) filterAndGroupByPriority(repoArtifacts map[string][]*model.IndexArtifactDescriptor, version, os, arch string) (map[uint][]*model.IndexArtifactDescriptor, error) {
	repoPrioArtifacts := make(map[uint][]*model.IndexArtifactDescriptor)
	for idxName, pkgs := range repoArtifacts {
		for _, pkg := range pkgs {
			if !pkg.MatchVersion(version) || !pkg.MatchOs(os) || !pkg.MatchArch(arch) {
				continue
			}
			repo, err := rm.getRepository(idxName)
			if err != nil {
				return nil, errors.ErrRepositoryNotFoundWithName(idxName)
			}
			if repoPrioArtifacts[repo.Priority] == nil {
				repoPrioArtifacts[repo.Priority] = make([]*model.IndexArtifactDescriptor, 0, 5)
			}
			repoPrioArtifacts[repo.Priority] = append(repoPrioArtifacts[repo.Priority], pkg)
		}
	}
	return repoPrioArtifacts, nil
}

// availableVersionsForPlatform lists versions that match OS/arch regardless of version constraint.
func availableVersionsForPlatform(repoArtifacts map[string][]*model.IndexArtifactDescriptor, os, arch string) []string {
	versions := make([]string, 0)
	for _, pkgs := range repoArtifacts {
		for _, pkg := range pkgs {
			if pkg.MatchOs(os) && pkg.MatchArch(arch) {
				versions = append(versions, pkg.Version)
			}
		}
	}
	return versions
}

// selectBestByPriorityAndVersion selects the highest-priority, highest-version artifact.
func selectBestByPriorityAndVersion(repoPrioArtifacts map[uint][]*model.IndexArtifactDescriptor) *model.IndexArtifactDescriptor {
	prios := slices.Collect(maps.Keys(repoPrioArtifacts))
	sort.Sort(sort.Reverse(UintSlice(prios)))
	var finalArtifact *model.IndexArtifactDescriptor
	for _, prio := range prios {
		for _, pkg := range repoPrioArtifacts[prio] {
			if finalArtifact == nil || pkg.GetVersion().GreaterThanOrEqual(finalArtifact.GetVersion()) {
				finalArtifact = pkg
			}
		}
	}
	return finalArtifact
}

// GetIndex retrieves the index for a specific repository by name.
func (rm *ManagerImpl) GetIndex(name string) (*Index, error) {
	index, err := ParseIndexFromFile(rm.getIndexPath(name))
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

// Reload clears and reloads indexes from disk
func (rm *ManagerImpl) Reload() error {
	rm.indexes = make(map[string]*Index, len(rm.repositories))
	return rm.loadIndexes()
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

// ListRepositories returns a list of all configured repositories.
func (rm *ManagerImpl) ListRepositories() []*Repository {
	return rm.repositories
}

func (rm *ManagerImpl) getRepository(name string) (*Repository, error) {
	idx := slices.IndexFunc(rm.repositories, func(r *Repository) bool {
		return r.Name == name
	})
	if idx == -1 {
		return nil, errors.ErrRepositoryNotFoundWithName(name)
	}
	return rm.repositories[idx], nil
}

func (rm *ManagerImpl) getIndexPath(repoName string) string {
	return filepath.Join(rm.indexPath, fmt.Sprintf("%s.json", repoName))
}
