package pkg

import (
	"errors"
	"fmt"
)

// Common package errors.
var (
	// Validation errors.
	ErrSourceDirEmpty         = errors.New("source directory path cannot be empty")
	ErrOutputDirEmpty         = errors.New("output directory path cannot be empty")
	ErrPackageNameEmpty       = errors.New("package name cannot be empty")
	ErrPackageVersionEmpty    = errors.New("package version cannot be empty")
	ErrTargetOSEmpty          = errors.New("target OS cannot be empty")
	ErrTargetArchEmpty        = errors.New("target architecture cannot be empty")
	ErrSourceNotDir           = errors.New("source path is not a directory")
	ErrNoFilesFound           = errors.New("no files found to package")
	ErrInvalidPackageName     = errors.New("invalid package name: cannot contain path separators")
	ErrInvalidVersionString   = errors.New("invalid version string: must contain only alphanumeric characters, dots, underscores, plus, and hyphens")
	ErrOutputFileExists       = errors.New("output file already exists")
	ErrPackageTooSmall        = errors.New("package file is too small to be valid")
	ErrMetadataMissing        = errors.New("package is missing required metadata (pkg.json)")
	ErrMetadataFileNotFound   = errors.New("package metadata file not found")
	ErrNameRequired           = errors.New("package name is required")
	ErrVersionRequired        = errors.New("package version is required")
	ErrDescriptionRequired    = errors.New("package description is required")
	ErrDirectoryNotWritable   = errors.New("directory is not writable")
	ErrTarballWriteFailed     = errors.New("failed to write tarball")
	ErrInvalidSourceDirectory = errors.New("invalid source directory")
	ErrDirectoryStatFailed    = errors.New("failed to get directory info")
	ErrNotADirectory          = errors.New("path is not a directory")
)

// Error types for specific error conditions.
type (
	// HashCalculationError is returned when there's an error calculating a file hash.
	HashCalculationError struct {
		Path string
		Err  error
	}

	// PathTraversalError is returned when a path traversal attempt is detected.
	PathTraversalError struct {
		Path string
	}

	// PackageVerificationError is returned when package verification fails.
	PackageVerificationError struct {
		ExpectedName    string
		ExpectedVersion string
		ActualName      string
		ActualVersion   string
	}

	// FileOperationError is returned for file operation errors.
	FileOperationError struct {
		Path string
		Op   string
		Err  error
	}
)

// Error implements the error interface for HashCalculationError.
func (e *HashCalculationError) Error() string {
	return fmt.Sprintf("error calculating hash for %s: %v", e.Path, e.Err)
}

// Unwrap returns the underlying error for HashCalculationError.
func (e *HashCalculationError) Unwrap() error {
	return e.Err
}

// NewHashCalculationError creates a new HashCalculationError.
func NewHashCalculationError(path string, err error) error {
	return &HashCalculationError{
		Path: path,
		Err:  err,
	}
}

// Error implements the error interface for PathTraversalError.
func (e *PathTraversalError) Error() string {
	return fmt.Sprintf("path traversal attempt detected: %s", e.Path)
}

// Error implements the error interface for PackageVerificationError.
func (e *PackageVerificationError) Error() string {
	return fmt.Sprintf("package verification failed: expected %s-%s, got %s-%s",
		e.ExpectedName, e.ExpectedVersion, e.ActualName, e.ActualVersion)
}

// Error implements the error interface for FileOperationError.
func (e *FileOperationError) Error() string {
	return fmt.Sprintf("file operation failed: %s %s: %v", e.Op, e.Path, e.Err)
}

// Unwrap returns the underlying error for FileOperationError.
func (e *FileOperationError) Unwrap() error {
	return e.Err
}

// NewPathTraversalError creates a new PathTraversalError.
func NewPathTraversalError(path string) error {
	return &PathTraversalError{Path: path}
}

// NewPackageVerificationError creates a new PackageVerificationError.
func NewPackageVerificationError(expectedName, expectedVersion, actualName, actualVersion string) error {
	return &PackageVerificationError{
		ExpectedName:    expectedName,
		ExpectedVersion: expectedVersion,
		ActualName:      actualName,
		ActualVersion:   actualVersion,
	}
}

// NewFileOperationError creates a new FileOperationError.
func NewFileOperationError(op, path string, err error) error {
	return &FileOperationError{
		Op:   op,
		Path: path,
		Err:  err,
	}
}
