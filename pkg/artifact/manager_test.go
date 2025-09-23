package artifact

import (
	"context"
	"testing"

	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/model"
	"github.com/cperrin88/gotya/pkg/platform"
	"github.com/stretchr/testify/assert"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager(platform.OSLinux, platform.ArchAMD64, t.TempDir(), "")
	assert.NotNil(t, mgr)
}

func TestInstallArtifact_MissingLocalFile(t *testing.T) {
	mgr := NewManager("linux", "amd64", t.TempDir(), "")

	desc := &model.IndexArtifactDescriptor{
		Name:    "invalid-artifact",
		Version: "1.0.0",
		OS:      "linux",
		Arch:    "amd64",
	}

	err := mgr.InstallArtifact(context.Background(), desc, "/non/existent/path.gotya")
	assert.Equal(t, errors.ErrArtifactNotFound, err)
}
