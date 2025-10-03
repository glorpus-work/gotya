// Package index provides functionality for working with package indexes in the gotya package manager.
// It handles the creation, parsing, and querying of package indexes, which contain metadata
// about available packages, their versions, and platform-specific artifacts. The package
// supports versioning, filtering, and serialization of index data in JSON format.
//
// The index package is a core component that enables package discovery, version resolution,
// and dependency management in the gotya ecosystem.
package index

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/glorpus-work/gotya/pkg/errors"
	"github.com/glorpus-work/gotya/pkg/model"
)

// ScoredArtifact represents an artifact with its relevance score for sorting
type ScoredArtifact struct {
	Artifact *model.IndexArtifactDescriptor
	Score    int
}

// GetFormatVersion returns the format version.
func (idx *Index) GetFormatVersion() string {
	return idx.FormatVersion
}

// GetLastUpdate returns the last update timestamp as string.
func (idx *Index) GetLastUpdate() string {
	return idx.LastUpdate.Format(time.RFC3339)
}

// GetArtifacts returns all packages.
func (idx *Index) GetArtifacts() []*model.IndexArtifactDescriptor {
	return idx.Artifacts
}

// ParseIndex parses an index from JSON data.
func ParseIndex(data []byte) (*Index, error) {
	var index Index
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, errors.Wrap(err, "failed to parse index")
	}

	// Validate format version
	if index.FormatVersion == "" {
		return nil, fmt.Errorf("missing format version in index: %w", errors.ErrValidation)
	}

	return &index, nil
}

// ParseIndexFromReader parses an index from an io.Reader.
func ParseIndexFromReader(reader io.Reader) (*Index, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read index data")
	}

	return ParseIndex(data)
}

// ParseIndexFromFile reads and parses an index from the specified file path.
func ParseIndexFromFile(filePath string) (*Index, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, errors.Wrapf(err, "Cannot open index file %s for parsing", filePath)
	}
	defer func() { _ = file.Close() }()
	return ParseIndexFromReader(file)
}

// ToJSON converts the index to JSON bytes.
func (idx *Index) ToJSON() ([]byte, error) {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal index to JSON")
	}
	return data, nil
}

// FindArtifacts returns all artifacts with the specified name from this index.
func (idx *Index) FindArtifacts(name string) []*model.IndexArtifactDescriptor {
	packages := make([]*model.IndexArtifactDescriptor, 0, 5)
	for _, pkg := range idx.Artifacts {
		if pkg.Name == name {
			packages = append(packages, pkg)
		}
	}

	return packages
}

// FuzzySearchArtifacts performs fuzzy search for artifacts by name.
// Returns artifacts sorted by relevance (best matches first).
func (idx *Index) FuzzySearchArtifacts(query string) []*model.IndexArtifactDescriptor {
	if query == "" {
		return []*model.IndexArtifactDescriptor{}
	}

	query = strings.ToLower(query)
	var scoredMatches []ScoredArtifact

	for _, pkg := range idx.Artifacts {
		// Calculate similarity score
		score := fuzzyMatchScore(query, strings.ToLower(pkg.Name))
		if score > 0 {
			scoredMatches = append(scoredMatches, ScoredArtifact{
				Artifact: pkg,
				Score:    score,
			})
		}
	}

	// Sort by relevance (higher scores first)
	for i := 0; i < len(scoredMatches)-1; i++ {
		for j := i + 1; j < len(scoredMatches); j++ {
			if scoredMatches[j].Score > scoredMatches[i].Score {
				scoredMatches[i], scoredMatches[j] = scoredMatches[j], scoredMatches[i]
			}
		}
	}

	// Extract artifacts from scored results
	matches := make([]*model.IndexArtifactDescriptor, len(scoredMatches))
	for i, scored := range scoredMatches {
		matches[i] = scored.Artifact
	}

	return matches
}

// fuzzyMatchScore calculates a similarity score between query and target.
// Returns 0 if no match, higher scores for better matches (0-100 range).
func fuzzyMatchScore(query, target string) int {
	if query == target {
		return 100 // Exact match
	}

	// Simple fuzzy matching: check if query is contained in target
	if strings.Contains(target, query) {
		// Bonus for prefix matches
		if strings.HasPrefix(target, query) {
			return 90
		}
		// Penalty for substring matches (less specific)
		return 70
	}

	// Check for partial word matches
	queryWords := strings.Fields(query)
	targetWords := strings.Fields(target)

	for _, qWord := range queryWords {
		for _, tWord := range targetWords {
			if strings.Contains(tWord, qWord) {
				return 50
			}
		}
	}

	return 0 // No match
}

// WriteIndexToFile writes the index to the specified file path.
func WriteIndexToFile(idx *Index, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create index file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Encode and write the index with pretty-printing
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err := enc.Encode(idx); err != nil {
		return fmt.Errorf("failed to encode index: %w", err)
	}

	return nil
}
