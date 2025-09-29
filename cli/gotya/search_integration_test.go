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

func TestSearch_BasicFuzzySearch(t *testing.T) {
	tempDir := t.TempDir()

	// Build a repo with multiple artifacts for searching
	defs := [][2]string{
		{"testapp", "1.0.0"},
		{"testlib", "2.0.0"},
		{"myapp", "1.5.0"},
		{"otherlib", "3.0.0"},
		{"database-tool", "1.2.0"},
	}
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

	// First sync to download the index
	syncCmd := newRootCmd()
	syncCmd.SetArgs([]string{"--config", cfgPath, "sync"})
	require.NoError(t, syncCmd.ExecuteContext(context.Background()))

	// Test basic fuzzy search for "test"
	searchCmd := newRootCmd()
	searchCmd.SetArgs([]string{"--config", cfgPath, "search", "test"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := searchCmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	// Restore stdout and read output
	_ = w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify header and results are present
	assert.Contains(t, output, "PACKAGE NAME")
	assert.Contains(t, output, "VERSION")
	assert.Contains(t, output, "OS/ARCH")
	assert.Contains(t, output, "DESCRIPTION")

	// Should find testapp and testlib
	assert.Contains(t, output, "testapp")
	assert.Contains(t, output, "testlib")
	// Should NOT find myapp, otherlib, or database-tool
	assert.NotContains(t, output, "myapp")
	assert.NotContains(t, output, "otherlib")
	assert.NotContains(t, output, "database-tool")

	// Test fuzzy search for "app"
	searchCmd2 := newRootCmd()
	searchCmd2.SetArgs([]string{"--config", cfgPath, "search", "app"})

	// Capture stdout for second test
	r2, w2, _ := os.Pipe()
	os.Stdout = w2

	err2 := searchCmd2.ExecuteContext(context.Background())
	require.NoError(t, err2)

	// Restore stdout and read output
	_ = w2.Close()
	os.Stdout = oldStdout

	var buf2 strings.Builder
	_, _ = io.Copy(&buf2, r2)
	output2 := buf2.String()

	// Should find testapp and myapp
	assert.Contains(t, output2, "testapp")
	assert.Contains(t, output2, "myapp")
	// Should NOT find testlib, otherlib, or database-tool
	assert.NotContains(t, output2, "testlib")
	assert.NotContains(t, output2, "otherlib")
	assert.NotContains(t, output2, "database-tool")
}

func TestSearch_SearchWithNoResults(t *testing.T) {
	tempDir := t.TempDir()

	// Build a repo with artifacts
	defs := [][2]string{
		{"testapp", "1.0.0"},
		{"testlib", "2.0.0"},
	}
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

	// First sync to download the index
	syncCmd := newRootCmd()
	syncCmd.SetArgs([]string{"--config", cfgPath, "sync"})
	require.NoError(t, syncCmd.ExecuteContext(context.Background()))

	// Search for something that doesn't exist - should succeed with "No packages found" message
	searchCmd := newRootCmd()
	searchCmd.SetArgs([]string{"--config", cfgPath, "search", "nonexistentpackage"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := searchCmd.ExecuteContext(context.Background())
	// The search should succeed (no error) and show "No packages found" message
	require.NoError(t, err)

	// Restore stdout and read output
	_ = w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Should show "No packages found" message
	assert.Contains(t, output, "No packages found matching 'nonexistentpackage'")
}

func TestSearch_SearchAcrossMultipleRepositories(t *testing.T) {
	tempDir := t.TempDir()

	// Build two repos with different artifacts
	// Repo 1: testlib
	libDefs := [][2]string{{"testlib", "1.0.0"}}
	libRepoDir, _ := buildRepoDirWithArtifacts(t, tempDir, libDefs)
	libSrv, libURL := startRepoServer(t, libRepoDir)
	defer libSrv.Close()

	// Repo 2: testapp
	appDefs := [][2]string{{"testapp", "2.0.0"}}
	appRepoDir, _ := buildRepoDirWithArtifacts(t, tempDir, appDefs)
	appSrv, appURL := startRepoServer(t, appRepoDir)
	defer appSrv.Close()

	// Write config with both repositories
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
  - name: lib-repo
    url: ` + libURL + `
    enabled: true
    priority: 1
  - name: app-repo
    url: ` + appURL + `
    enabled: true
    priority: 2
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(yamlContent), 0o600))

	// Create the state directory structure
	require.NoError(t, os.MkdirAll(filepath.Join(stateDir, "gotya", "state"), 0o755))

	// First sync to download the indexes for both repositories
	syncCmd := newRootCmd()
	syncCmd.SetArgs([]string{"--config", cfgPath, "sync"})
	require.NoError(t, syncCmd.ExecuteContext(context.Background()))

	// Search for "test" - should find packages from both repositories
	searchCmd := newRootCmd()
	searchCmd.SetArgs([]string{"--config", cfgPath, "search", "test"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := searchCmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	// Restore stdout and read output
	_ = w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Should find packages from both repositories (each has 2 packages)
	assert.Contains(t, output, "lib-repo:")
	assert.Contains(t, output, "app-repo:")
	assert.Contains(t, output, "testlib")
	assert.Contains(t, output, "testapp")

	// Should show total count (2 packages from each repo = 4 total)
	assert.Contains(t, output, "Found 4 package(s) matching 'test'")
}

func TestSearch_SearchWithSpecialCharacters(t *testing.T) {
	tempDir := t.TempDir()

	// Build a repo with artifacts containing special characters
	defs := [][2]string{
		{"test-app", "1.0.0"},    // hyphen
		{"test_app", "1.0.0"},    // underscore
		{"test.app", "1.0.0"},    // dot
		{"test123", "1.0.0"},     // numbers
		{"test-app-v2", "2.0.0"}, // multiple special chars
	}
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

	// First sync to download the index
	syncCmd := newRootCmd()
	syncCmd.SetArgs([]string{"--config", cfgPath, "sync"})
	require.NoError(t, syncCmd.ExecuteContext(context.Background()))

	// Test searching with special characters - search for "test" to find all variants
	searchCmd := newRootCmd()
	searchCmd.SetArgs([]string{"--config", cfgPath, "search", "test"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := searchCmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	// Restore stdout and read output
	_ = w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Should find packages with hyphens, underscores, and dots
	assert.Contains(t, output, "test-app")
	assert.Contains(t, output, "test_app")
	assert.Contains(t, output, "test.app")
	assert.Contains(t, output, "test123")
	assert.Contains(t, output, "test-app-v2")
}
