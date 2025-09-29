//go:build integration

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstall_BasicInstallFromRepository(t *testing.T) {
	tempDir := t.TempDir()
	// Build a repo with one artifact and serve it
	defs := [][2]string{{"testapp", "1.0.0"}}
	repoDir, _ := buildRepoDirWithArtifacts(t, tempDir, defs)
	srv, idxURL := startRepoServer(t, repoDir)
	defer srv.Close()

	// Debug: Read and print the generated index
	indexContent, _ := os.ReadFile(filepath.Join(repoDir, "index.json"))
	t.Logf("Generated index content: %s", string(indexContent))

	// Write a temporary config pointing to our served index.json
	cfgPath := filepath.Join(tempDir, "config.yaml")
	cacheDir := filepath.Join(tempDir, "cache")
	writeTempConfig(t, cfgPath, "testrepo", idxURL, cacheDir)

	// First sync to download the index
	syncCmd := newRootCmd()
	syncCmd.SetArgs([]string{"--config", cfgPath, "sync"})
	require.NoError(t, syncCmd.ExecuteContext(context.Background()))

	// Run install
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "testapp"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify the artifact was installed by using the list command
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	listCmd := newRootCmd()
	listCmd.SetArgs([]string{"--config", cfgPath, "list"})
	require.NoError(t, listCmd.ExecuteContext(context.Background()))

	// Restore stdout and read output
	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Check that the artifact is listed in the output
	require.Contains(t, output, "testapp", "testapp should be listed as installed")
	require.Contains(t, output, "1.0.0", "testapp version should be listed as 1.0.0")
}

func TestInstall_InstallWithDependencies(t *testing.T) {
	tempDir := t.TempDir()

	// Create dependency artifact first (testlib)
	depDefs := [][2]string{{"testlib", "1.0.0"}}
	depRepoDir, _ := buildRepoDirWithArtifacts(t, tempDir, depDefs)
	depSrv, depURL := startRepoServer(t, depRepoDir)
	defer depSrv.Close()

	// Create main app that depends on testlib
	appDefs := [][2]string{{"testapp", "1.0.0"}}
	appRepoDir, _ := buildRepoDirWithArtifactsWithDeps(t, tempDir, appDefs, "testlib:1.0.0")
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
  - name: dep-repo
    url: ` + depURL + `
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

	// Run install for the main app
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "testapp"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify both artifacts were installed by using the list command
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	listCmd := newRootCmd()
	listCmd.SetArgs([]string{"--config", cfgPath, "list"})
	require.NoError(t, listCmd.ExecuteContext(context.Background()))

	// Restore stdout and read output
	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Check that both artifacts are listed in the output
	require.Contains(t, output, "testapp", "testapp should be listed as installed")
	require.Contains(t, output, "testlib", "testlib dependency should be listed as installed")
	require.Contains(t, output, "1.0.0", "artifacts should have version 1.0.0")
}

func TestInstall_DryRunMode(t *testing.T) {
	tempDir := t.TempDir()
	// Build a repo with one artifact
	defs := [][2]string{{"testapp", "1.0.0"}}
	repoDir, _ := buildRepoDirWithArtifacts(t, tempDir, defs)
	srv, _ := startRepoServer(t, repoDir)
	defer srv.Close()

	// Write a temporary config
	cfgPath := filepath.Join(tempDir, "config.yaml")
	cacheDir := filepath.Join(tempDir, "cache")
	writeTempConfig(t, cfgPath, "testrepo", srv.URL+"/index.json", cacheDir)

	// First sync to download the index
	syncCmd := newRootCmd()
	syncCmd.SetArgs([]string{"--config", cfgPath, "sync"})
	require.NoError(t, syncCmd.ExecuteContext(context.Background()))

	// Run install with dry-run
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "--dry-run", "testapp"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify no artifacts were actually installed (dry run)
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	listCmd := newRootCmd()
	listCmd.SetArgs([]string{"--config", cfgPath, "list"})
	require.NoError(t, listCmd.ExecuteContext(context.Background()))

	// Restore stdout and read output
	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Check that no artifacts are listed (dry run should not install anything)
	assert.Contains(t, output, "No packages installed", "should show no packages installed in dry-run mode")
}

func TestInstall_WithCustomCacheDir(t *testing.T) {
	tempDir := t.TempDir()
	customCacheDir := filepath.Join(tempDir, "custom_cache")

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

	// Run install with custom cache directory
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "--cache-dir", customCacheDir, "testapp"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify the artifact was installed by using the list command
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	listCmd := newRootCmd()
	listCmd.SetArgs([]string{"--config", cfgPath, "list"})
	require.NoError(t, listCmd.ExecuteContext(context.Background()))

	// Restore stdout and read output
	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Check that the artifact is listed in the output
	require.Contains(t, output, "testapp", "testapp should be listed as installed")
	require.Contains(t, output, "1.0.0", "testapp version should be listed as 1.0.0")
}
