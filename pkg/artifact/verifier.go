package artifact

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/glorpus-work/gotya/pkg/errutils"
	"github.com/glorpus-work/gotya/pkg/model"
	"github.com/mholt/archives"
)

// Verifier handles artifact verification operations
type Verifier struct{}

// NewVerifier creates a new Verifier instance
func NewVerifier() *Verifier {
	return &Verifier{}
}

// VerifyArtifact verifies an artifact from a local file path against the provided descriptor.
// If the descriptor is nil, only the internal consistency of the artifact is verified.
// This method extracts the artifact first and then verifies it.
func (v *Verifier) VerifyArtifact(ctx context.Context, artifact *model.IndexArtifactDescriptor, filePath string) error {
	if _, err := os.Stat(filePath); err != nil {
		return errutils.ErrArtifactNotFound
	}

	// Create a temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "gotya-verify-*")
	if err != nil {
		return errutils.Wrap(err, "failed to create temp directory")
	}
	defer func() { _ = os.RemoveAll(tempDir) }() // Clean up temp directory

	// Extract the archive to the temporary directory
	if err := v.extractArchive(ctx, filePath, tempDir); err != nil {
		return errutils.Wrap(err, "failed to extract archive")
	}

	// Verify the extracted artifact
	return v.VerifyArtifactFromPath(ctx, artifact, tempDir)
}

// VerifyArtifactFromPath verifies an artifact from a local directory path against the provided descriptor.
// This method works on already extracted artifacts and is useful when the artifact has already been extracted
// or when working with local directories. If the descriptor is nil, only the internal consistency is verified.
func (v *Verifier) VerifyArtifactFromPath(_ context.Context, artifact *model.IndexArtifactDescriptor, dirPath string) error {
	// Check if the directory exists
	if _, err := os.Stat(dirPath); err != nil {
		return errutils.ErrArtifactNotFound
	}

	// Open the metadata file from the extracted directory
	metadataPath := filepath.Join(dirPath, artifactMetaDir, metadataFile)
	metadataFile, err := os.Open(metadataPath)
	if err != nil {
		return errutils.Wrap(err, "failed to open metadata file")
	}
	defer func() { _ = metadataFile.Close() }()

	metadata := &Metadata{}
	if err := json.NewDecoder(metadataFile).Decode(metadata); err != nil {
		return errutils.Wrap(err, "failed to decode metadata")
	}

	// Only verify against descriptor if provided
	if artifact != nil {
		if metadata.Name != artifact.Name || metadata.Version != artifact.Version ||
			metadata.GetOS() != artifact.GetOS() || metadata.GetArch() != artifact.GetArch() {
			return errutils.Wrapf(errutils.ErrArtifactInvalid,
				"metadata mismatch - expected Name: %s, Version: %s, OS: %s, Arch: %s but got Name: %s, Version: %s, OS: %s, Arch: %s",
				artifact.Name, artifact.Version, artifact.GetOS(), artifact.GetArch(),
				metadata.Name, metadata.Version, metadata.GetOS(), metadata.GetArch())
		}
	}

	return v.verifyArtifactContentsFromPath(dirPath, metadata)
}

// extractArchive extracts an archive file to the specified destination directory
func (v *Verifier) extractArchive(ctx context.Context, archivePath, destDir string) error {
	// Open the archive file
	fsys, err := archives.FileSystem(ctx, archivePath, nil)
	if err != nil {
		return fmt.Errorf("failed to open archive file: %w", err)
	}
	// Ensure archive FS is closed after extraction
	if closer, ok := fsys.(io.Closer); ok {
		defer func() { _ = closer.Close() }()
	}

	// Ensure the destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Walk through all files in the archive and extract them
	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return v.extractEntryFromArchiveFS(fsys, path, destDir, d)
	}

	// Start walking through the archive files
	return fs.WalkDir(fsys, ".", walkFn)
}

// extractEntryFromArchiveFS processes a single entry from an archive FS and writes it to destDir.
func (v *Verifier) extractEntryFromArchiveFS(fsys fs.FS, path, destDir string, d fs.DirEntry) error {
	// Skip the root directory
	if path == "." {
		return nil
	}

	targetPath := filepath.Join(destDir, path)

	if d.IsDir() {
		return os.MkdirAll(targetPath, 0755)
	}

	info, err := d.Info()
	if err != nil {
		return fmt.Errorf("failed to get file info for %s: %w", path, err)
	}

	// Handle symlinks
	if info.Mode()&os.ModeSymlink != 0 {
		linkTarget, err := fsys.Open(path)
		if err != nil {
			return fmt.Errorf("failed to read symlink %s: %w", path, err)
		}
		defer func() { _ = linkTarget.Close() }()

		targetBytes, err := io.ReadAll(linkTarget)
		if err != nil {
			return fmt.Errorf("failed to read symlink target %s: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory for symlink %s: %w", path, err)
		}
		_ = os.Remove(targetPath)
		return os.Symlink(string(targetBytes), targetPath)
	}

	// Regular file
	srcFile, err := fsys.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", path, err)
	}
	defer func() { _ = srcFile.Close() }()

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory for %s: %w", path, err)
	}

	dstFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", targetPath, err)
	}
	defer func() { _ = dstFile.Close() }()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file %s: %w", path, err)
	}
	if err := os.Chmod(targetPath, info.Mode().Perm()); err != nil {
		return fmt.Errorf("failed to set permissions for %s: %w", targetPath, err)
	}
	if err := os.Chtimes(targetPath, info.ModTime(), info.ModTime()); err != nil {
		return fmt.Errorf("failed to set modification time for %s: %w", targetPath, err)
	}
	return nil
}

// verifyArtifactContentsFromPath verifies the internal consistency of an artifact's contents from a local directory path.
func (v *Verifier) verifyArtifactContentsFromPath(dirPath string, metadata *Metadata) error {
	dataDirPath := filepath.Join(dirPath, artifactDataDir)

	// Check if data directory exists
	if _, err := os.Stat(dataDirPath); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(dataDirPath)
	if err != nil {
		return errutils.Wrap(err, "failed to read data directory")
	}

	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		artifactFile := filepath.ToSlash(filepath.Join(artifactDataDir, entry.Name()))
		val, ok := metadata.Hashes[artifactFile]
		if !ok {
			return errutils.Wrapf(errutils.ErrArtifactInvalid, "hash for file %s not found", artifactFile)
		}

		h := sha256.New()

		filePath := filepath.Join(dataDirPath, entry.Name())
		file, err := os.Open(filePath)
		if err != nil {
			return errutils.Wrap(err, "failed to open file")
		}

		if _, err := io.Copy(h, file); err != nil {
			return errutils.Wrap(err, "failed to copy file")
		}

		if err := file.Close(); err != nil {
			return errutils.Wrap(err, "failed to close file")
		}

		if fmt.Sprintf("%x", h.Sum(nil)) != val {
			return errutils.Wrapf(errutils.ErrArtifactInvalid, "Hashsum mismatch %x, %s", h.Sum(nil), val)
		}
	}

	return nil
}
