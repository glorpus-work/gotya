//go:build integration

package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCache_CacheInfoAndDirectory(t *testing.T) {
	tempDir := t.TempDir()

	// Write a temporary config
	cfgPath := filepath.Join(tempDir, "config.yaml")
	cacheDir := filepath.Join(tempDir, "cache")
	installDir := filepath.Join(tempDir, "install")
	metaDir := filepath.Join(tempDir, "meta")
	stateDir := filepath.Join(tempDir, "state")

	yamlContent := `settings:
  cache_dir: ` + cacheDir + `
  install_dir: ` + installDir + `
  meta_dir: ` + metaDir + `
  state_dir: ` + stateDir + `
  http_timeout: 5s
  max_concurrent_syncs: 2
repositories: []
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(yamlContent), 0o600))

	// Create the state directory structure
	require.NoError(t, os.MkdirAll(filepath.Join(stateDir, "gotya", "state"), 0o755))

	// Test cache info command
	infoCmd := newRootCmd()
	infoCmd.SetArgs([]string{"--config", cfgPath, "cache", "info"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := infoCmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	// Restore stdout and read output
	_ = w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify cache info is displayed (note: actual format is "Cache Information:\n  Directory:")
	assert.Contains(t, output, "Cache Information:")
	assert.Contains(t, output, "Directory:")
	assert.Contains(t, output, cacheDir)
	assert.Contains(t, output, "Total Size:")

	// Test cache dir command
	dirCmd := newRootCmd()
	dirCmd.SetArgs([]string{"--config", cfgPath, "cache", "dir"})

	// Capture stdout for dir command
	r2, w2, _ := os.Pipe()
	os.Stdout = w2

	err2 := dirCmd.ExecuteContext(context.Background())
	require.NoError(t, err2)

	// Restore stdout and read output
	_ = w2.Close()
	os.Stdout = oldStdout

	var buf2 strings.Builder
	_, _ = io.Copy(&buf2, r2)
	output2 := buf2.String()

	// Verify cache directory path is shown
	assert.Contains(t, output2, cacheDir)
}

func TestCache_CacheCleanOperations(t *testing.T) {
	tempDir := t.TempDir()

	// Build a repo with artifacts and serve it
	defs := [][2]string{{"testapp", "1.0.0"}}
	repoDir, _ := buildRepoDirWithArtifacts(t, tempDir, defs)
	srv, idxURL := startRepoServer(t, repoDir)
	defer srv.Close()

	// Write a temporary config
	cfgPath := filepath.Join(tempDir, "config.yaml")
	cacheDir := filepath.Join(tempDir, "cache")
	installDir := filepath.Join(tempDir, "install")
	metaDir := filepath.Join(tempDir, "meta")
	stateDir := filepath.Join(tempDir, "state")

	yamlContent := `settings:
  cache_dir: ` + cacheDir + `
  install_dir: ` + installDir + `
  meta_dir: ` + metaDir + `
  state_dir: ` + stateDir + `
  http_timeout: 5s
  max_concurrent_syncs: 2
repositories:
  - name: testrepo
    url: ` + idxURL + `
    enabled: true
    priority: 1
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(yamlContent), 0o600))

	// Create the state directory structure
	require.NoError(t, os.MkdirAll(filepath.Join(stateDir, "gotya", "state"), 0o755))

	// First sync to download the index (creates cache files)
	syncCmd := newRootCmd()
	syncCmd.SetArgs([]string{"--config", cfgPath, "sync"})
	require.NoError(t, syncCmd.ExecuteContext(context.Background()))

	// Verify cache directory exists and has files
	cacheFiles, err := filepath.Glob(filepath.Join(cacheDir, "*"))
	require.NoError(t, err)
	assert.NotEmpty(t, cacheFiles, "cache should contain files after sync")

	// Test cache clean --all command
	cleanCmd := newRootCmd()
	cleanCmd.SetArgs([]string{"--config", cfgPath, "cache", "clean", "--all"})

	err = cleanCmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	// Verify cache was cleaned (check that cache info shows zero size)
	infoCmdAfter := newRootCmd()
	infoCmdAfter.SetArgs([]string{"--config", cfgPath, "cache", "info"})

	// Capture stdout for cache info after cleaning
	oldStdout := os.Stdout
	rAfter, wAfter, _ := os.Pipe()
	os.Stdout = wAfter

	errAfter := infoCmdAfter.ExecuteContext(context.Background())
	require.NoError(t, errAfter)

	// Restore stdout and read output
	_ = wAfter.Close()
	os.Stdout = oldStdout

	var bufAfter strings.Builder
	_, _ = io.Copy(&bufAfter, rAfter)
	outputAfter := bufAfter.String()

	// Cache should show zero size after cleaning
	assert.Contains(t, outputAfter, "Total Size:   0 B")
}

func TestCache_CacheWithCustomDirectories(t *testing.T) {
	tempDir := t.TempDir()
	customCacheDir := filepath.Join(tempDir, "custom_cache")

	// Write a temporary config with custom cache directory
	cfgPath := filepath.Join(tempDir, "config.yaml")
	installDir := filepath.Join(tempDir, "install")
	metaDir := filepath.Join(tempDir, "meta")
	stateDir := filepath.Join(tempDir, "state")

	yamlContent := `settings:
  cache_dir: ` + customCacheDir + `
  install_dir: ` + installDir + `
  meta_dir: ` + metaDir + `
  state_dir: ` + stateDir + `
  http_timeout: 5s
  max_concurrent_syncs: 2
repositories: []
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(yamlContent), 0o600))

	// Create the state directory structure
	require.NoError(t, os.MkdirAll(filepath.Join(stateDir, "gotya", "state"), 0o755))

	// Test cache dir command with custom directory
	dirCmd := newRootCmd()
	dirCmd.SetArgs([]string{"--config", cfgPath, "cache", "dir"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := dirCmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	// Restore stdout and read output
	_ = w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify custom cache directory path is shown
	assert.Contains(t, output, customCacheDir)
}

func TestCache_CacheCorruptionRecovery(t *testing.T) {
	tempDir := t.TempDir()

	// Write a temporary config
	cfgPath := filepath.Join(tempDir, "config.yaml")
	cacheDir := filepath.Join(tempDir, "cache")
	installDir := filepath.Join(tempDir, "install")
	metaDir := filepath.Join(tempDir, "meta")
	stateDir := filepath.Join(tempDir, "state")

	yamlContent := `settings:
  cache_dir: ` + cacheDir + `
  install_dir: ` + installDir + `
  meta_dir: ` + metaDir + `
  state_dir: ` + stateDir + `
  http_timeout: 5s
  max_concurrent_syncs: 2
repositories: []
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(yamlContent), 0o600))

	// Create the state directory structure
	require.NoError(t, os.MkdirAll(filepath.Join(stateDir, "gotya", "state"), 0o755))

	// Create some corrupted cache files in the indexes directory (which gets cleaned)
	require.NoError(t, os.MkdirAll(filepath.Join(cacheDir, "indexes"), 0o755))
	corruptedFile := filepath.Join(cacheDir, "indexes", "corrupted_index.json")
	require.NoError(t, os.WriteFile(corruptedFile, []byte("invalid json content{"), 0o600))

	// Test cache info with corrupted files - should handle gracefully
	infoCmd := newRootCmd()
	infoCmd.SetArgs([]string{"--config", cfgPath, "cache", "info"})

	err := infoCmd.ExecuteContext(context.Background())
	// The command might succeed or fail depending on how corruption is handled
	// The important thing is that it doesn't crash the application
	if err != nil {
		// If it fails, it should be due to corruption, not a panic
		assert.Contains(t, err.Error(), "cache")
	} else {
		// If it succeeds, cache info should still be displayed
		t.Log("Cache info command handled corruption gracefully")
	}

	// Test cache clean --all to recover from corruption
	cleanCmd := newRootCmd()
	cleanCmd.SetArgs([]string{"--config", cfgPath, "cache", "clean", "--all"})

	err = cleanCmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	// Verify cache was cleaned (check that cache info shows zero size)
	infoCmdAfter := newRootCmd()
	infoCmdAfter.SetArgs([]string{"--config", cfgPath, "cache", "info"})

	// Capture stdout for cache info after cleaning
	oldStdout := os.Stdout
	rAfter, wAfter, _ := os.Pipe()
	os.Stdout = wAfter

	errAfter := infoCmdAfter.ExecuteContext(context.Background())
	require.NoError(t, errAfter)

	// Restore stdout and read output
	_ = wAfter.Close()
	os.Stdout = oldStdout

	var bufAfter strings.Builder
	_, _ = io.Copy(&bufAfter, rAfter)
	outputAfter := bufAfter.String()

	// Cache should show zero size after cleaning
	assert.Contains(t, outputAfter, "Total Size:   0 B")

	// Verify corrupted file is gone (it should have been cleaned)
	_, err = os.Stat(corruptedFile)
	assert.True(t, os.IsNotExist(err), "corrupted file should be removed")
}
