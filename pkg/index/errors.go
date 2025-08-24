package index

import (
	"fmt"

	"github.com/cperrin88/gotya/pkg/errors"
)

// Common index errors.
var (
	// ErrRepositoryNotFound is returned when a index is not found.
	ErrRepositoryNotFound = fmt.Errorf("index not found")

	// ErrRepositoryDisabled is returned when trying to access a disabled index.
	ErrRepositoryDisabled = fmt.Errorf("index is disabled")

	// ErrRepositoryNameEmpty is returned when a index name is empty.
	ErrRepositoryNameEmpty = fmt.Errorf("index name cannot be empty")

	// ErrRepositoryURLMissing is returned when a index URL is empty.
	ErrRepositoryURLMissing = fmt.Errorf("index URL cannot be empty")

	// ErrRepositoryNotModified is returned when a index index hasn't been modified since last check.
	ErrRepositoryNotModified = fmt.Errorf("index index not modified")

	// ErrRepositoryIndexInvalid is returned when a index index is invalid.
	ErrRepositoryIndexInvalid = fmt.Errorf("invalid index index")

	// ErrRepositoryIndexMissing is returned when a index index is missing.
	ErrRepositoryIndexMissing = fmt.Errorf("index index not found")

	// ErrPackageNotFound is returned when a pkg is not found in any index.
	ErrPackageNotFound = fmt.Errorf("pkg not found")

	// ErrPackageNameEmpty is returned when a pkg name is empty.
	ErrPackageNameEmpty = fmt.Errorf("pkg name cannot be empty")

	// ErrNoRepositories is returned when no repositories are configured.
	ErrNoRepositories = fmt.Errorf("no repositories configured")
)

// Wrap wraps an error with additional context specific to the index pkg.
func Wrap(err error, msg string) error {
	return errors.Wrap(err, "index: "+msg)
}

// Wrapf wraps an error with additional formatted context specific to the index pkg.
func Wrapf(err error, format string, args ...interface{}) error {
	return errors.Wrapf(err, "index: "+format, args...)
}

// Errorf creates a new error with formatted message specific to the index pkg.
func Errorf(format string, args ...interface{}) error {
	return fmt.Errorf("index: "+format, args...)
}
