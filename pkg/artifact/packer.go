package artifact

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/glorpus-work/gotya/pkg/archive"
	"github.com/glorpus-work/gotya/pkg/errors"
	"github.com/glorpus-work/gotya/pkg/fsutil"
	"github.com/glorpus-work/gotya/pkg/model"
)

// Packer creates .gotya artifacts from input directories.
type Packer struct {
	name         string
	version      string
	os           string
	arch         string
	maintainer   string
	description  string
	dependencies []model.Dependency
	hooks        map[string]string

	inputDir  string
	outputDir string
	tempDir   string
	metadata  *Metadata
}

var allowedTopLevelFiles = []string{
	artifactMetaDir,
	artifactDataDir,
}

// NewPacker creates a new Packer instance with the specified configuration.
func NewPacker(name, version, operatingSystem, arch, maintainer, description string, dependencies []model.Dependency, hooks map[string]string, inputDir, outputDir string) *Packer {
	return &Packer{
		name:         name,
		version:      version,
		os:           operatingSystem,
		arch:         arch,
		maintainer:   maintainer,
		description:  description,
		dependencies: dependencies,
		hooks:        hooks,
		inputDir:     inputDir,
		outputDir:    outputDir,
	}
}

// Pack creates a .gotya artifact from the configured input directory and returns the path to the created artifact.
func (p *Packer) Pack() (string, error) {
	dir, err := os.MkdirTemp("", "gotya-packer")
	if err != nil {
		return "", err
	}

	p.tempDir = dir

	defer func() { _ = os.RemoveAll(dir) }()

	if err := p.checkInput(); err != nil {
		return "", err
	}

	p.metadata = &Metadata{
		Name:         p.name,
		Version:      p.version,
		OS:           p.os,
		Arch:         p.arch,
		Maintainer:   p.maintainer,
		Description:  p.description,
		Dependencies: p.dependencies,
		Hooks:        p.hooks,
		Hashes:       make(map[string]string),
	}

	if err := p.copyInputDir(); err != nil {
		return "", err
	}

	if err := p.createMetadataFile(); err != nil {
		return "", err
	}

	archiveManager := archive.NewManager()
	if err := archiveManager.Create(context.Background(), p.tempDir, p.getOutputFile()); err != nil {
		return "", err
	}

	if err := p.verify(); err != nil {
		return "", err
	}

	return p.getOutputFile(), nil
}

func (p *Packer) verify() error {
	verifier := NewVerifier()
	desc := &model.IndexArtifactDescriptor{
		Name:    p.name,
		Version: p.version,
		OS:      p.os,
		Arch:    p.arch,
	}
	// Use VerifyArtifactFromPath since we already have the artifact unpacked in tempDir
	if err := verifier.VerifyArtifactFromPath(context.Background(), desc, p.tempDir); err != nil {
		return err
	}
	return nil
}

// checkInput checks if the input is valid
// It ensures that:
// - The input directory exists
// - No artifact.json exists in the input directory
// - No other files than meta and data directories exist in the input directory
// - Only hook scripts with the .tengo extension exist in the meta directory
// - All hook scripts in the meta directory are referenced
func (p *Packer) checkInput() error {
	if _, err := os.Stat(p.inputDir); err != nil {
		return errors.Wrapf(errors.ErrInvalidPath, "input directory %s does not exist", p.inputDir)
	}

	if _, err := os.Stat(filepath.Join(p.inputDir, artifactMetaDir, metadataFile)); err == nil {
		return errors.Wrapf(errors.ErrInvalidPath, "artifact.json already exists in input directory")
	}

	rootDir, err := os.ReadDir(p.inputDir)
	if err != nil {
		return err
	}
	for _, entry := range rootDir {
		if !slices.Contains(allowedTopLevelFiles, entry.Name()) {
			return errors.Wrapf(errors.ErrInvalidPath, "file %s is not allowed in input directory", entry.Name())
		}
	}

	if _, err := os.Stat(filepath.Join(p.inputDir, artifactMetaDir)); err == nil {
		metaDir, err := os.ReadDir(filepath.Join(p.inputDir, artifactMetaDir))
		if err != nil {
			return err
		}
		for _, entry := range metaDir {
			if !strings.HasSuffix(entry.Name(), ".tengo") {
				return errors.Wrapf(errors.ErrInvalidPath, "file %s is not allowed in meta directory", entry.Name())
			}
			if !slices.Contains(slices.Collect(maps.Values(p.hooks)), entry.Name()) {
				return errors.Wrapf(errors.ErrInvalidPath, "hook %s is not referenced", entry.Name())
			}
		}
	}

	return nil
}

// copyInputDir copies the input directory to the temporary directory
// It also checks if any symlinks are not relative to the input directory and calculates files hashes
func (p *Packer) copyInputDir() error {
	absInputDir, err := filepath.Abs(p.inputDir)
	if err != nil {
		return errors.Wrap(err, "error getting absolute path of input directory")
	}
	err = filepath.WalkDir(absInputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return errors.Wrapf(err, "error accessing path %s", path)
		}
		if path == p.inputDir {
			return nil
		}

		relPath, err := filepath.Rel(absInputDir, path)
		if err != nil {
			return errors.Wrapf(err, "error getting relative path of %s", path)
		}
		tempPath := filepath.Join(p.tempDir, relPath)
		switch d.Type() & os.ModeType {
		case os.ModeDir:
			return p.copyDirEntryDir(tempPath, path)
		case os.ModeSymlink:
			return p.copyDirEntrySymlink(absInputDir, path, tempPath)
		default:
			return p.copyDirEntryFile(path, relPath, tempPath)
		}
	})
	if err != nil {
		return err
	}
	return nil
}

func (p *Packer) copyDirEntryDir(tempPath, sourcePath string) error {
	if err := os.Mkdir(tempPath, fsutil.DirModeDefault); err != nil {
		return errors.Wrapf(err, "error creating directory %s", sourcePath)
	}
	return nil
}

func (p *Packer) copyDirEntrySymlink(absInputDir, sourcePath, tempPath string) error {
	target, err := os.Readlink(sourcePath)
	if err != nil {
		return errors.Wrapf(err, "error reading symlink %s", sourcePath)
	}
	if filepath.IsAbs(target) {
		return errors.Wrapf(errors.ErrInvalidPath, "symlink %s is absolute", sourcePath)
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return errors.Wrapf(err, "error getting absolute path of symlink %s", sourcePath)
	}
	if !strings.HasPrefix(absTarget, absInputDir) {
		return errors.Wrapf(errors.ErrInvalidPath, "symlink %s points outside the input directory", sourcePath)
	}
	if err := os.Symlink(target, tempPath); err != nil {
		return errors.Wrapf(err, "error creating symlink %s", sourcePath)
	}
	return nil
}

func (p *Packer) copyDirEntryFile(sourcePath, relPath, tempPath string) error {
	out, err := fsutil.CreateFilePerm(tempPath, fsutil.FileModeDefault)
	if err != nil {
		return errors.Wrapf(err, "error creating file %s", sourcePath)
	}

	in, err := os.Open(sourcePath)
	if err != nil {
		_ = out.Close()
		return errors.Wrapf(err, "error opening file %s", sourcePath)
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, in); err != nil {
		_ = in.Close()
		_ = out.Close()
		return errors.Wrapf(err, "error copying file %s", sourcePath)
	}

	// Normalize to forward slashes for archive-internal paths
	p.metadata.Hashes[filepath.ToSlash(relPath)] = fmt.Sprintf("%x", hash.Sum(nil))

	if _, err := in.Seek(0, 0); err != nil {
		_ = in.Close()
		_ = out.Close()
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = in.Close()
		_ = out.Close()
		return errors.Wrapf(err, "error copying file %s", sourcePath)
	}
	// Close both files to avoid handle leaks (important on Windows)
	if err := in.Close(); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return nil
}

// createMetadataFile creates the metadata file in the temporary directory
func (p *Packer) createMetadataFile() error {
	metaJSON, err := json.MarshalIndent(p.metadata, "", "  ")
	if err != nil {
		return errors.Wrap(err, "error marshaling metadata")
	}
	metaJSON = append(metaJSON, '\n')

	if err := os.MkdirAll(filepath.Join(p.tempDir, artifactMetaDir), fsutil.DirModeDefault); err != nil {
		return err
	}

	file, err := fsutil.CreateFilePerm(filepath.Join(p.tempDir, artifactMetaDir, metadataFile), fsutil.FileModeDefault)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	if _, err := file.Write(metaJSON); err != nil {
		return err
	}
	return nil
}

func (p *Packer) getOutputFile() string {
	return filepath.Join(p.outputDir, fmt.Sprintf("%s_%s_%s_%s.%s", p.name, p.version, p.os, p.arch, artifactSuffix))
}
