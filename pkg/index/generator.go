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
	// ForceOverwrite controls whether to overwrite an existing output file.
	ForceOverwrite bool
	// BaselineIndexPath is an optional path to an existing index file to use as a baseline.
	// The new index will include all artifacts from the baseline that don't conflict with new artifacts.
	BaselineIndexPath string
}

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
	if f, err := os.Create(testFile); err != nil {
		return errors.Wrapf(err, "output directory is not writable: %s", outputDir)
	} else {
		f.Close()
		os.Remove(testFile) // Clean up test file
	}

	return nil
}

// CountArtifacts counts the number of .gotya files in the source directory
func (g *Generator) CountArtifacts() (int, error) {
	count := 0
	err := filepath.Walk(g.Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
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

// findConflicts checks for conflicts between a new artifact and existing artifacts in the baseline.
// Returns an error if a conflict is found, nil otherwise.
func findConflicts(newArtifact *model.IndexArtifactDescriptor, baselineArtifacts []*model.IndexArtifactDescriptor) error {
	for _, existing := range baselineArtifacts {
		if existing.Name == newArtifact.Name && existing.Version == newArtifact.Version {
			// Found an artifact with the same name and version, check if they're identical
			if !artifactsEqual(existing, newArtifact) {
				return errors.Wrapf(errors.ErrIndexConflict,
					"conflict for artifact %s@%s: artifacts with the same name and version must be identical",
					existing.Name, existing.Version)
			}
		}
	}
	return nil
}

// mergeArtifacts merges new artifacts with baseline artifacts, checking for conflicts.
// Returns the merged list of artifacts.
func mergeArtifacts(baselineArtifacts, newArtifacts []*model.IndexArtifactDescriptor) ([]*model.IndexArtifactDescriptor, error) {
	// Create a map of existing artifacts for quick lookup
	existingArtifacts := make(map[string]map[string]*model.IndexArtifactDescriptor)
	for _, artifact := range baselineArtifacts {
		if _, exists := existingArtifacts[artifact.Name]; !exists {
			existingArtifacts[artifact.Name] = make(map[string]*model.IndexArtifactDescriptor)
		}
		existingArtifacts[artifact.Name][artifact.Version] = artifact
	}

	// Process new artifacts, checking for conflicts
	for _, artifact := range newArtifacts {
		if versions, exists := existingArtifacts[artifact.Name]; exists {
			// Artifact with this name exists, check versions
			if existing, versionExists := versions[artifact.Version]; versionExists {
				// Version exists, check for conflicts
				if !artifactsEqual(existing, artifact) {
					return nil, errors.Wrapf(errors.ErrIndexConflict,
						"conflict for artifact %s@%s: artifacts with the same name and version must be identical",
						artifact.Name, artifact.Version)
				}
				// No conflict, skip adding this artifact as it already exists
				continue
			}
		}

		// No conflict, add the artifact to the baseline
		baselineArtifacts = append(baselineArtifacts, artifact)
	}

	return baselineArtifacts, nil
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
		for _, artifact := range baseline.Artifacts {
			key := fmt.Sprintf("%s@%s", artifact.Name, artifact.Version)
			artifacts[key] = artifact
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
				return fmt.Errorf("conflict for artifact %s@%s: metadata differs from baseline", desc.Name, desc.Version)
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
	for _, artifact := range artifacts {
		artifactList = append(artifactList, artifact)
	}

	return artifactList, nil
}

// describeArtifact opens an artifact file, reads its metadata, and returns a descriptor.
func (g *Generator) describeArtifact(ctx context.Context, filePath string) (*model.IndexArtifactDescriptor, error) {
	// Open as archive filesystem and read meta/metadata.json
	fsys, err := archives.FileSystem(ctx, filePath, nil)
	if err != nil {
		return nil, err
	}
	// Use path.Join for archive-internal paths (always forward slashes)
	metaFile, err := fsys.Open(path.Join("meta", "artifact.json"))
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
	defer f.Close()

	// Encode and write the index with pretty-printing
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(index); err != nil {
		return errors.Wrapf(err, "failed to encode index to %s", g.OutputPath)
	}

	return nil
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

// sha256File computes the SHA256 checksum of a file.
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
