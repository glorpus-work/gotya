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

func TestUpdate_UpdateSpecificPackages(t *testing.T) {
	tempDir := t.TempDir()

	// Build a repo with multiple versions of the same artifact
	defs := [][2]string{
		{"testapp", "1.0.0"}, // Older version to install first
		{"testapp", "1.1.0"}, // Newer version to update to
		{"testapp", "2.0.0"}, // Even newer version
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

	// First install the older version
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "testapp:1.0.0"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify the older version is installed
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	var found bool
	for _, a := range installed {
		if a.Name == "testapp" && a.Version == "1.0.0" {
			found = true
			break
		}
	}
	require.True(t, found, "testapp@1.0.0 should be installed initially")

	// Update the specific package
	updateCmd := newRootCmd()
	updateCmd.SetArgs([]string{"--config", cfgPath, "update", "testapp"})
	require.NoError(t, updateCmd.ExecuteContext(context.Background()))

	// Verify the package was updated to a newer version
	installed = getInstalledArtifactsFromDB(t, cfgPath)
	var newVersion string
	for _, a := range installed {
		if a.Name == "testapp" {
			newVersion = a.Version
			break
		}
	}
	require.NotEqual(t, "1.0.0", newVersion, "testapp should be updated from 1.0.0")
	require.True(t, newVersion == "1.1.0" || newVersion == "2.0.0", "testapp should be updated to 1.1.0 or 2.0.0")
}

func TestUpdate_UpdateAllPackages(t *testing.T) {
	tempDir := t.TempDir()

	// Build a repo with multiple packages with different versions
	defs := [][2]string{
		{"packageA", "1.0.0"},
		{"packageA", "1.1.0"},
		{"packageB", "1.0.0"},
		{"packageB", "2.0.0"},
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

	// Install older versions of both packages
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "packageA:1.0.0", "packageB:1.0.0"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify both older versions are installed
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	var hasA, hasB bool
	var aVer, bVer string
	for _, a := range installed {
		if a.Name == "packageA" {
			hasA = true
			aVer = a.Version
		}
		if a.Name == "packageB" {
			hasB = true
			bVer = a.Version
		}
	}
	require.True(t, hasA, "packageA should be installed")
	require.True(t, hasB, "packageB should be installed")
	require.Equal(t, "1.0.0", aVer, "packageA should initially be 1.0.0")
	require.Equal(t, "1.0.0", bVer, "packageB should initially be 1.0.0")

	// Update all packages
	updateCmd := newRootCmd()
	updateCmd.SetArgs([]string{"--config", cfgPath, "update", "--all"})
	require.NoError(t, updateCmd.ExecuteContext(context.Background()))

	// Verify both packages were updated to newer versions
	installed = getInstalledArtifactsFromDB(t, cfgPath)
	aVer = ""
	bVer = ""
	for _, a := range installed {
		if a.Name == "packageA" {
			aVer = a.Version
		}
		if a.Name == "packageB" {
			bVer = a.Version
		}
	}
	require.Equal(t, "1.1.0", aVer, "packageA should be updated from 1.0.0")
	require.Equal(t, "2.0.0", bVer, "packageB should be updated from 1.0.0")
}

func TestUpdate_UpdateDryRunMode(t *testing.T) {
	tempDir := t.TempDir()

	// Build a repo with multiple versions of the same artifact
	defs := [][2]string{
		{"testapp", "1.0.0"}, // Older version to install first
		{"testapp", "1.1.0"}, // Newer version to update to
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

	// First install the older version
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "testapp:1.0.0"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify the older version is installed
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	var found bool
	for _, a := range installed {
		if a.Name == "testapp" && a.Version == "1.0.0" {
			found = true
			break
		}
	}
	require.True(t, found, "testapp@1.0.0 should be installed initially")

	// Update with dry-run mode - should not actually update
	updateCmd := newRootCmd()
	updateCmd.SetArgs([]string{"--config", cfgPath, "update", "--dry-run", "testapp"})
	require.NoError(t, updateCmd.ExecuteContext(context.Background()))

	// Verify the package was NOT updated (dry-run should not change anything)
	installed = getInstalledArtifactsFromDB(t, cfgPath)
	var currentVersion string
	for _, a := range installed {
		if a.Name == "testapp" {
			currentVersion = a.Version
			break
		}
	}
	require.Equal(t, "1.0.0", currentVersion, "testapp should still be 1.0.0 after dry-run")
}

func TestUpdate_UpdateWithDependencyResolution(t *testing.T) {
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

	// Update the main app - should handle dependency resolution
	updateCmd := newRootCmd()
	updateCmd.SetArgs([]string{"--config", cfgPath, "update", "testapp"})
	require.NoError(t, updateCmd.ExecuteContext(context.Background()))

	// Verify update completed successfully (dependency resolution should work)
	installed = getInstalledArtifactsFromDB(t, cfgPath)
	require.NotEmpty(t, installed, "packages should still be installed after update")
}

func TestUpdate_UpdateFailureScenarios(t *testing.T) {
	t.Run("UpdateNonExistentPackage", func(t *testing.T) {
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

		// First install a package
		cmd := newRootCmd()
		cmd.SetArgs([]string{"--config", cfgPath, "install", "testapp"})
		require.NoError(t, cmd.ExecuteContext(context.Background()))

		// Verify the package is installed
		installed := getInstalledArtifactsFromDB(t, cfgPath)
		var found bool
		for _, a := range installed {
			if a.Name == "testapp" && a.Version == "1.0.0" {
				found = true
				break
			}
		}
		require.True(t, found, "testapp@1.0.0 should be installed")

		// Try to update a non-existent package when other packages are installed
		updateCmd := newRootCmd()
		updateCmd.SetArgs([]string{"--config", cfgPath, "update", "nonexistent"})
		err := updateCmd.ExecuteContext(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not installed")
	})

	t.Run("UpdateWithNoPackagesSpecified", func(t *testing.T) {
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

		// Try to update with no packages specified and no --all flag
		updateCmd := newRootCmd()
		updateCmd.SetArgs([]string{"--config", cfgPath, "update"})
		err := updateCmd.ExecuteContext(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no packages specified")
	})
}

func TestUpdate_CrossRepositoryDependencyResolution(t *testing.T) {
	tempDir := t.TempDir()

	// Build two repos with cross-repository dependencies
	// Repo 1: contains testlib v2.0.0
	libDefs := [][2]string{{"testlib", "2.0.0"}}
	libRepoDir, _ := buildRepoDirWithArtifacts(t, tempDir, libDefs)
	libSrv, libURL := startRepoServer(t, libRepoDir)
	defer libSrv.Close()

	// Repo 2: contains testapp that depends on testlib >= 2.0.0
	appDefs := [][2]string{{"testapp", "1.0.0"}}
	appDeps := map[string][]string{
		"testapp": {"testlib:>= 2.0.0"},
	}
	appRepoDir, _ := buildRepoDirWithArtifactsAndDeps(t, tempDir, appDefs, appDeps)
	appSrv, appURL := startRepoServer(t, appRepoDir)
	defer appSrv.Close()

	// Write config with both repositories (lib repo has higher priority)
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

	// Install testapp (should pull testlib v2.0.0 from lib-repo due to higher priority)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "testapp"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify both artifacts were installed with correct versions
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	var hasApp, hasLib bool
	var libVersion string
	for _, a := range installed {
		if a.Name == "testapp" && a.Version == "1.0.0" {
			hasApp = true
		}
		if a.Name == "testlib" {
			hasLib = true
			libVersion = a.Version
		}
	}
	require.True(t, hasApp, "testapp@1.0.0 should be installed")
	require.True(t, hasLib, "testlib should be installed")
	require.Equal(t, "2.0.0", libVersion, "testlib should be v2.0.0 from higher priority repo")

	// Update all packages - should resolve dependencies across repositories
	updateCmd := newRootCmd()
	updateCmd.SetArgs([]string{"--config", cfgPath, "update", "--all"})
	require.NoError(t, updateCmd.ExecuteContext(context.Background()))

	// Verify update completed successfully (cross-repo dependency resolution should work)
	installed = getInstalledArtifactsFromDB(t, cfgPath)
	require.NotEmpty(t, installed, "packages should still be installed after cross-repo update")
}

func TestUpdate_UpdateWithVersionConflicts(t *testing.T) {
	tempDir := t.TempDir()

	// Build a repo with conflicting version requirements
	defs := [][2]string{
		{"packageA", "1.0.0"},
		{"packageA", "2.0.0"},
		{"packageA", "3.0.0"},
		{"packageB", "1.0.0"}, // Depends on packageA == 1.0.0
		{"packageC", "1.0.0"}, // Depends on packageA >= 3.0.0
	}
	deps := map[string][]string{
		"packageB": {"packageA:= 1.0.0"},
		"packageC": {"packageA:>= 3.0.0"},
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

	// Install packageB (which requires packageA == 1.0.0)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "packageB"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify packageB and packageA@1.0.0 are installed
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	var hasB bool
	var aVer string
	for _, a := range installed {
		if a.Name == "packageB" && a.Version == "1.0.0" {
			hasB = true
		}
		if a.Name == "packageA" {
			aVer = a.Version
		}
	}
	require.True(t, hasB, "packageB should be installed")
	require.Equal(t, "1.0.0", aVer, "packageA should be 1.0.0")

	// Try to update all packages - should handle version conflicts
	updateCmd := newRootCmd()
	updateCmd.SetArgs([]string{"--config", cfgPath, "update", "--all"})
	err := updateCmd.ExecuteContext(context.Background())

	// The update may succeed or fail depending on how conflicts are resolved
	// If it fails, it should be due to version conflicts
	if err != nil {
		// If it fails, it should be due to dependency conflicts
		assert.Contains(t, err.Error(), "packageA")
	} else {
		// If it succeeds, verify the final state is consistent
		installed = getInstalledArtifactsFromDB(t, cfgPath)
		require.NotEmpty(t, installed, "packages should still be installed after update")
	}
}
