package pkg

import "github.com/cperrin88/gotya/pkg/index"

func InstallPackage(repoManager *index.Manager) error {
	repoManager.FindPackage()
	return nil
}
