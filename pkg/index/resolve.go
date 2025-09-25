package index

import (
	"context"
	"fmt"
	"net/url"
	slices2 "slices"
	"strings"

	"github.com/cperrin88/gotya/pkg/model"
)

// ResolveRequest describes what the user asked to resolve.
type ResolveRequest struct {
	Name    string
	Version string // semver constraint (e.g., ">= 0.0.0" for latest)
	OS      string // target os
	Arch    string // target arch
}

// ResolvedArtifact is one concrete installation action.
type ResolvedArtifact struct {
	ID        string
	Name      string
	Version   string
	OS        string
	Arch      string
	SourceURL *url.URL
	Checksum  string
}

// ResolvedArtifacts is an ordered list of steps. Topologically sorted if deps are present.
type ResolvedArtifacts struct {
	Artifacts []ResolvedArtifact
}

// Resolve computes resolved artifacts with dependency resolution.
// Rules:
// - Resolve transitive dependencies.
// - For each artifact name, select a single version that satisfies all accumulated constraints.
// - Pick the latest version (by semver) that satisfies constraints and platform filters across all indexes.
// - Error if a dependency cannot be found in any index, or if no version satisfies combined constraints.
func (rm *ManagerImpl) Resolve(ctx context.Context, req ResolveRequest) (ResolvedArtifacts, error) { //nolint:revive // ctx reserved for future
	_ = ctx // reserved for future use

	// Normalize version request
	if req.Version == "" {
		req.Version = ">= 0.0.0"
	}

	// Delegate to a small resolver helper for clarity.
	res := newResolver(rm, req)
	res.addConstraint(req.Name, req.Version)
	if err := res.resolveNode(req.Name); err != nil {
		return ResolvedArtifacts{}, err
	}

	order := res.topoOrder(req.Name)
	Artifacts := res.resolveArtifacts(order)
	return ResolvedArtifacts{Artifacts: Artifacts}, nil
}

// --- Internal planning helpers ---

type resolver struct {
	manager     *ManagerImpl
	installReq  ResolveRequest
	constraints map[string][]string                       // name -> constraints (AND)
	selected    map[string]*model.IndexArtifactDescriptor // name -> chosen descriptor
	deps        map[string][]string                       // name -> dep names
	visiting    map[string]struct{}                       // for cycle detection
}

func newResolver(mgr *ManagerImpl, request ResolveRequest) *resolver {
	return &resolver{
		manager:     mgr,
		installReq:  request,
		constraints: make(map[string][]string),
		selected:    make(map[string]*model.IndexArtifactDescriptor),
		deps:        make(map[string][]string),
		visiting:    make(map[string]struct{}),
	}
}

func (r *resolver) addConstraint(name, c string) {
	if c == "" {
		c = ">= 0.0.0"
	}
	r.constraints[name] = append(r.constraints[name], c)
}

func (r *resolver) combineConstraints(list []string) string {
	// deduplicate while preserving order
	out := make([]string, 0, len(list))
	seen := make(map[string]struct{}, len(list))
	for _, s := range list {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	if len(out) == 0 {
		return ">= 0.0.0"
	}
	// Hashicorp's constraint lib supports comma as AND
	return strings.Join(out, ", ")
}

func (r *resolver) resolveNode(name string) error {
	if _, ok := r.visiting[name]; ok {
		return fmt.Errorf("dependency cycle detected involving %s", name)
	}
	r.visiting[name] = struct{}{}
	defer delete(r.visiting, name)

	constraint := r.combineConstraints(r.constraints[name])
	desc, err := r.manager.ResolveArtifact(name, constraint, r.installReq.OS, r.installReq.Arch)
	if err != nil {
		return err
	}

	prev, had := r.selected[name]
	if !had || prev.Version != desc.Version || prev.GetOS() != desc.GetOS() || prev.GetArch() != desc.GetArch() {
		// record selection
		r.selected[name] = desc
		// refresh deps list
		r.deps[name] = nil
		for _, d := range desc.Dependencies {
			r.deps[name] = append(r.deps[name], d.Name)
			r.addConstraint(d.Name, d.VersionConstraint)
			if err := r.resolveNode(d.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *resolver) topoOrder(root string) []string {
	order := make([]string, 0, len(r.selected))
	seen := make(map[string]bool, len(r.selected))
	var dfs func(n string)
	dfs = func(n string) {
		if seen[n] {
			return
		}
		seen[n] = true
		for _, m := range r.deps[n] {
			dfs(m)
		}
		order = append(order, n)
	}
	dfs(root)
	for name := range r.selected {
		if !slices2.Contains(order, name) {
			dfs(name)
		}
	}
	return order
}

func (r *resolver) resolveArtifacts(order []string) []ResolvedArtifact {
	steps := make([]ResolvedArtifact, 0, len(order))
	for _, name := range order {
		d := r.selected[name]
		if d == nil {
			continue
		}
		steps = append(steps, ResolvedArtifact{
			ID:        d.Name + "@" + d.Version,
			Name:      d.Name,
			Version:   d.Version,
			OS:        d.GetOS(),
			Arch:      d.GetArch(),
			SourceURL: d.GetURL(),
			Checksum:  d.Checksum,
		})
	}
	return steps
}

// ToGraph returns a trivially resolved graph for the descriptor (placeholder for future deps).
func ToGraph(desc *model.IndexArtifactDescriptor) []*model.IndexArtifactDescriptor {
	return []*model.IndexArtifactDescriptor{desc}
}
