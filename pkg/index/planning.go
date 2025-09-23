package index

import (
	"context"
	"net/url"

	"github.com/cperrin88/gotya/pkg/model"
)

// InstallRequest describes what the user asked to install.
type InstallRequest struct {
	Name    string
	Version string // exact for now; ranges can be added later
	OS      string // target os
	Arch    string // target arch
}

// InstallStep is one concrete installation action.
type InstallStep struct {
	ID        string
	Name      string
	Version   string
	OS        string
	Arch      string
	SourceURL *url.URL
	Checksum  string
}

// InstallPlan is an ordered list of steps. Topologically sorted if deps are present.
type InstallPlan struct {
	Steps []InstallStep
}

// Plan computes a minimal plan for the given request.
// Initial implementation: resolve a single artifact and return a single step.
func (rm *ManagerImpl) Plan(ctx context.Context, req InstallRequest) (InstallPlan, error) { //nolint:revive // ctx reserved for future
	// Use existing single-artifact resolution.
	desc, err := rm.ResolveArtifact(req.Name, req.Version, req.OS, req.Arch)
	if err != nil {
		return InstallPlan{}, err
	}
	step := InstallStep{
		ID:        desc.Name + "@" + desc.Version,
		Name:      desc.Name,
		Version:   desc.Version,
		OS:        desc.GetOS(),
		Arch:      desc.GetArch(),
		SourceURL: desc.GetURL(),
		Checksum:  desc.Checksum,
	}
	return InstallPlan{Steps: []InstallStep{step}}, nil
}

// ToGraph returns a trivially-resolved graph for the descriptor (placeholder for future deps).
func ToGraph(desc *model.IndexArtifactDescriptor) []*model.IndexArtifactDescriptor {
	return []*model.IndexArtifactDescriptor{desc}
}
