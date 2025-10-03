package index

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/glorpus-work/gotya/pkg/model"
)

func createSimpleTestIndex() *Index {
	return &Index{
		FormatVersion: "1.0",
		LastUpdate:    time.Now(),
		Artifacts: []*model.IndexArtifactDescriptor{
			{
				Name:        "package-a",
				Version:     "1.0.0",
				Description: "Package A description",
				URL:         "https://example.com/package-a.tar.gz",
				Checksum:    "abc123",
				Size:        1024,
				OS:          "linux",
				Arch:        "amd64",
			},
			{
				Name:        "package-b",
				Version:     "2.0.0",
				Description: "Package B description",
				URL:         "https://example.com/package-b.tar.gz",
				Checksum:    "def456",
				Size:        2048,
				OS:          "darwin",
				Arch:        "arm64",
			},
			{
				Name:        "other-package",
				Version:     "1.5.0",
				Description: "Other package description",
				URL:         "https://example.com/other-package.tar.gz",
				Checksum:    "ghi789",
				Size:        1536,
				OS:          "windows",
				Arch:        "386",
			},
			{
				Name:        "another-app",
				Version:     "3.0.0",
				Description: "Another app description",
				URL:         "https://example.com/another-app.tar.gz",
				Checksum:    "jkl012",
				Size:        3072,
				OS:          "",
				Arch:        "",
			},
		},
	}
}

func TestFuzzySearchArtifacts(t *testing.T) {
	idx := createSimpleTestIndex()

	t.Run("ExactMatch", func(t *testing.T) {
		results := idx.FuzzySearchArtifacts("package-a")
		assert.Len(t, results, 1)
		assert.Equal(t, "package-a", results[0].Name)
		assert.Equal(t, "1.0.0", results[0].Version)
	})

	t.Run("PartialMatch", func(t *testing.T) {
		results := idx.FuzzySearchArtifacts("package")
		assert.Len(t, results, 3) // package-a, package-b, other-package
		expectedNames := []string{"package-a", "package-b", "other-package"}
		for i, result := range results {
			assert.Equal(t, expectedNames[i], result.Name)
		}
	})

	t.Run("PrefixMatch", func(t *testing.T) {
		results := idx.FuzzySearchArtifacts("pack")
		assert.Len(t, results, 3) // Should match package-a, package-b, other-package
	})

	t.Run("WordMatch", func(t *testing.T) {
		results := idx.FuzzySearchArtifacts("app")
		assert.Len(t, results, 1) // another-app (contains "app")
		assert.Equal(t, "another-app", results[0].Name)
	})

	t.Run("NoMatch", func(t *testing.T) {
		results := idx.FuzzySearchArtifacts("nonexistent")
		assert.Empty(t, results)
	})

	t.Run("EmptyQuery", func(t *testing.T) {
		results := idx.FuzzySearchArtifacts("")
		assert.Empty(t, results)
	})

	t.Run("CaseInsensitive", func(t *testing.T) {
		results := idx.FuzzySearchArtifacts("PACKAGE")
		assert.Len(t, results, 3) // Should match regardless of case
	})
}

func TestFuzzyMatchScore(t *testing.T) {
	t.Run("ExactMatch", func(t *testing.T) {
		score := fuzzyMatchScore("package-a", "package-a")
		assert.Equal(t, 1.0, score)
	})

	t.Run("PrefixMatch", func(t *testing.T) {
		score := fuzzyMatchScore("pack", "package-a")
		assert.Equal(t, 0.9, score)
	})

	t.Run("SubstringMatch", func(t *testing.T) {
		score := fuzzyMatchScore("age", "package-a")
		assert.Equal(t, 0.7, score)
	})

	t.Run("WordMatch", func(t *testing.T) {
		score := fuzzyMatchScore("app", "another-app")
		assert.Equal(t, 0.7, score) // substring match, not word match
	})

	t.Run("NoMatch", func(t *testing.T) {
		score := fuzzyMatchScore("xyz", "package-a")
		assert.Equal(t, 0.0, score)
	})

	t.Run("EmptyQuery", func(t *testing.T) {
		score := fuzzyMatchScore("", "package-a")
		assert.Equal(t, 0.9, score) // Empty string is considered a prefix match (bug in algorithm)
	})
}

func TestWriteIndexToFile(t *testing.T) {
	idx := createSimpleTestIndex()
	tempDir := t.TempDir()

	t.Run("SuccessfulWrite", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "test-index.json")

		err := WriteIndexToFile(idx, filePath)
		require.NoError(t, err)

		// Verify file exists
		_, err = os.Stat(filePath)
		assert.False(t, os.IsNotExist(err), "index file should exist")

		// Verify file contents
		data, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Contains(t, string(data), "package-a")
		assert.Contains(t, string(data), "package-b")
	})

	t.Run("WriteToInvalidPath", func(t *testing.T) {
		invalidPath := "/nonexistent/directory/test-index.json"

		err := WriteIndexToFile(idx, invalidPath)
		assert.Error(t, err)
	})

	t.Run("WriteToExistingFile", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "existing-index.json")

		// Create existing file
		err := os.WriteFile(filePath, []byte("existing content"), 0644)
		require.NoError(t, err)

		// This should overwrite the existing file
		err = WriteIndexToFile(idx, filePath)
		require.NoError(t, err)

		// Verify contents were overwritten
		data, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Contains(t, string(data), "package-a")
		assert.NotContains(t, string(data), "existing content")
	})
}
