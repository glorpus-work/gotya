package index

import (
	"fmt"
)

// Common index errors.
var (
	// ErrArtifactNotFound is returned when a artifact is not found in any index.
	ErrArtifactNotFound = fmt.Errorf("artifact not found")
)
