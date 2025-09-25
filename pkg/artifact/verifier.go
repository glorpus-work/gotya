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

	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/model"
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
// TODO rewrite to use a local filepath instead of archives.FileSystem.
func (v *Verifier) VerifyArtifact(ctx context.Context, artifact *model.IndexArtifactDescriptor, filePath string) error {
	if _, err := os.Stat(filePath); err != nil {
		return errors.ErrArtifactNotFound
	}

	fsys, err := archives.FileSystem(ctx, filePath, nil)
	if err != nil {
		return err
	}

	metadataFile, err := fsys.Open(filepath.ToSlash(filepath.Join(artifactMetaDir, metadataFile)))
	if err != nil {
		return err
	}
	defer metadataFile.Close()

	metadata := &Metadata{}
	if err := json.NewDecoder(metadataFile).Decode(metadata); err != nil {
		return err
	}

	// Only verify against descriptor if provided
	if artifact != nil {
		if metadata.Name != artifact.Name || metadata.Version != artifact.Version ||
			metadata.GetOS() != artifact.GetOS() || metadata.GetArch() != artifact.GetArch() {
			return errors.Wrapf(errors.ErrArtifactInvalid,
				"metadata mismatch - expected Name: %s, Version: %s, OS: %s, Arch: %s but got Name: %s, Version: %s, OS: %s, Arch: %s",
				artifact.Name, artifact.Version, artifact.GetOS(), artifact.GetArch(),
				metadata.Name, metadata.Version, metadata.GetOS(), metadata.GetArch())
		}
	}

	return v.verifyArtifactContents(fsys, metadata)
}

// VerifyArtifactFile verifies the internal consistency of an artifact file without comparing against a descriptor.
func (v *Verifier) VerifyArtifactFile(ctx context.Context, filePath string) error {
	return v.VerifyArtifact(ctx, nil, filePath)
}

// verifyArtifactContents verifies the internal consistency of an artifact's contents.
func (v *Verifier) verifyArtifactContents(fsys fs.FS, metadata *Metadata) error {

	dataDir, err := fsys.Open(artifactDataDir)
	if err != nil {
		return nil
	}

	defer dataDir.Close()

	if dir, ok := dataDir.(fs.ReadDirFile); ok {
		entries, err := dir.ReadDir(0)
		if err != nil {
			return errors.Wrap(err, "failed to read data directory")
		}

		for _, entry := range entries {
			if !entry.Type().IsRegular() {
				continue
			}
			artifactFile := filepath.ToSlash(filepath.Join(artifactDataDir, entry.Name()))
			val, ok := metadata.Hashes[artifactFile]
			if !ok {
				return errors.Wrapf(errors.ErrArtifactInvalid, "hash for file %s not found", artifactFile)
			}

			h := sha256.New()

			file, err := fsys.Open(artifactFile)
			if err != nil {
				return errors.Wrap(err, "failed to open file")
			}

			if _, err := io.Copy(h, file); err != nil {
				return errors.Wrap(err, "failed to copy file")
			}

			if err := file.Close(); err != nil {
				return errors.Wrap(err, "failed to close file")
			}

			if fmt.Sprintf("%x", h.Sum(nil)) != val {
				return errors.Wrapf(errors.ErrArtifactInvalid, "Hashsum mismatch %x, %s", h.Sum(nil), val)
			}
		}
	}

	return nil
}

// extractArtifact extracts an artifact from a local file path to a directory.
func (v *Verifier) extractArtifact(ctx context.Context, filePath, destDir string) error {
	// Open the artifact file
	fsys, err := archives.FileSystem(ctx, filePath, nil)
	if err != nil {
		return errors.Wrap(err, "failed to open artifact file")
	}

	// Ensure the destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return errors.Wrapf(err, "failed to create destination directory: %s", destDir)
	}

	// Walk through all files in the artifact and extract them
	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory
		if path == "." {
			return nil
		}

		targetPath := filepath.Join(destDir, path)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		// Handle regular files and symlinks
		info, err := d.Info()
		if err != nil {
			return errors.Wrapf(err, "failed to get file info for %s", path)
		}

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := fsys.Open(path)
			if err != nil {
				return errors.Wrapf(err, "failed to read symlink %s", path)
			}
			defer linkTarget.Close()

			// Read the symlink target
			targetBytes, err := io.ReadAll(linkTarget)
			if err != nil {
				return errors.Wrapf(err, "failed to read symlink target %s", path)
			}

			// Ensure the target directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return errors.Wrapf(err, "failed to create parent directory for symlink %s", path)
			}

			// Remove existing file/symlink if it exists
			_ = os.Remove(targetPath)

			return os.Symlink(string(targetBytes), targetPath)
		}

		// Handle regular files
		srcFile, err := fsys.Open(path)
		if err != nil {
			return errors.Wrapf(err, "failed to open source file %s", path)
		}
		defer srcFile.Close()

		// Ensure the target directory exists
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return errors.Wrapf(err, "failed to create parent directory for %s", path)
		}

		dstFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			return errors.Wrapf(err, "failed to create destination file %s", targetPath)
		}
		defer dstFile.Close()

		if _, err := io.Copy(dstFile, srcFile); err != nil {
			return errors.Wrapf(err, "failed to copy file %s", path)
		}

		// Preserve file permissions
		if err := os.Chmod(targetPath, info.Mode().Perm()); err != nil {
			return errors.Wrapf(err, "failed to set permissions for %s", targetPath)
		}

		// Preserve modification time if possible
		if err := os.Chtimes(targetPath, info.ModTime(), info.ModTime()); err != nil {
			return errors.Wrapf(err, "failed to set modification time for %s", targetPath)
		}

		return nil
	}

	// Start walking through the artifact files
	return fs.WalkDir(fsys, ".", walkFn)
}
