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

func TestCleanup_DryRun(t *testing.T) {
	tempDir := t.TempDir()

	// Build a repo with artifacts that have dependencies
	defs := [][2]string{
		{"mainapp", "1.0.0"},
		{"libA", "1.0.0"}, // Will be installed as dependency
		{"libB", "1.0.0"}, // Will be installed as dependency
	}
	deps := map[string][]string{
		"mainapp": {"libA", "libB"}, // mainapp depends on libA and libB
	}

	repoDir, _ := buildRepoDirWithArtifactsAndDeps(t, tempDir, defs, deps)
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

	// Install mainapp which will also install libA and libB as dependencies
	installCmd := newRootCmd()
	installCmd.SetArgs([]string{"--config", cfgPath, "install", "mainapp"})
	require.NoError(t, installCmd.ExecuteContext(context.Background()))

	// Verify all three packages are installed
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	require.Len(t, installed, 3, "should have 3 installed artifacts")

	// Uninstall mainapp, leaving libA and libB as orphaned dependencies
	uninstallCmd := newRootCmd()
	uninstallCmd.SetArgs([]string{"--config", cfgPath, "uninstall", "mainapp"})
	require.NoError(t, uninstallCmd.ExecuteContext(context.Background()))

	// Verify mainapp is gone but libA and libB are still there as orphaned dependencies
	installed = getInstalledArtifactsFromDB(t, cfgPath)
	require.Len(t, installed, 2, "should have 2 orphaned artifacts")

	// Check that they are automatic (dependency) installations
	var orphanedNames []string
	for _, artifact := range installed {
		orphanedNames = append(orphanedNames, artifact.Name)
	}
	assert.ElementsMatch(t, []string{"libA", "libB"}, orphanedNames)

	// Run cleanup with dry-run to see what would be cleaned up
	cleanupCmd := newRootCmd()
	cleanupCmd.SetArgs([]string{"--config", cfgPath, "cleanup", "--dry-run"})
	err := cleanupCmd.ExecuteContext(context.Background())
	require.NoError(t, err, "cleanup dry-run should not return an error")

	// Verify that libA and libB are still installed after dry-run
	installed = getInstalledArtifactsFromDB(t, cfgPath)
	require.Len(t, installed, 2, "dry-run should not remove any artifacts")
}

func TestCleanup_ActualCleanup(t *testing.T) {
	tempDir := t.TempDir()

	// Build a repo with artifacts that have dependencies
	defs := [][2]string{
		{"mainapp", "1.0.0"},
		{"libA", "1.0.0"}, // Will be installed as dependency
		{"libB", "1.0.0"}, // Will be installed as dependency
	}
	deps := map[string][]string{
		"mainapp": {"libA", "libB"}, // mainapp depends on libA and libB
	}

	repoDir, _ := buildRepoDirWithArtifactsAndDeps(t, tempDir, defs, deps)
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

	// Install mainapp which will also install libA and libB as dependencies
	installCmd := newRootCmd()
	installCmd.SetArgs([]string{"--config", cfgPath, "install", "mainapp"})
	require.NoError(t, installCmd.ExecuteContext(context.Background()))

	// Verify all three packages are installed
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	require.Len(t, installed, 3, "should have 3 installed artifacts")

	// Uninstall mainapp, leaving libA and libB as orphaned dependencies
	uninstallCmd := newRootCmd()
	uninstallCmd.SetArgs([]string{"--config", cfgPath, "uninstall", "mainapp"})
	require.NoError(t, uninstallCmd.ExecuteContext(context.Background()))

	// Verify mainapp is gone but libA and libB are still there as orphaned dependencies
	installed = getInstalledArtifactsFromDB(t, cfgPath)
	require.Len(t, installed, 2, "should have 2 orphaned artifacts")

	// Run actual cleanup to remove orphaned artifacts
	cleanupCmd := newRootCmd()
	cleanupCmd.SetArgs([]string{"--config", cfgPath, "cleanup"})
	err := cleanupCmd.ExecuteContext(context.Background())
	require.NoError(t, err, "cleanup should not return an error")

	// Verify that libA and libB have been cleaned up
	installed = getInstalledArtifactsFromDB(t, cfgPath)
	require.Len(t, installed, 0, "all orphaned artifacts should be cleaned up")
}

func TestCleanup_NoOrphanedArtifacts(t *testing.T) {
	tempDir := t.TempDir()

	// Build a simple repo with artifacts
	defs := [][2]string{
		{"packageA", "1.0.0"},
		{"packageB", "1.0.0"},
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

	// Install both packages as manual installations
	installCmd := newRootCmd()
	installCmd.SetArgs([]string{"--config", cfgPath, "install", "packageA", "packageB"})
	require.NoError(t, installCmd.ExecuteContext(context.Background()))

	// Verify both packages are installed
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	require.Len(t, installed, 2, "should have 2 installed artifacts")

	// Run cleanup - should find no orphaned artifacts
	cleanupCmd := newRootCmd()
	cleanupCmd.SetArgs([]string{"--config", cfgPath, "cleanup"})
	err := cleanupCmd.ExecuteContext(context.Background())
	require.NoError(t, err, "cleanup should not return an error when no orphaned artifacts exist")

	// Verify both packages are still installed
	installed = getInstalledArtifactsFromDB(t, cfgPath)
	require.Len(t, installed, 2, "manually installed packages should not be cleaned up")
}

func TestCleanup_MixedManualAndAutomatic(t *testing.T) {
	tempDir := t.TempDir()

	// Build a repo with artifacts that have dependencies
	defs := [][2]string{
		{"mainapp", "1.0.0"},
		{"libA", "1.0.0"},      // Will be installed as dependency
		{"manualLib", "1.0.0"}, // Will be installed manually
	}
	deps := map[string][]string{
		"mainapp": {"libA"}, // mainapp depends on libA
	}

	repoDir, _ := buildRepoDirWithArtifactsAndDeps(t, tempDir, defs, deps)
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

	// Install mainapp (which will also install libA as dependency) and manualLib manually
	installCmd := newRootCmd()
	installCmd.SetArgs([]string{"--config", cfgPath, "install", "mainapp", "manualLib"})
	require.NoError(t, installCmd.ExecuteContext(context.Background()))

	// Verify all three packages are installed
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	require.Len(t, installed, 3, "should have 3 installed artifacts")

	// Uninstall mainapp, leaving libA as orphaned dependency but manualLib should remain
	uninstallCmd := newRootCmd()
	uninstallCmd.SetArgs([]string{"--config", cfgPath, "uninstall", "mainapp"})
	require.NoError(t, uninstallCmd.ExecuteContext(context.Background()))

	// Verify manualLib is still there but libA is orphaned
	installed = getInstalledArtifactsFromDB(t, cfgPath)
	require.Len(t, installed, 2, "should have 2 artifacts: manualLib and orphaned libA")

	// Run cleanup - should only remove libA (the orphaned automatic artifact)
	cleanupCmd := newRootCmd()
	cleanupCmd.SetArgs([]string{"--config", cfgPath, "cleanup"})
	err := cleanupCmd.ExecuteContext(context.Background())
	require.NoError(t, err, "cleanup should not return an error")

	// Verify that only manualLib remains (libA was cleaned up)
	installed = getInstalledArtifactsFromDB(t, cfgPath)
	require.Len(t, installed, 1, "should have 1 artifact remaining")
	require.Equal(t, "manualLib", installed[0].Name, "manualLib should remain after cleanup")
}

func TestCleanup_ManualDependencyConversion(t *testing.T) {
	tempDir := t.TempDir()

	// Build a repo with artifacts that have dependencies
	defs := [][2]string{
		{"mainapp", "1.0.0"},
		{"libA", "1.0.0"}, // Will initially be automatic, then converted to manual
		{"libB", "1.0.0"}, // Will remain automatic and should be cleaned up
	}
	deps := map[string][]string{
		"mainapp": {"libA", "libB"}, // mainapp depends on both libA and libB
	}

	repoDir, _ := buildRepoDirWithArtifactsAndDeps(t, tempDir, defs, deps)
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

	// Install mainapp which will also install libA and libB as automatic dependencies
	installCmd := newRootCmd()
	installCmd.SetArgs([]string{"--config", cfgPath, "install", "mainapp"})
	require.NoError(t, installCmd.ExecuteContext(context.Background()))

	// Verify all three packages are installed
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	require.Len(t, installed, 3, "should have 3 installed artifacts")

	// Now manually install libA to convert it from automatic to manual
	installLibACmd := newRootCmd()
	installLibACmd.SetArgs([]string{"--config", cfgPath, "install", "libA"})
	require.NoError(t, installLibACmd.ExecuteContext(context.Background()))

	// Verify all three packages are still installed (libA is now manual)
	installed = getInstalledArtifactsFromDB(t, cfgPath)
	require.Len(t, installed, 3, "should still have 3 installed artifacts after manual install")

	// Uninstall mainapp - this should leave libA (now manual) and libB (orphaned automatic)
	uninstallCmd := newRootCmd()
	uninstallCmd.SetArgs([]string{"--config", cfgPath, "uninstall", "mainapp"})
	require.NoError(t, uninstallCmd.ExecuteContext(context.Background()))

	// Verify libA and libB are still installed (libA is manual, libB is orphaned automatic)
	installed = getInstalledArtifactsFromDB(t, cfgPath)
	require.Len(t, installed, 2, "should have 2 artifacts: manual libA and orphaned libB")

	var libANames, libBNames []string
	for _, artifact := range installed {
		if artifact.Name == "libA" {
			libANames = append(libANames, artifact.Name)
		}
		if artifact.Name == "libB" {
			libBNames = append(libBNames, artifact.Name)
		}
	}
	require.Len(t, libANames, 1, "libA should still be installed")
	require.Len(t, libBNames, 1, "libB should still be installed as orphaned automatic")

	// Run cleanup - should only remove libB (the orphaned automatic artifact)
	cleanupCmd := newRootCmd()
	cleanupCmd.SetArgs([]string{"--config", cfgPath, "cleanup"})
	err := cleanupCmd.ExecuteContext(context.Background())
	require.NoError(t, err, "cleanup should not return an error")

	// Verify that only libA remains (libB was cleaned up, libA was preserved as manual)
	installed = getInstalledArtifactsFromDB(t, cfgPath)
	require.Len(t, installed, 1, "should have 1 artifact remaining")
	require.Equal(t, "libA", installed[0].Name, "libA should remain after cleanup as it's manually installed")
}
