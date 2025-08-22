package hook

import "fmt"

// Common hook errors.
var (
	// ErrHookTypeEmpty is returned when a hook type is empty.
	ErrHookTypeEmpty = fmt.Errorf("hook type cannot be empty")

	// ErrHookExecution is returned when there's an error executing a hook.
	ErrHookExecution = fmt.Errorf("error executing hook")

	// ErrHookScript is returned when there's an error in a hook script.
	ErrHookScript = fmt.Errorf("hook script error")

	// ErrHookLoad is returned when there's an error loading a hook.
	ErrHookLoad = fmt.Errorf("failed to load hook")
)

// ErrUnsupportedHookEvent is returned when an unsupported hook event is used.
func ErrUnsupportedHookEvent(event string) error {
	return fmt.Errorf("unsupported hook event: %s", event)
}
