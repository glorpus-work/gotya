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

func TestList_ListAllInstalledPackages(t *testing.T) {
	tempDir := t.TempDir()

	// Build a repo with multiple artifacts and serve it
	defs := [][2]string{
		{"packageA", "1.0.0"},
		{"packageB", "2.0.0"},
		{"packageC", "1.5.0"},
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

	// Install multiple packages
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "packageA", "packageB", "packageC"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify packages are installed
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	var hasA, hasB, hasC bool
	for _, a := range installed {
		if a.Name == "packageA" && a.Version == "1.0.0" {
			hasA = true
		}
		if a.Name == "packageB" && a.Version == "2.0.0" {
			hasB = true
		}
		if a.Name == "packageC" && a.Version == "1.5.0" {
			hasC = true
		}
	}
	require.True(t, hasA, "packageA@1.0.0 should be installed")
	require.True(t, hasB, "packageB@2.0.0 should be installed")
	require.True(t, hasC, "packageC@1.5.0 should be installed")

	// Now list all packages - capture output instead of using DB directly
	// to test the actual list command functionality
	listCmd := newRootCmd()
	listCmd.SetArgs([]string{"--config", cfgPath, "list"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := listCmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	// Restore stdout and read output
	_ = w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify header is present
	assert.Contains(t, output, "PACKAGE NAME")
	assert.Contains(t, output, "VERSION")
	assert.Contains(t, output, "STATUS")

	// Verify all packages are listed
	assert.Contains(t, output, "packageA")
	assert.Contains(t, output, "packageB")
	assert.Contains(t, output, "packageC")
	assert.Contains(t, output, "1.0.0")
	assert.Contains(t, output, "2.0.0")
	assert.Contains(t, output, "1.5.0")

	// Verify status is "installed" for all
	assert.Contains(t, output, "installed")
}

func TestList_ListWithNameFiltering(t *testing.T) {
	tempDir := t.TempDir()

	// Build a repo with multiple artifacts and serve it
	defs := [][2]string{
		{"testapp", "1.0.0"},
		{"testlib", "2.0.0"},
		{"myapp", "1.5.0"},
		{"otherlib", "3.0.0"},
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

	// Install all packages
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "testapp", "testlib", "myapp", "otherlib"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Test filtering by name "test"
	listCmd := newRootCmd()
	listCmd.SetArgs([]string{"--config", cfgPath, "list", "--name", "test"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := listCmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	// Restore stdout and read output
	_ = w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Should only contain testapp and testlib, not myapp or otherlib
	assert.Contains(t, output, "testapp")
	assert.Contains(t, output, "testlib")
	assert.NotContains(t, output, "myapp")
	assert.NotContains(t, output, "otherlib")

	// Test filtering by name "app"
	listCmd2 := newRootCmd()
	listCmd2.SetArgs([]string{"--config", cfgPath, "list", "--name", "app"})

	// Capture stdout for second test
	r2, w2, _ := os.Pipe()
	os.Stdout = w2

	err2 := listCmd2.ExecuteContext(context.Background())
	require.NoError(t, err2)

	// Restore stdout and read output
	_ = w2.Close()
	os.Stdout = oldStdout

	var buf2 strings.Builder
	_, _ = io.Copy(&buf2, r2)
	output2 := buf2.String()

	// Should only contain testapp and myapp
	assert.Contains(t, output2, "testapp")
	assert.Contains(t, output2, "myapp")
	assert.NotContains(t, output2, "testlib")
	assert.NotContains(t, output2, "otherlib")
}

func TestList_ListEmptyDatabase(t *testing.T) {
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

	// List packages when database is empty
	listCmd := newRootCmd()
	listCmd.SetArgs([]string{"--config", cfgPath, "list"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := listCmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	// Restore stdout and read output
	_ = w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Should show "No packages installed"
	assert.Contains(t, output, "No packages installed")
}

func TestList_ListWithStatusIndicators(t *testing.T) {
	tempDir := t.TempDir()

	// Build a repo with one artifact and serve it
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

	// First sync to download the index
	syncCmd := newRootCmd()
	syncCmd.SetArgs([]string{"--config", cfgPath, "sync"})
	require.NoError(t, syncCmd.ExecuteContext(context.Background()))

	// Install a package
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "testapp"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// List packages - should show "installed" status
	listCmd := newRootCmd()
	listCmd.SetArgs([]string{"--config", cfgPath, "list"})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := listCmd.ExecuteContext(context.Background())
	require.NoError(t, err)

	// Restore stdout and read output
	_ = w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Should contain status indicators
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "installed")

	// For a more comprehensive test of missing status, we'd need to manipulate
	// the file system to make files missing, but that's complex for this test
	// The current test verifies that installed packages show "installed" status
}
