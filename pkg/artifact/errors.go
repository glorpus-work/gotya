package artifact

import (
	"fmt"
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

	// VerificationError is returned when artifact verification fails.
	VerificationError struct {
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

// Common artifact errors.
var (
	// Path and file related errors.
	ErrInvalidSourceDirectory = fmt.Errorf("invalid source directory")
	ErrDirectoryStatFailed    = fmt.Errorf("failed to get directory info")
	ErrDirectoryNotWritable   = fmt.Errorf("directory is not writable")

	// Artifact validation errors.
	ErrInvalidVersion        = fmt.Errorf("invalid version")
	ErrArtifactAlreadyExists = fmt.Errorf("artifact already exists")

	// I/O and processing errors.
	ErrReadFailed      = fmt.Errorf("read failed")
	ErrWriteFailed     = fmt.Errorf("write failed")
	ErrInvalidMetadata = fmt.Errorf("invalid artifact metadata")

	// Artifact installation errors.
	// ErrArtifactInvalid is imported from artifact/errors/errors.go.
	ErrSourceDirEmpty       = fmt.Errorf("source directory path cannot be empty")
	ErrOutputDirEmpty       = fmt.Errorf("output directory path cannot be empty")
	ErrArtifactNameEmpty    = fmt.Errorf("artifact name cannot be empty")
	ErrArtifactVersionEmpty = fmt.Errorf("artifact version cannot be empty")
	ErrNotADirectory        = fmt.Errorf("path is not a directory")
	ErrNoFilesFound         = fmt.Errorf("no files found to artifact")
	ErrOutputFileExists     = fmt.Errorf("output file already exists")
	ErrArtifactTooSmall     = fmt.Errorf("artifact file is too small to be valid")
	ErrDescriptionRequired  = fmt.Errorf("artifact description is required")

	// Metadata related errors.
	ErrMetadataMissing      = fmt.Errorf("artifact is missing required metadata (artifact.json)")
	ErrMetadataFileNotFound = fmt.Errorf("artifact metadata file not found")
	ErrMetadataNotFound     = fmt.Errorf("metadata not found in artifact")

	// Archive and extraction errors.
	ErrUnsupportedArchiveFormat = fmt.Errorf("unsupported archive format (only .tar.gz and .tgz files are supported)")
	ErrInvalidFilePath          = fmt.Errorf("invalid file path in archive")
	ErrInvalidSymlinkTarget     = fmt.Errorf("invalid symlink target: points outside the target directory")
	ErrInvalidLinkTarget        = fmt.Errorf("invalid link target in archive")
	ErrUnsupportedFileType      = fmt.Errorf("unsupported file type in archive")
)

// Artifact installation error functions.
var (
	// ErrArtifactAlreadyInstalled returns an error for when a artifact is already installed.
	//nolint:err113 // Function variable for creating contextual errors is appropriate here
	ErrArtifactAlreadyInstalled = func(pkgName string) error {
		return fmt.Errorf("artifact %s is already installed (use --force to reinstall)", pkgName)
	}
	// ErrArtifactNotInstalled returns an error for when a artifact is not installed.
	//nolint:err113 // Function variable for creating contextual errors is appropriate here
	ErrArtifactNotInstalled = func(pkgName string) error {
		return fmt.Errorf("artifact %s is not installed", pkgName)
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

// Error implements the error interface for VerificationError.
func (e *VerificationError) Error() string {
	return fmt.Sprintf("artifact verification failed: expected %s@%s, got %s@%s.",
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

// NewVerificationError creates a new VerificationError.
func NewVerificationError(expectedName, expectedVersion, actualName, actualVersion string) error {
	return &VerificationError{
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
