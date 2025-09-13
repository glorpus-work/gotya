# Artifact Management Design Document

## Overview

The `package.go` file implements core functionality for creating and managing software packages in the Gotya package manager. It provides tools for creating, verifying, and processing package files in a tarball format with metadata.

## Core Components

### 1. Data Structures

#### `Metadata`
Represents package metadata with the following fields:
- `Name`: Artifact name (required)
- `Version`: Artifact version (required)
- `Maintainer`: Artifact maintainer (optional)
- `Description`: Artifact description
- `Dependencies`: List of package dependencies
- `Files`: List of files included in the package
- `Hooks`: Map of hook names to script paths

#### `File`
Represents a file entry in the package metadata:
- `Path`: Relative file path
- `Size`: File size in bytes
- `Mode`: File permissions
- `Digest`: SHA256 checksum of the file

### 2. Core Functions

#### `CreateArtifact`
Main entry point for creating a new package. It:
1. Validates input parameters
2. Processes files in the source directory
3. Creates package metadata
4. Generates a gzipped tarball containing all files
5. Verifies the created package

### File Processing Functions

#### `validateArtifactStructure`
Validates the package directory structure:
- Verifies presence of required `files/` and `meta/` directories
- Ensures no `package.json` exists in the source directory
- Validates that `meta/` only contains allowed files
- Returns an error if the structure is invalid

#### `processArtifactFiles`
Processes all files in the `files/` directory:
- Recursively walks through the directory
- Calculates SHA256 hashes for all files
- Validates that no symlinks point outside the `files/` directory
- Returns a slice of `File` structs with metadata
- Returns an error if no files are found or if any validation fails

#### `processHookScripts`
Processes hook scripts in the `meta/` directory:
- Processes only `.tengo` scripts referenced in package metadata
- Validates that no other files exist in `meta/`
- Calculates hashes for all referenced hook scripts
- Returns a slice of `File` structs for the hook scripts
- Returns an error if any validation fails

#### `processFiles` (orchestrator)
Coordinates the file processing workflow:
1. Calls `validateArtifactStructure` to validate the directory structure
2. Calls `processArtifactFiles` to process the package files
3. Calls `processHookScripts` to process hook scripts
4. Combines all results into the package metadata
5. Returns any errors that occur during processing

#### `createTarball`
Creates a gzipped tarball from the source directory. It:
1. Creates a temporary directory structure
2. Copies all files to the temporary directory
3. Adds metadata files
4. Creates a gzipped tarball
5. Cleans up temporary files

#### `verifyArtifact`
Verifies the integrity of a package file by:
1. Checking file hashes
2. Verifying file permissions and sizes
3. Ensuring all expected files are present

### 3. Helper Functions

- `validatePath`: Validates that a path is absolute and exists
- `calculateFileHash`: Calculates SHA256 hash of a file
- `openArtifactFile`: Opens a package file for reading
- `createGzipReader`: Creates a gzip reader from a file
- `processArtifactContents`: Processes and verifies package contents
- `isRegularFile`: Checks if a tar header represents a regular file
- `processFile`: Processes and verifies a single file from the package

## Artifact Structure Requirements

The package structure is strictly defined as follows:

```
package-name/
├── files/                 # Required: Contains all package files
│   └── ...               # Artifact files in their desired directory structure
└── meta/                  # Required: Contains package metadata and hooks
    ├── package.json      # Required: Artifact metadata in JSON format
    └── *.tengo           # Optional: Hook scripts (e.g., post-install.tengo)
```

### Requirements:

1. **Top-Level Directories**
   - `files/` directory MUST exist and contain the package files
   - `meta/` directory MUST exist and contain package metadata
   - NO other top-level directories are allowed

2. **`files/` Directory**
   - MUST contain at least one file
   - MUST NOT contain symlinks pointing outside the `files/` directory
   - Directory structure is determined by the packager

3. **`meta/` Directory**
   - MUST contain `package.json` with package metadata
   - MAY contain hook scripts with `.tengo` extension
   - Hook scripts are referenced by their filename in `package.json`

## Error Handling

All functions return errors that implement the `error` interface. The package defines custom error types in the `errors` package for specific error conditions.

## Security Considerations

1. File permissions are strictly validated
2. All file paths are sanitized
3. File hashes are verified during package creation and installation
4. Only regular files are processed (no symlinks or special files)

## Dependencies

- `archive/tar`: For creating and reading tarballs
- `compress/gzip`: For compression/decompression
- `crypto/sha256`: For file hashing
- `path/filepath`: For cross-platform path handling
- `github.com/cperrin88/gotya/pkg/errors`: Custom error types
- `github.com/cperrin88/gotya/pkg/logger`: Logging functionality

## Testing

Tests are located in `package_test.go` and cover:
- Artifact creation with various metadata
- Error conditions
- File processing
- Artifact verification

## Future Improvements

1. Support for package signing
2. Delta updates
3. Parallel file processing for better performance
4. Support for more archive formats
5. Better handling of file attributes (e.g., xattrs, ACLs)
