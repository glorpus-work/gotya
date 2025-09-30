package index

import (
	"context"
	"fmt"
	slices2 "slices"
	"strings"

	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/model"
)

// multiResolver handles dependency resolution for multiple resolve requests.
type multiResolver struct {
	manager     *ManagerImpl
	requests    []model.ResolveRequest
	constraints map[string][]string                       // name -> constraints (AND)
	selected    map[string]*model.IndexArtifactDescriptor // name -> chosen descriptor
	deps        map[string][]string                       // name -> dep names
	visiting    map[string]struct{}                       // for cycle detection
	preferences map[string]versionPreference              // name -> version preferences
}

// versionPreference represents version preference settings for an artifact.
type versionPreference struct {
	oldVersion  string
	keepVersion bool
}

const defaultConstraint = ">= 0.0.0"

// Resolve computes resolved artifacts with dependency resolution for multiple requests.
// Rules:
// - Resolve transitive dependencies for all requests.
// - For each artifact name, select a single version that satisfies all accumulated constraints.
// - Pick the latest version (by semver) that satisfies constraints and platform filters across all indexes.
// - Honor KeepVersion preferences where possible, but hard constraints take precedence.
// - Error if a dependency cannot be found in any index, or if no version satisfies combined constraints.
func (rm *ManagerImpl) Resolve(ctx context.Context, requests []model.ResolveRequest) (model.ResolvedArtifacts, error) { //nolint:revive // ctx reserved for future
	_ = ctx // reserved for future use

	if len(requests) == 0 {
		return model.ResolvedArtifacts{}, fmt.Errorf("no resolve requests provided: %w", errors.ErrValidation)
	}

	// Normalize version constraints
	for i := range requests {
		if requests[i].VersionConstraint == "" {
			requests[i].VersionConstraint = defaultConstraint
		}
	}

	// Delegate to a small resolver helper for clarity.
	res := newMultiResolver(rm, requests)
	if err := res.resolveAll(); err != nil {
		return model.ResolvedArtifacts{}, err
	}

	order := res.topoOrder()
	artifacts := res.resolveArtifacts(order)
	return model.ResolvedArtifacts{Artifacts: artifacts}, nil
}

// --- Internal planning helpers ---

func newMultiResolver(mgr *ManagerImpl, requests []model.ResolveRequest) *multiResolver {
	// Build preferences map from requests
	preferences := make(map[string]versionPreference)
	for _, req := range requests {
		preferences[req.Name] = versionPreference{
			oldVersion:  req.OldVersion,
			keepVersion: req.KeepVersion,
		}
	}

	return &multiResolver{
		manager:     mgr,
		requests:    requests,
		constraints: make(map[string][]string),
		selected:    make(map[string]*model.IndexArtifactDescriptor),
		deps:        make(map[string][]string),
		visiting:    make(map[string]struct{}),
		preferences: preferences,
	}
}

func (r *multiResolver) addConstraint(name, c string) {
	if c == "" {
		c = defaultConstraint
	}
	r.constraints[name] = append(r.constraints[name], c)
}

func (r *multiResolver) combineConstraints(list []string) string {
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
		return defaultConstraint
	}
	// Hashicorp's constraint lib supports comma as AND
	return strings.Join(out, ", ")
}

func (r *multiResolver) resolveAll() error {
	// First pass: add all initial constraints from requests
	for _, req := range r.requests {
		r.addConstraint(req.Name, req.VersionConstraint)
	}

	// Second pass: resolve all requested packages
	for _, req := range r.requests {
		if err := r.resolveNode(req.Name); err != nil {
			return err
		}
	}

	return nil
}

func (r *multiResolver) resolveNode(name string) error {
	if _, ok := r.visiting[name]; ok {
		return fmt.Errorf("dependency cycle detected involving %s: %w", name, errors.ErrValidation)
	}
	r.visiting[name] = struct{}{}
	defer delete(r.visiting, name)

	constraint := r.combineConstraints(r.constraints[name])

	// Try to honor keep preference by pinning to OldVersion if possible.
	// If the pinned resolution fails, fall back to the general hard constraint.
	var desc *model.IndexArtifactDescriptor
	var err error
	if pref, hasPref := r.preferences[name]; hasPref && pref.keepVersion && pref.oldVersion != "" {
		pinned := constraint + ", = " + pref.oldVersion
		if d, e := r.manager.ResolveArtifact(name, pinned, r.getCommonOS(), r.getCommonArch()); e == nil {
			desc = d
		} else {
			// fall back to non-pinned constraint
			desc, err = r.manager.ResolveArtifact(name, constraint, r.getCommonOS(), r.getCommonArch())
			if err != nil {
				return err
			}
		}
	} else {
		// No keep preference, resolve with hard constraint
		desc, err = r.manager.ResolveArtifact(name, constraint, r.getCommonOS(), r.getCommonArch())
		if err != nil {
			return err
		}
	}

	// Check if we have a preference for this package (indicating it was already installed)
	if pref, hasPref := r.preferences[name]; hasPref && pref.oldVersion != "" {
		// If the selected version is different from the old version, it will be marked as update
		// This is just for tracking - we still use the best version that satisfies constraints
		_ = pref // Keep for potential future use
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

func (r *multiResolver) getCommonOS() string {
	osSet := make(map[string]bool)
	for _, req := range r.requests {
		if req.OS != "" {
			osSet[req.OS] = true
		}
	}
	if len(osSet) == 1 {
		for os := range osSet {
			return os
		}
	}
	// Default fallback - in real implementation, this should be handled better
	return "linux"
}

func (r *multiResolver) getCommonArch() string {
	archSet := make(map[string]bool)
	for _, req := range r.requests {
		if req.Arch != "" {
			archSet[req.Arch] = true
		}
	}
	if len(archSet) == 1 {
		for arch := range archSet {
			return arch
		}
	}
	// Default fallback
	return "amd64"
}

func (r *multiResolver) topoOrder() []string {
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

	// Start with all requested packages
	for _, req := range r.requests {
		dfs(req.Name)
	}

	// Add any remaining selected packages
	for name := range r.selected {
		if !slices2.Contains(order, name) {
			dfs(name)
		}
	}
	return order
}

func (r *multiResolver) resolveArtifacts(order []string) []model.ResolvedArtifact {
	steps := make([]model.ResolvedArtifact, 0, len(order))
	for _, name := range order {
		d := r.selected[name]
		if d == nil {
			continue
		}

		// Determine the action to take
		action := model.ResolvedActionInstall
		reason := "new artifact installation"

		// Check if this artifact has a preference (indicating it was already installed)
		if pref, hasPref := r.preferences[name]; hasPref && pref.oldVersion != "" {
			if pref.oldVersion == d.Version {
				action = model.ResolvedActionSkip
				reason = "already at the required version"
			} else {
				action = model.ResolvedActionUpdate
				reason = fmt.Sprintf("updating from %s to %s", pref.oldVersion, d.Version)
			}
		}

		steps = append(steps, model.ResolvedArtifact{
			Name:      d.Name,
			Version:   d.Version,
			OS:        d.GetOS(),
			Arch:      d.GetArch(),
			SourceURL: d.GetURL(),
			Checksum:  d.Checksum,
			Action:    action,
			Reason:    reason,
		})
	}
	return steps
}

// ToGraph returns a trivially resolved graph for the descriptor (placeholder for future deps).
func ToGraph(desc *model.IndexArtifactDescriptor) []*model.IndexArtifactDescriptor {
	return []*model.IndexArtifactDescriptor{desc}
}
