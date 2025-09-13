package hooks

import (
	"fmt"

	"github.com/cperrin88/gotya/pkg/errors"
)

// Common hooks errors.
var (
	// ErrHookTypeEmpty is returned when a hooks type is empty.
	ErrHookTypeEmpty = fmt.Errorf("hooks type cannot be empty")

	// ErrHookExecution is returned when there's an error executing a hooks.
	ErrHookExecution = fmt.Errorf("error executing hooks")

	// ErrHookScript is returned when there's an error in a hooks script.
	ErrHookScript = fmt.Errorf("hooks script error")

	// ErrHookLoad is returned when there's an error loading a hooks.
	ErrHookLoad = fmt.Errorf("failed to load hooks")
)

// ErrUnsupportedHookEvent is returned when an unsupported hooks event is used.
func ErrUnsupportedHookEvent(event string) error {
	return errors.Wrapf(ErrHookExecution, "unsupported hooks event: %s", event)
}
