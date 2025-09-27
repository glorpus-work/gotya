//go:build integration

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cperrin88/gotya/pkg/index"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndex_Generate_SimpleSuccess(t *testing.T) {
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	repoDir := filepath.Join(tempDir, "repo")
	outFile := filepath.Join(repoDir, "index.json")

	// Create artifacts
	src1 := createSampleArtifactSource(t, tempDir)
	_ = createArtifactViaCLI(t, src1, "alpha", "1.0.0", artifactsDir)

	src2 := createSampleArtifactSource(t, tempDir)
	_ = createArtifactViaCLI(t, src2, "beta", "2.0.0", artifactsDir)

	// Generate index
	generateIndexViaCLI(t, artifactsDir, outFile, "packages", true)

	// Validate index file exists and basic content
	if _, err := os.Stat(outFile); err != nil {
		t.Fatalf("expected index file to be created at %s: %v", outFile, err)
	}

	indexFile, err := index.ParseIndexFromFile(outFile)
	require.NoError(t, err)
	require.NotNil(t, indexFile)
	require.GreaterOrEqual(t, len(indexFile.Artifacts), 2)

	names := map[string]bool{}
	for _, it := range indexFile.Artifacts {
		names[it.Name] = true
		assert.NotEmpty(t, it.URL)
		// URL should be a relative path (not absolute URL or filesystem root)
		assert.False(t, strings.HasPrefix(it.URL, "http"), "url should be relative, not absolute")
		assert.False(t, strings.HasPrefix(it.URL, "/"), "url should be relative, not absolute")
		assert.True(t, strings.HasSuffix(strings.ToLower(it.URL), ".gotya"))
		assert.Equal(t, 64, len(it.Checksum))
		assert.NotZero(t, it.Size)
		assert.NotEmpty(t, it.OS)
		assert.NotEmpty(t, it.Arch)
	}
	assert.True(t, names["alpha"])
	assert.True(t, names["beta"])
}

func TestIndex_Generate_FailsForMissingSourceDir(t *testing.T) {
	tempDir := t.TempDir()
	outFile := filepath.Join(tempDir, "index.json")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"index", "generate", filepath.Join(tempDir, "nope"), outFile})
	err := cmd.ExecuteContext(context.Background())
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "source directory")
}

func TestIndex_Generate_OverwriteBehavior(t *testing.T) {
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	repoDir := filepath.Join(tempDir, "repo")
	outFile := filepath.Join(repoDir, "index.json")

	src := createSampleArtifactSource(t, tempDir)
	_ = createArtifactViaCLI(t, src, "gamma", "0.1.0", artifactsDir)

	require.NoError(t, os.MkdirAll(filepath.Dir(outFile), 0o755))
	require.NoError(t, os.WriteFile(outFile, []byte("{}"), 0o644))

	// Without --force should fail
	cmd := newRootCmd()
	cmd.SetArgs([]string{"index", "generate", artifactsDir, outFile})
	err := cmd.ExecuteContext(context.Background())
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "output file exists")

	// With --force succeeds and produces valid index
	generateIndexViaCLI(t, artifactsDir, outFile, "", true)
	parsed, perr := index.ParseIndexFromFile(outFile)
	require.NoError(t, perr)
	require.NotNil(t, parsed)
	assert.GreaterOrEqual(t, len(parsed.Artifacts), 1)
}
