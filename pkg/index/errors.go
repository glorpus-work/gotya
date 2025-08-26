package index

import (
	"fmt"
)

// Common index errors.
var (
	// ErrPackageNotFound is returned when a pkg is not found in any index.
	ErrPackageNotFound = fmt.Errorf("pkg not found")
)
