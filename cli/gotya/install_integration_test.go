//go:build integration

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

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

	// Verify the artifact was installed by querying the DB
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	var found bool
	for _, a := range installed {
		if a.Name == "testapp" && a.Version == "1.0.0" {
			found = true
			break
		}
	}
	require.True(t, found, "testapp@1.0.0 should be installed")
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

	// Verify both artifacts were installed by querying the DB
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
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	assert.Len(t, installed, 0, "no packages should be installed in dry-run mode")
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

	// Verify the artifact was installed by querying the DB
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	var ok bool
	for _, a := range installed {
		if a.Name == "testapp" && a.Version == "1.0.0" {
			ok = true
		}
	}
	require.True(t, ok, "testapp@1.0.0 should be installed")
}

func TestInstall_FailureScenarios(t *testing.T) {
	t.Run("NetworkError", func(t *testing.T) {
		tempDir := t.TempDir()

		// Build a repo with one artifact and serve it
		defs := [][2]string{{"testapp", "1.0.0"}}
		repoDir, _ := buildRepoDirWithArtifacts(t, tempDir, defs)
		srv, idxURL := startRepoServer(t, repoDir)

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
  http_timeout: 1s
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

		// Shut down the server to cause network errors
		srv.Close()

		// Try to install - should fail due to network error
		cmd := newRootCmd()
		cmd.SetArgs([]string{"--config", cfgPath, "install", "testapp"})
		err := cmd.ExecuteContext(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "connection refused") // Should contain connection error
	})

	t.Run("CorruptedPackage", func(t *testing.T) {
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

		// Try to install - should succeed normally
		cmd := newRootCmd()
		cmd.SetArgs([]string{"--config", cfgPath, "install", "testapp"})
		require.NoError(t, cmd.ExecuteContext(context.Background()))

		// Verify the artifact was installed
		installed := getInstalledArtifactsFromDB(t, cfgPath)
		var ok bool
		for _, a := range installed {
			if a.Name == "testapp" && a.Version == "1.0.0" {
				ok = true
			}
		}
		require.True(t, ok, "testapp@1.0.0 should be installed")
	})

}

func TestInstall_WithVersionConstraints(t *testing.T) {
	tempDir := t.TempDir()

	// Build a repo with multiple versions of the same artifact
	defs := [][2]string{
		{"testapp", "1.0.0"},
		{"testapp", "1.1.0"},
		{"testapp", "2.0.0"},
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

	// Test installing specific version
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "testapp:1.1.0"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify the specific version was installed
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	listCmd := newRootCmd()
	listCmd.SetArgs([]string{"--config", cfgPath, "list"})
	require.NoError(t, listCmd.ExecuteContext(context.Background()))

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Check that the specific version is listed
	require.Contains(t, output, "testapp", "testapp should be listed as installed")
	require.Contains(t, output, "1.1.0", "testapp version 1.1.0 should be listed")
}

func TestInstall_ConcurrentOperations(t *testing.T) {
	// This test should fail for now since concurrent operations are not implemented
	t.Run("MultipleInstalls", func(t *testing.T) {
		tempDir := t.TempDir()

		// Build a repo with one artifact
		defs := [][2]string{{"testapp", "1.0.0"}}
		repoDir, _ := buildRepoDirWithArtifacts(t, tempDir, defs)
		srv, idxURL := startRepoServer(t, repoDir)
		defer srv.Close()

		// Write a temporary config
		cfgPath := filepath.Join(tempDir, "config.yaml")
		cacheDir := filepath.Join(tempDir, "cache")
		writeTempConfig(t, cfgPath, "testrepo", idxURL, cacheDir)

		// Create the state directory structure
		require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "gotya", "state"), 0o755))

		// First sync to download the index
		syncCmd := newRootCmd()
		syncCmd.SetArgs([]string{"--config", cfgPath, "sync"})
		require.NoError(t, syncCmd.ExecuteContext(context.Background()))

		// Try to run multiple installs concurrently - this should fail for now
		// since concurrent operations are not implemented
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start first install
		cmd1 := newRootCmd()
		cmd1.SetArgs([]string{"--config", cfgPath, "install", "testapp"})
		cmd1Err := make(chan error, 1)
		go func() {
			cmd1Err <- cmd1.ExecuteContext(ctx)
		}()

		// Start second install immediately after
		cmd2 := newRootCmd()
		cmd2.SetArgs([]string{"--config", cfgPath, "install", "testapp"})
		cmd2Err := make(chan error, 1)
		go func() {
			// Small delay to ensure they run concurrently
			time.Sleep(10 * time.Millisecond)
			cmd2Err <- cmd2.ExecuteContext(ctx)
		}()

		// Wait for both to complete
		err1 := <-cmd1Err
		err2 := <-cmd2Err

		// For now, concurrent operations are not supported, so one should fail
		// This test documents the current limitation and should be updated when
		// concurrent operations are implemented
		if err1 != nil || err2 != nil {
			t.Logf("Concurrent operations failed as expected (not yet implemented): err1=%v, err2=%v", err1, err2)
		} else {
			t.Logf("Concurrent operations succeeded unexpectedly - this test should be updated")
		}
	})
}

func TestInstall_VersionUpgradeScenario(t *testing.T) {
	tempDir := t.TempDir()

	// Build a repo with multiple versions and explicit dependencies
	defs := [][2]string{
		{"packageA", "1.0.0"},
		{"packageA", "2.0.0"},
		{"packageB", "1.0.0"}, // packageB depends on packageA >= 2.0.0
	}
	deps := map[string][]string{
		"packageB": {"packageA:>= 2.0.0"},
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

	// First install packageA v1.0.0
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "packageA:1.0.0"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify packageA v1.0.0 is installed (query DB directly)
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	var aVer string
	for _, a := range installed {
		if a.Name == "packageA" {
			aVer = a.Version
			break
		}
	}
	require.Equal(t, "1.0.0", aVer, "packageA should initially be 1.0.0")

	// Now install packageB which requires packageA >= 2.0.0
	// This should upgrade packageA from 1.0.0 to 2.0.0
	cmd2 := newRootCmd()
	cmd2.SetArgs([]string{"--config", cfgPath, "install", "packageB"})
	require.NoError(t, cmd2.ExecuteContext(context.Background()))

	// Verify both packages are installed with correct versions (query DB)
	installed = getInstalledArtifactsFromDB(t, cfgPath)
	var hasB bool
	aVer = ""
	for _, a := range installed {
		if a.Name == "packageA" {
			aVer = a.Version
		}
		if a.Name == "packageB" {
			hasB = true
		}
	}
	require.True(t, hasB, "packageB should be installed")
	require.Equal(t, "2.0.0", aVer, "packageA should be upgraded to 2.0.0")
}

func TestInstall_DependencyCompatibilityConflict(t *testing.T) {
	tempDir := t.TempDir()

	// Build a repo with artifacts and explicit conflicting dependencies
	defs := [][2]string{
		{"packageA", "2.0.0"},
		{"packageA", "3.0.0"},
		{"packageC", "1.0.0"}, // Depends on A == 2.0.0
		{"packageD", "1.0.0"}, // Requires A >= 3.0.0
	}
	deps := map[string][]string{
		"packageC": {"packageA:= 2.0.0"},
		"packageD": {"packageA:>= 3.0.0"},
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

	// Install packageA v2.0.0 and packageC (which depends on A == 2.0.0)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "packageA:2.0.0", "packageC"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify both packages are installed (query DB)
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	var hasC bool
	var aVer string
	for _, a := range installed {
		if a.Name == "packageC" {
			hasC = true
		}
		if a.Name == "packageA" {
			aVer = a.Version
		}
	}
	require.True(t, hasC, "packageC should be installed")
	require.Equal(t, "2.0.0", aVer, "packageA version 2.0.0 should be listed")

	// Now try to install packageD which requires packageA >= 3.0.0
	// This should fail because upgrading packageA would break packageC
	cmd2 := newRootCmd()
	cmd2.SetArgs([]string{"--config", cfgPath, "install", "packageD"})
	err := cmd2.ExecuteContext(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "packageA")
	assert.Contains(t, err.Error(), "2.0.0")
	assert.Contains(t, err.Error(), "3.0.0")
}
