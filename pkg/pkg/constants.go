package pkg

import "github.com/cperrin88/gotya/pkg/fsutil"

// Re-export file permission constants from fsutil pkg for backward compatibility.
const (
	// File mode masks.
	FileModeMask = fsutil.FileModeMask
	DirModeMask  = fsutil.DirModeMask

	// Default file modes.
	FileModeDefault = fsutil.FileModeDefault
	FileModeSecure  = fsutil.FileModeSecure
	FileModeExec    = fsutil.FileModeExec

	// Default directory modes.
	DirModeDefault = fsutil.DirModeDefault
	DirModeSecure  = fsutil.DirModeSecure
	DirModePrivate = fsutil.DirModePrivate

	// Special file modes.
	Umask = fsutil.Umask
)

// File type constants.
const (
	// Package file names.
	MetadataFileName = "metadata.json"
	PackageFileName  = "pkg.json"

	// Directory names.
	FilesDirName   = "files"
	ScriptsDirName = "scripts"
	TempDirPrefix  = "gotya-*"
)

// Archive related constants.
const (
	// Archive file extensions.
	TarGzExt = ".tar.gz"
	TgzExt   = ".tgz"

	// Archive type indicators.
	TypeReg     = '0'    // Regular file
	TypeRegA    = '\x00' // Regular file (alternate)
	TypeLink    = '1'    // Hard link
	TypeSymlink = '2'    // Symbolic link
	TypeChar    = '3'    // Character device node
	TypeBlock   = '4'    // Block device node
	TypeDir     = '5'    // Directory
	TypeFifo    = '6'    // FIFO node
	TypeCont    = '7'    // Reserved
)

// Validation constants.
const (
	// Minimum pkg size in bytes (smallest possible gzip file is ~20 bytes).
	MinPackageSize = 50

	// Maximum file size for in-memory operations (10MB).
	MaxInMemoryFileSize = 10 << 20
)

// Default buffer sizes.
const (
	DefaultBufferSize = 32 * 1024 // 32KB buffer for file operations
	TarBlockSize      = 512       // Standard tar block size
)

// Common byte units for size calculations.
const (
	ByteUnit = 1024 // Base unit for byte size calculations (1KB)
)
