package index

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/cperrin88/gotya/pkg/artifact"
	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/model"
	"github.com/mholt/archives"
)

// Generator builds an index.json from a directory of .gotya artifact files.
// It inspects each artifact's embedded metadata and populates an Index.
// URLs in the produced index are recorded relative to the index file location.
// Optionally, a basePath can be prefixed to those relative URLs.
// Example: if basePath is "packages" and the relative path is "a/b.gotya",
// the URL becomes "packages/a/b.gotya".
//
// The generator does not attempt network access; it only reads local files.
// It also computes file size and sha256 checksum for each artifact file.
//
// Minimal public API intended to be used by CLI or other packages.

const (
	// CurrentFormatVersion is the current version of the index format.
	CurrentFormatVersion = "1"
)

type Generator struct {
	// Dir is the root directory containing artifact files (.gotya). It can
	// contain subdirectories; all .gotya files will be discovered recursively.
	Dir string
	// OutputPath is the full path of the index file to write (e.g., "/repo/index.json").
	OutputPath string
	// BasePath is an optional prefix to apply to artifact URLs in the index.
	// The resulting URL is path.Join(BasePath, relPathFromIndexDirToArtifact).
	BasePath string
}

// Generate scans Dir, builds an Index, and writes it to OutputPath.
func (g *Generator) Generate(ctx context.Context) error {
	if g.Dir == "" {
		return errors.Wrapf(errors.ErrInvalidPath, "packages directory is required")
	}
	if g.OutputPath == "" {
		return errors.Wrapf(errors.ErrInvalidPath, "output index path is required")
	}
	index := &Index{
		FormatVersion: CurrentFormatVersion,
		LastUpdate:    time.Now(),
		Artifacts:     make([]*model.IndexArtifactDescriptor, 0, InitialArtifactCapacity),
	}

	// Walk the directory tree and find .gotya files
	err := filepath.WalkDir(g.Dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".gotya") {
			return nil
		}
		// Parse this artifact and convert to descriptor
		desc, err := g.describeArtifact(ctx, p)
		if err != nil {
			return err
		}
		index.Artifacts = append(index.Artifacts, desc)
		return nil
	})
	if err != nil {
		return err
	}

	// Write index to file
	if err := os.MkdirAll(filepath.Dir(g.OutputPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(g.OutputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(index); err != nil {
		return err
	}
	return nil
}

func (g *Generator) describeArtifact(ctx context.Context, filePath string) (*model.IndexArtifactDescriptor, error) {
	// Open as archive filesystem and read meta/metadata.json
	fsys, err := archives.FileSystem(ctx, filePath, nil)
	if err != nil {
		return nil, err
	}
	metaFile, err := fsys.Open(filepath.Join("meta", "artifact.json"))
	if err != nil {
		return nil, err
	}
	defer metaFile.Close()

	md := &artifact.Metadata{}
	if err := json.NewDecoder(metaFile).Decode(md); err != nil {
		return nil, err
	}

	// Filesize and checksum of the artifact itself
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	checksum, err := sha256File(filePath)
	if err != nil {
		return nil, err
	}

	urlStr, err := g.makeURL(filePath)
	if err != nil {
		return nil, err
	}

	deps := make([]model.Dependency, 0, len(md.Dependencies))
	for _, name := range md.Dependencies {
		deps = append(deps, model.Dependency{Name: name})
	}

	desc := &model.IndexArtifactDescriptor{
		Name:         md.Name,
		Version:      md.Version,
		Description:  md.Description,
		URL:          urlStr,
		Checksum:     checksum,
		Size:         stat.Size(),
		OS:           md.GetOS(),
		Arch:         md.GetArch(),
		Dependencies: deps,
	}
	return desc, nil
}

func (g *Generator) makeURL(artifactPath string) (string, error) {
	// URL must be relative to the index file location
	indexDir := filepath.Dir(g.OutputPath)
	rel, err := filepath.Rel(indexDir, artifactPath)
	if err != nil {
		return "", err
	}
	// Normalize to URL path separators
	relURL := filepath.ToSlash(rel)
	if g.BasePath != "" {
		// Clean and join using path (URL style)
		relURL = path.Join(strings.TrimSuffix(g.BasePath, "/"), relURL)
	}
	// Validate it parses as a URL (relative URL is fine)
	if _, err := url.Parse(relURL); err != nil {
		return "", err
	}
	return relURL, nil
}

func sha256File(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
