package pkg

import (
	"errors"
	"fmt"
)

// Common package errors.
var (
	// Package creation and validation errors
	ErrSourceDirEmpty       = errors.New("source directory path cannot be empty")
	ErrOutputDirEmpty       = errors.New("output directory path cannot be empty")
	ErrPackageNameEmpty     = errors.New("package name cannot be empty")
	ErrPackageVersionEmpty  = errors.New("package version cannot be empty")
	ErrTargetOSEmpty        = errors.New("target OS cannot be empty")
	ErrTargetArchEmpty      = errors.New("target architecture cannot be empty")
	ErrNotADirectory        = errors.New("path is not a directory")
	ErrNoFilesFound         = errors.New("no files found to package")
	ErrInvalidPackageName   = errors.New("invalid package name: cannot contain path separators")
	ErrInvalidVersionString = errors.New("invalid version string: must contain only alphanumeric characters, dots, underscores, plus, and hyphens")
	ErrOutputFileExists     = errors.New("output file already exists")
	ErrPackageTooSmall      = errors.New("package file is too small to be valid")
	ErrDescriptionRequired  = errors.New("package description is required")

	// File operation errors
	ErrInvalidSourceDirectory = errors.New("invalid source directory")
	ErrDirectoryStatFailed    = errors.New("failed to get directory info")
	ErrDirectoryNotWritable   = errors.New("directory is not writable")

	// Metadata related errors
	ErrMetadataMissing      = errors.New("package is missing required metadata (pkg.json)")
	ErrMetadataFileNotFound = errors.New("package metadata file not found")
	ErrMetadataNotFound     = errors.New("metadata not found in package")
	ErrNameRequired         = errors.New("package name is required")
	ErrVersionRequired      = errors.New("package version is required")

	// Archive and extraction errors
	ErrUnsupportedArchiveFormat = errors.New("unsupported archive format (only .tar.gz and .tgz files are supported)")
	ErrInvalidFilePath          = errors.New("invalid file path in archive")
	ErrInvalidSymlinkTarget     = errors.New("invalid symlink target: points outside the target directory")
	ErrInvalidLinkTarget        = errors.New("invalid link target in archive")
	ErrUnsupportedFileType      = errors.New("unsupported file type in archive")
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

// wrapError is a helper function that wraps an error with additional context.
// If the error is nil, it returns nil.
func wrapError(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}
