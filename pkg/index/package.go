package index

import (
	"net/url"

	"github.com/cperrin88/gotya/pkg/platform"
	"github.com/hashicorp/go-version"
)

type Package struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description"`
	URL          string            `json:"url"`
	Checksum     string            `json:"checksum"`
	Size         int64             `json:"size"`
	OS           string            `json:"os,omitempty"`
	Arch         string            `json:"arch,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

func (pkg *Package) MatchOs(os string) bool {
	return pkg.OS == "" || pkg.OS == os || pkg.OS == platform.AnyOS
}

func (pkg *Package) MatchArch(arch string) bool {
	return pkg.Arch == "" || pkg.Arch == arch || pkg.Arch == platform.AnyArch
}

func (pkg *Package) MatchVersion(versionConstraint string) bool {
	constraint, err := version.NewConstraint(versionConstraint)
	if err != nil {
		return false
	}
	v := pkg.GetVersion()
	if v == nil {
		return false
	}
	return constraint.Check(v)
}

func (pkg *Package) GetVersion() *version.Version {
	v, err := version.NewVersion(pkg.Version)
	if err != nil {
		return nil
	}
	return v
}

func (pkg *Package) GetOS() string {
	if pkg.OS == "" {
		return platform.AnyOS
	}
	return pkg.OS
}

func (pkg *Package) GetArch() string {
	if pkg.Arch == "" {
		return platform.AnyArch
	}
	return pkg.Arch
}

func (pkg *Package) GetURL() *url.URL {
	parse, err := url.Parse(pkg.URL)
	if err != nil {
		return nil
	}
	return parse
}
