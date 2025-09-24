package artifact

import (
	"context"
	"time"

	"github.com/cperrin88/gotya/pkg/model"
)

type Manager interface {
	// InstallArtifact installs (verifies/stages) an artifact strictly from a local file.
	// The descriptor must describe the artifact and localPath must point to the local archive file.
	InstallArtifact(ctx context.Context, desc *model.IndexArtifactDescriptor, localPath string) error
}
