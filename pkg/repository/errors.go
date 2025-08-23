package repository

import (
	"fmt"

	"github.com/cperrin88/gotya/pkg/errors"
)

// Common repository errors.
var (
	// ErrRepositoryNotFound is returned when a repository is not found.
	ErrRepositoryNotFound = fmt.Errorf("repository not found")

	// ErrRepositoryDisabled is returned when trying to access a disabled repository.
	ErrRepositoryDisabled = fmt.Errorf("repository is disabled")

	// ErrRepositoryNameEmpty is returned when a repository name is empty.
	ErrRepositoryNameEmpty = fmt.Errorf("repository name cannot be empty")

	// ErrRepositoryURLInvalid is returned when a repository URL is invalid.
	ErrRepositoryURLInvalid = fmt.Errorf("repository URL is invalid")

	// ErrRepositoryURLMissing is returned when a repository URL is empty.
	ErrRepositoryURLMissing = fmt.Errorf("repository URL cannot be empty")

	// ErrRepositoryNotModified is returned when a repository index hasn't been modified since last check.
	ErrRepositoryNotModified = fmt.Errorf("repository index not modified")

	// ErrRepositoryIndexInvalid is returned when a repository index is invalid.
	ErrRepositoryIndexInvalid = fmt.Errorf("invalid repository index")

	// ErrRepositoryIndexMissing is returned when a repository index is missing.
	ErrRepositoryIndexMissing = fmt.Errorf("repository index not found")

	// ErrPackageNotFound is returned when a package is not found in any repository.
	ErrPackageNotFound = fmt.Errorf("package not found")

	// ErrPackageNameEmpty is returned when a package name is empty.
	ErrPackageNameEmpty = fmt.Errorf("package name cannot be empty")

	// ErrNoRepositories is returned when no repositories are configured.
	ErrNoRepositories = fmt.Errorf("no repositories configured")
)

// Wrap wraps an error with additional context specific to the repository package.
func Wrap(err error, msg string) error {
	return errors.Wrap(err, "repository: "+msg)
}

// Wrapf wraps an error with additional formatted context specific to the repository package.
func Wrapf(err error, format string, args ...interface{}) error {
	return errors.Wrapf(err, "repository: "+format, args...)
}

// Errorf creates a new error with formatted message specific to the repository package.
func Errorf(format string, args ...interface{}) error {
	return fmt.Errorf("repository: "+format, args...)
}
