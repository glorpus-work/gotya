//go:build integration

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUninstall_BasicUninstall(t *testing.T) {
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

	// First install the package
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "testapp"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify the artifact was installed
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	var found bool
	for _, a := range installed {
		if a.Name == "testapp" && a.Version == "1.0.0" {
			found = true
			break
		}
	}
	require.True(t, found, "testapp@1.0.0 should be installed")

	// Now uninstall the package
	uninstallCmd := newRootCmd()
	uninstallCmd.SetArgs([]string{"--config", cfgPath, "uninstall", "testapp"})
	require.NoError(t, uninstallCmd.ExecuteContext(context.Background()))

	// Verify the artifact was uninstalled
	installed = getInstalledArtifactsFromDB(t, cfgPath)
	found = false
	for _, a := range installed {
		if a.Name == "testapp" {
			found = true
			break
		}
	}
	require.False(t, found, "testapp should be uninstalled")
}

func TestUninstall_WithPurgeFlag(t *testing.T) {
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

	// First install the package
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "testapp"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify the artifact was installed
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	var found bool
	for _, a := range installed {
		if a.Name == "testapp" && a.Version == "1.0.0" {
			found = true
			break
		}
	}
	require.True(t, found, "testapp@1.0.0 should be installed")

	// Now uninstall the package with --purge flag
	uninstallCmd := newRootCmd()
	uninstallCmd.SetArgs([]string{"--config", cfgPath, "uninstall", "--purge", "testapp"})
	require.NoError(t, uninstallCmd.ExecuteContext(context.Background()))

	// Verify the artifact was uninstalled
	installed = getInstalledArtifactsFromDB(t, cfgPath)
	found = false
	for _, a := range installed {
		if a.Name == "testapp" {
			found = true
			break
		}
	}
	require.False(t, found, "testapp should be uninstalled with purge")
}

func TestUninstall_UninstallNonExistentPackage(t *testing.T) {
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

	// Try to uninstall a non-existent package
	uninstallCmd := newRootCmd()
	uninstallCmd.SetArgs([]string{"--config", cfgPath, "uninstall", "nonexistent"})
	err := uninstallCmd.ExecuteContext(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not installed")
}

func TestUninstall_UninstallWithDependencies(t *testing.T) {
	tempDir := t.TempDir()

	// Build repos with dependencies
	// Create dependency artifact first (testlib)
	depDefs := [][2]string{{"testlib", "1.0.0"}}
	depRepoDir, _ := buildRepoDirWithArtifacts(t, tempDir, depDefs)
	depSrv, depURL := startRepoServer(t, depRepoDir)
	defer depSrv.Close()

	// Create main app that depends on testlib
	appDefs := [][2]string{{"testapp", "1.0.0"}}
	deps := map[string][]string{
		"testapp": {"testlib:1.0.0"},
	}
	appRepoDir, _ := buildRepoDirWithArtifactsAndDeps(t, tempDir, appDefs, deps)
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

	// Install the main app (which should also install the dependency)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "testapp"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify both artifacts were installed
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	var hasApp, hasLib bool
	for _, a := range installed {
		if a.Name == "testapp" && a.Version == "1.0.0" {
			hasApp = true
		}
		if a.Name == "testlib" && a.Version == "1.0.0" {
			hasLib = true
		}
	}
	require.True(t, hasApp, "testapp@1.0.0 should be installed")
	require.True(t, hasLib, "testlib@1.0.0 should be installed")

	// Now uninstall just the main app
	uninstallCmd := newRootCmd()
	uninstallCmd.SetArgs([]string{"--config", cfgPath, "uninstall", "testapp"})
	require.NoError(t, uninstallCmd.ExecuteContext(context.Background()))

	// Verify only the dependency remains (should not be uninstalled automatically)
	installed = getInstalledArtifactsFromDB(t, cfgPath)
	hasApp = false
	hasLib = false
	for _, a := range installed {
		if a.Name == "testapp" {
			hasApp = true
		}
		if a.Name == "testlib" {
			hasLib = true
		}
	}
	require.False(t, hasApp, "testapp should be uninstalled")
	require.True(t, hasLib, "testlib dependency should remain installed")
}

func TestUninstall_UninstallWithMissingFiles(t *testing.T) {
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

	// First install the package
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "testapp"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify the artifact was installed
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	var found bool
	for _, a := range installed {
		if a.Name == "testapp" && a.Version == "1.0.0" {
			found = true
			break
		}
	}
	require.True(t, found, "testapp@1.0.0 should be installed")

	// Simulate missing files by removing the install directory
	require.NoError(t, os.RemoveAll(installDir))

	// Now try to uninstall - should handle missing files gracefully
	uninstallCmd := newRootCmd()
	uninstallCmd.SetArgs([]string{"--config", cfgPath, "uninstall", "testapp"})
	require.NoError(t, uninstallCmd.ExecuteContext(context.Background()))

	// Verify the artifact was removed from DB even though files were missing
	installed = getInstalledArtifactsFromDB(t, cfgPath)
	found = false
	for _, a := range installed {
		if a.Name == "testapp" {
			found = true
			break
		}
	}
	require.False(t, found, "testapp should be uninstalled even with missing files")
}
