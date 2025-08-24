package index

import (
	"github.com/cperrin88/gotya/pkg/platform"
	version "github.com/hashicorp/go-version"
)

func (pkg *Package) MatchOs(os string) bool {
	return pkg.OS == "" || pkg.OS == os || pkg.OS == platform.AnyOS
}

func (pkg *Package) MatchArch(arch string) bool {
	return pkg.Arch == "" || pkg.Arch == arch || pkg.Arch == platform.AnyArch
}

func (pkg *Package) MatchVersion(versionConstraint string) bool {
	constraint, errversion.NewConstraint(versionConstraint)
	return
}
