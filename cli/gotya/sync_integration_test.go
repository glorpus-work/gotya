//go:build integration

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/cperrin88/gotya/pkg/index"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSync_SuccessDownloadsIndex(t *testing.T) {
	tempDir := t.TempDir()
	// Build a repo with two artifacts and serve it
	defs := [][2]string{{"alpha", "1.0.0"}, {"beta", "2.1.0"}}
	repoDir, _ := buildRepoDirWithArtifacts(t, tempDir, defs)
	srv, idxURL := startRepoServer(t, repoDir)
	defer srv.Close()

	// Write a temporary config pointing to our served index.json
	cfgPath := filepath.Join(tempDir, "config.yaml")
	cacheDir := filepath.Join(tempDir, "cache")
	writeTempConfig(t, cfgPath, "testrepo", idxURL, cacheDir)

	// Run sync
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "sync"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify index was downloaded to cacheDir/indexes/testrepo.json
	downloaded := filepath.Join(cacheDir, "indexes", "testrepo.json")
	if _, err := os.Stat(downloaded); err != nil {
		t.Fatalf("expected downloaded index at %s: %v", downloaded, err)
	}

	// Parse and validate content
	parsed, err := index.ParseIndexFromFile(downloaded)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	assert.GreaterOrEqual(t, len(parsed.Artifacts), 2)
}

func TestSync_FailsForMissingIndex(t *testing.T) {
	tempDir := t.TempDir()
	// Serve an empty directory; index.json will be missing and return 404
	emptyDir := filepath.Join(tempDir, "empty")
	require.NoError(t, os.MkdirAll(emptyDir, 0o755))
	srv := httptest.NewServer(http.FileServer(http.Dir(emptyDir)))
	defer srv.Close()

	// Point repo URL to missing index.json
	cfgPath := filepath.Join(tempDir, "config.yaml")
	cacheDir := filepath.Join(tempDir, "cache")
	writeTempConfig(t, cfgPath, "badrepo", srv.URL+"/index.json", cacheDir)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "sync"})
	err := cmd.ExecuteContext(context.Background())
	require.Error(t, err)
}

func TestSync_NoRepositoriesDoesNothing(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.yaml")
	cacheDir := filepath.Join(tempDir, "cache")
	// Write config with empty repositories
	writeTempConfig(t, cfgPath, "", "", cacheDir)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "sync"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Ensure no indexes directory is created (or empty)
	idxDir := filepath.Join(cacheDir, "indexes")
	if fi, err := os.Stat(idxDir); err == nil && fi.IsDir() {
		entries, _ := os.ReadDir(idxDir)
		assert.Equal(t, 0, len(entries))
	}
}
