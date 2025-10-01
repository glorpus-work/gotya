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

	"github.com/glorpus-work/gotya/pkg/archive"
	"github.com/glorpus-work/gotya/pkg/artifact"
	"github.com/glorpus-work/gotya/pkg/errors"
	"github.com/glorpus-work/gotya/pkg/model"
)

// Generator creates package indexes by scanning directories for artifact files.
// It supports incremental index generation using baseline indexes and conflict detection.
type Generator struct {
	// Dir is the root directory containing artifact files (.gotya). It can
	// contain subdirectories; all .gotya files will be discovered recursively.
	Dir string
	// OutputPath is the full path of the index file to write (e.g., "/repo/index.json").
	OutputPath string
	// BasePath is an optional prefix to apply to artifact URLs in the index.
	// The resulting URL is path.Join(BasePath, relPathFromIndexDirToArtifact).
	BasePath string
	// ForceOverwrite controls whether to overwrite an existing output file.
	ForceOverwrite bool
	// BaselineIndexPath is the path to an existing index file to use as a baseline.
	// If provided, only new/changed artifacts will be included in the output.
	BaselineIndexPath string
}

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

// NewGenerator creates a new Generator with default values.
func NewGenerator(dir, outputPath string) *Generator {
	return &Generator{
		Dir:        dir,
		OutputPath: outputPath,
	}
}

// WithBaseline sets the path to a baseline index file that will be used to extend the new index.
// The baseline index will be loaded and its artifacts will be included in the new index,
// unless they conflict with newly discovered artifacts.
func (g *Generator) WithBaseline(baselinePath string) *Generator {
	g.BaselineIndexPath = baselinePath
	return g
}

// Validate checks if the generator is properly configured.
func (g *Generator) Validate() error {
	// Check source directory exists and is readable
	if g.Dir == "" {
		return errors.Wrapf(errors.ErrInvalidPath, "source directory is required")
	}
	if g.OutputPath == "" {
		return errors.Wrapf(errors.ErrInvalidPath, "output path is required")
	}

	// Check source directory exists and is accessible
	if fi, err := os.Stat(g.Dir); os.IsNotExist(err) {
		return errors.Wrapf(errors.ErrInvalidPath, "source directory does not exist: %s", g.Dir)
	} else if !fi.IsDir() {
		return errors.Wrapf(errors.ErrInvalidPath, "source is not a directory: %s", g.Dir)
	}

	// Check output file doesn't exist (unless force is set)
	if !g.ForceOverwrite {
		if _, err := os.Stat(g.OutputPath); err == nil {
			return errors.Wrapf(errors.ErrAlreadyExists,
				"output file exists (use ForceOverwrite to overwrite): %s", g.OutputPath)
		}
	}

	// Ensure output directory exists and is writable
	outputDir := filepath.Dir(g.OutputPath)
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return errors.Wrapf(err, "failed to create output directory: %s", outputDir)
		}
	}

	// Check if we can write to the output directory
	testFile := filepath.Join(outputDir, ".gotya_test_"+time.Now().Format("20060102150405"))
	f, err := os.Create(testFile)
	if err != nil {
		return errors.Wrapf(err, "output directory is not writable: %s", outputDir)
	}
	_ = f.Close()
	_ = os.Remove(testFile) // Clean up test file

	return nil
}

// CountArtifacts counts the number of .gotya files in the source directory
func (g *Generator) CountArtifacts() (int, error) {
	count := 0
	err := filepath.Walk(g.Dir, func(_ string, info os.FileInfo, _ error) error {
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".gotya") {
			count++
		}
		return nil
	})

	return count, err
}

// artifactsEqual compares two IndexArtifactDescriptor structs for equality.
// It checks all relevant fields that should be identical for artifacts with the same name and version.
func artifactsEqual(a, b *model.IndexArtifactDescriptor) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Compare all relevant fields
	if a.Name != b.Name ||
		a.Version != b.Version ||
		a.Description != b.Description ||
		a.URL != b.URL ||
		a.Checksum != b.Checksum ||
		a.Size != b.Size ||
		a.OS != b.OS ||
		a.Arch != b.Arch ||
		len(a.Dependencies) != len(b.Dependencies) {
		return false
	}

	// Compare dependencies
	for i := range a.Dependencies {
		if a.Dependencies[i] != b.Dependencies[i] {
			return false
		}
	}

	return true
}

// Generate scans Dir, builds an Index, and writes it to OutputPath.
// If a baseline index is specified, it will be loaded and used as a starting point.
// New artifacts will be added to the baseline, with conflicts detected and reported.
func (g *Generator) Generate(ctx context.Context) error {
	// Validate generator configuration
	if err := g.Validate(); err != nil {
		return err
	}

	// Count artifacts to provide better error messages
	if _, err := g.CountArtifacts(); err != nil {
		return errors.Wrap(err, "failed to count artifacts")
	}

	// Load baseline index if specified
	baselineIndex, err := g.loadBaselineIndex()
	if err != nil {
		return errors.Wrap(err, "failed to load baseline index")
	}

	// Process artifacts from the directory
	artifacts, err := g.processArtifacts(ctx, baselineIndex)
	if err != nil {
		return errors.Wrap(err, "failed to process artifacts")
	}

	// If we have artifacts, create and write the index
	if len(artifacts) > 0 {
		index := &Index{
			FormatVersion: CurrentFormatVersion,
			LastUpdate:    time.Now(),
			Artifacts:     artifacts,
		}

		if err := g.writeIndex(index); err != nil {
			return fmt.Errorf("failed to write index: %w", err)
		}
	}

	return nil
}

// loadBaselineIndex loads an index file to use as a baseline for generating a new index.
// It validates the format version and returns the loaded index or an error.
// If no baseline path is set, it returns nil without an error.
func (g *Generator) loadBaselineIndex() (*Index, error) {
	if g.BaselineIndexPath == "" {
		// Return nil, nil to indicate no baseline available (not an error condition)
		//nolint:nilnil // This is intentional: nil return value with nil error means no baseline
		return nil, nil
	}

	// Use the existing ParseIndexFromFile function to load and validate the index
	index, err := ParseIndexFromFile(g.BaselineIndexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load baseline index: %w", err)
	}

	return index, nil
}

// processArtifacts scans the directory for .gotya files and processes them.
// If a baseline is provided, it will be used to initialize the artifacts map.
// Returns a slice of artifact descriptors or an error if processing fails.
func (g *Generator) processArtifacts(ctx context.Context, baseline *Index) ([]*model.IndexArtifactDescriptor, error) {
	// Create a map to store artifacts by name@version
	artifacts := make(map[string]*model.IndexArtifactDescriptor)

	// If we have a baseline, add its artifacts to our map
	if baseline != nil {
		for _, art := range baseline.Artifacts {
			key := fmt.Sprintf("%s@%s", art.Name, art.Version)
			artifacts[key] = art
		}
	}

	// Walk the directory to find .gotya files
	err := filepath.Walk(g.Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking directory: %w", err)
		}

		// Skip directories and non-.gotya files
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".gotya") {
			return nil
		}

		// Process the artifact file
		desc, err := g.describeArtifact(ctx, path)
		if err != nil {
			return fmt.Errorf("failed to process artifact %s: %w", path, err)
		}

		// Check for conflicts with existing artifacts
		key := fmt.Sprintf("%s@%s", desc.Name, desc.Version)
		if existing, exists := artifacts[key]; exists {
			if !artifactsEqual(desc, existing) {
				return fmt.Errorf("conflict for artifact %s@%s: metadata differs from baseline: %w", desc.Name, desc.Version, errors.ErrIndexConflict)
			}
		}
		artifacts[key] = desc

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error processing artifacts: %w", err)
	}

	// Convert the map to a slice
	artifactList := make([]*model.IndexArtifactDescriptor, 0, len(artifacts))
	for _, art := range artifacts {
		artifactList = append(artifactList, art)
	}

	return artifactList, nil
}

// describeArtifact opens an artifact file, reads its metadata, and returns a descriptor.
func (g *Generator) describeArtifact(ctx context.Context, filePath string) (*model.IndexArtifactDescriptor, error) {
	// Create a temporary directory to extract the metadata file
	tempDir, err := os.MkdirTemp("", "gotya-describe")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Extract the metadata file from the artifact
	archiveManager := archive.NewManager()
	metaFilePath := filepath.Join(tempDir, "artifact.json")
	err = archiveManager.ExtractFile(ctx, filePath, path.Join("meta", "artifact.json"), metaFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract metadata: %w", err)
	}

	// Read and parse the metadata file
	metaFile, err := os.Open(metaFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open metadata file: %w", err)
	}
	defer func() { _ = metaFile.Close() }()

	md := &artifact.Metadata{}
	if err := json.NewDecoder(metaFile).Decode(md); err != nil {
		return nil, fmt.Errorf("failed to decode metadata: %w", err)
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

	desc := &model.IndexArtifactDescriptor{
		Name:         md.Name,
		Version:      md.Version,
		Description:  md.Description,
		URL:          urlStr,
		Checksum:     checksum,
		Size:         stat.Size(),
		OS:           md.GetOS(),
		Arch:         md.GetArch(),
		Dependencies: md.Dependencies,
	}
	return desc, nil
}

// writeIndex writes the index to the output file.
func (g *Generator) writeIndex(index *Index) error {
	// Ensure the output directory exists
	if err := os.MkdirAll(filepath.Dir(g.OutputPath), 0o755); err != nil {
		return errors.Wrapf(err, "failed to create output directory for %s", g.OutputPath)
	}

	// Create the output file
	f, err := os.Create(g.OutputPath)
	if err != nil {
		return errors.Wrapf(err, "failed to create output file %s", g.OutputPath)
	}
	defer func() { _ = f.Close() }()

	// Encode and write the index with pretty-printing
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(index); err != nil {
		return errors.Wrapf(err, "failed to encode index to %s", g.OutputPath)
	}

	return nil
}

func (g *Generator) makeURL(artifactPath string) (string, error) {
	// Get just the filename from the artifact path
	artifactFileName := filepath.Base(artifactPath)

	// Construct URL as <basepath>/<artifact>
	// The idea is that the input directory only contains artifacts directly,
	// and the file list should never contain any path elements.
	var relURL string
	if g.BasePath != "" {
		// Clean and join using path (URL style)
		relURL = path.Join(strings.TrimSuffix(g.BasePath, "/"), artifactFileName)
	} else {
		relURL = artifactFileName
	}

	// Validate it parses as a URL (relative URL is fine)
	if _, err := url.Parse(relURL); err != nil {
		return "", err
	}
	return relURL, nil
}

// sha256File computes the SHA256 checksum of a file.
func sha256File(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
