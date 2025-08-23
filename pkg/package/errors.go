package pkg

import (
	"fmt"
)

// Common package errors.
var (
	// Path and file related errors.
	ErrInvalidSourceDirectory = fmt.Errorf("invalid source directory")
	ErrDirectoryStatFailed    = fmt.Errorf("failed to get directory info")
	ErrDirectoryNotWritable   = fmt.Errorf("directory is not writable")

	// Package validation errors.
	ErrInvalidVersion       = fmt.Errorf("invalid version")
	ErrPackageAlreadyExists = fmt.Errorf("package already exists")

	// I/O and processing errors.
	ErrReadFailed      = fmt.Errorf("read failed")
	ErrWriteFailed     = fmt.Errorf("write failed")
	ErrInvalidMetadata = fmt.Errorf("invalid package metadata")

	// Package installation errors.
	// ErrPackageInvalid is imported from pkg/errors/errors.go.
	ErrSourceDirEmpty      = fmt.Errorf("source directory path cannot be empty")
	ErrOutputDirEmpty      = fmt.Errorf("output directory path cannot be empty")
	ErrPackageNameEmpty    = fmt.Errorf("package name cannot be empty")
	ErrPackageVersionEmpty = fmt.Errorf("package version cannot be empty")
	ErrNotADirectory       = fmt.Errorf("path is not a directory")
	ErrNoFilesFound        = fmt.Errorf("no files found to package")
	ErrOutputFileExists    = fmt.Errorf("output file already exists")
	ErrPackageTooSmall     = fmt.Errorf("package file is too small to be valid")
	ErrDescriptionRequired = fmt.Errorf("package description is required")

	// Metadata related errors.
	ErrMetadataMissing      = fmt.Errorf("package is missing required metadata (pkg.json)")
	ErrMetadataFileNotFound = fmt.Errorf("package metadata file not found")
	ErrMetadataNotFound     = fmt.Errorf("metadata not found in package")

	// Archive and extraction errors.
	ErrUnsupportedArchiveFormat = fmt.Errorf("unsupported archive format (only .tar.gz and .tgz files are supported)")
	ErrInvalidFilePath          = fmt.Errorf("invalid file path in archive")
	ErrInvalidSymlinkTarget     = fmt.Errorf("invalid symlink target: points outside the target directory")
	ErrInvalidLinkTarget        = fmt.Errorf("invalid link target in archive")
	ErrUnsupportedFileType      = fmt.Errorf("unsupported file type in archive")
)

// Package installation error functions.
var (
	// ErrPackageAlreadyInstalled returns an error for when a package is already installed.
	ErrPackageAlreadyInstalled = func(pkgName string) error {
		return fmt.Errorf("package %s is already installed (use --force to reinstall)", pkgName)
	}
	// ErrPackageNotInstalled returns an error for when a package is not installed.
	ErrPackageNotInstalled = func(pkgName string) error {
		return fmt.Errorf("package %s is not installed", pkgName)
	}
)

// Error types for specific error conditions.
type (
	// HashCalculationError represents an error that occurs during hash calculation.
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
	return fmt.Sprintf("failed to calculate hash for %s: %v.", e.Path, e.Err)
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
	return fmt.Sprintf("path traversal attempt detected: %s.", e.Path)
}

// Error implements the error interface for PackageVerificationError.
func (e *PackageVerificationError) Error() string {
	return fmt.Sprintf("package verification failed: expected %s@%s, got %s@%s.",
		e.ExpectedName, e.ExpectedVersion, e.ActualName, e.ActualVersion)
}

// Error implements the error interface for FileOperationError.
func (e *FileOperationError) Error() string {
	return fmt.Sprintf("failed to %s %s: %v.", e.Op, e.Path, e.Err)
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
