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

// TestInstall_WithPreInstallHook verifies that pre-install hooks are executed during installation
func TestInstall_WithPreInstallHook(t *testing.T) {
	tempDir := t.TempDir()

	// Create artifact source with a pre-install hook
	src := createArtifactSourceWithHook(t, tempDir, "pre-install", `
// Pre-install hook - creates a marker file
os := import("os")
dirs := import("dirs")

marker_path := dirs.temp_meta_dir + "/pre-install-executed.txt"
file := os.create(marker_path)
file.write_string("pre-install hook executed")
file.close()
`)

	// Build repo with the artifact
	repoDir := filepath.Join(tempDir, "repo")
	artifactsDir := filepath.Join(repoDir, "artifacts")
	require.NoError(t, os.MkdirAll(artifactsDir, 0o755))

	path := createArtifactViaCLI(t, src, "testapp", "1.0.0", artifactsDir, nil, []string{"pre-install=pre-install.tengo"})
	require.NotEmpty(t, path)

	// Generate index
	outFile := filepath.Join(repoDir, "index.json")
	generateIndexViaCLI(t, artifactsDir, outFile, "artifacts", true)

	// Start server
	srv, idxURL := startRepoServer(t, repoDir)
	defer srv.Close()

	// Write config
	cfgPath := filepath.Join(tempDir, "config.yaml")
	cacheDir := filepath.Join(tempDir, "cache")
	writeTempConfig(t, cfgPath, "testrepo", idxURL, cacheDir)

	// Sync and install
	syncCmd := newRootCmd()
	syncCmd.SetArgs([]string{"--config", cfgPath, "sync"})
	require.NoError(t, syncCmd.ExecuteContext(context.Background()))

	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "testapp"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify the hook was executed by checking for the marker file
	metaDir := filepath.Join(tempDir, "meta", "testapp")
	markerFile := filepath.Join(metaDir, "pre-install-executed.txt")
	assert.FileExists(t, markerFile, "pre-install hook should have created marker file")

	content, err := os.ReadFile(markerFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "pre-install hook executed")
}

// TestInstall_WithPostInstallHook verifies that post-install hooks are executed after installation
func TestInstall_WithPostInstallHook(t *testing.T) {
	tempDir := t.TempDir()

	// Create artifact source with a post-install hook
	src := createArtifactSourceWithHook(t, tempDir, "post-install", `
// Post-install hook - creates a marker file
os := import("os")
dirs := import("dirs")

marker_path := dirs.meta_dir + "/post-install-executed.txt"

file := os.create(marker_path)
file.write_string("post-install hook executed")
file.close()
`)

	// Build repo with the artifact
	repoDir := filepath.Join(tempDir, "repo")
	artifactsDir := filepath.Join(repoDir, "artifacts")
	require.NoError(t, os.MkdirAll(artifactsDir, 0o755))

	path := createArtifactViaCLI(t, src, "testapp", "1.0.0", artifactsDir, nil, []string{"post-install=post-install.tengo"})
	require.NotEmpty(t, path)

	// Generate index
	outFile := filepath.Join(repoDir, "index.json")
	generateIndexViaCLI(t, artifactsDir, outFile, "artifacts", true)

	// Start server
	srv, idxURL := startRepoServer(t, repoDir)
	defer srv.Close()

	// Write config
	cfgPath := filepath.Join(tempDir, "config.yaml")
	cacheDir := filepath.Join(tempDir, "cache")
	writeTempConfig(t, cfgPath, "testrepo", idxURL, cacheDir)

	// Sync and install
	syncCmd := newRootCmd()
	syncCmd.SetArgs([]string{"--config", cfgPath, "sync"})
	require.NoError(t, syncCmd.ExecuteContext(context.Background()))

	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "testapp"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify the hook was executed by checking for the marker file
	metaDir := filepath.Join(tempDir, "meta", "testapp")
	markerFile := filepath.Join(metaDir, "post-install-executed.txt")
	assert.FileExists(t, markerFile, "post-install hook should have created marker file")

	content, err := os.ReadFile(markerFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "post-install hook executed")
}

// TestUpdate_WithUpdateHooks verifies that update hooks are executed during updates
func TestUpdate_WithUpdateHooks(t *testing.T) {
	tempDir := t.TempDir()

	scriptOut := filepath.Join(tempDir, "script_out")
	t.Setenv("SCRIPT_OUT_DIR", scriptOut)
	require.NoError(t, os.MkdirAll(scriptOut, 0o755))

	// Create v1.0.0 with pre-update hook
	srcV1 := createArtifactSourceWithHook(t, tempDir, "pre-update", `
// Pre-update hook - creates a marker file
os := import("os")
context := import("context")

marker_path := os.getenv("SCRIPT_OUT_DIR") + "/pre-update-executed.txt"

file := os.create(marker_path)
file.write_string("pre-update hook executed for version " + context.old_version)
file.close()
`)

	// Create v2.0.0 with post-update hook
	srcV2 := createArtifactSourceWithHook(t, tempDir+"_v2", "post-update", `
// Post-update hook - creates a marker file
os := import("os")
context := import("context")

marker_path := os.getenv("SCRIPT_OUT_DIR") + "/post-update-executed.txt"

file := os.create(marker_path)
file.write_string("pre-update hook executed for version " + context.old_version)
file.close()
`)

	// Build repo with both versions
	repoDir := filepath.Join(tempDir, "repo")
	artifactsDir := filepath.Join(repoDir, "artifacts")
	require.NoError(t, os.MkdirAll(artifactsDir, 0o755))

	pathV1 := createArtifactViaCLI(t, srcV1, "testapp", "1.0.0", artifactsDir, nil, []string{"pre-update=pre-update.tengo"})
	require.NotEmpty(t, pathV1)

	pathV2 := createArtifactViaCLI(t, srcV2, "testapp", "2.0.0", artifactsDir, nil, []string{"post-update=post-update.tengo"})
	require.NotEmpty(t, pathV2)

	// Generate index
	outFile := filepath.Join(repoDir, "index.json")
	generateIndexViaCLI(t, artifactsDir, outFile, "artifacts", true)

	// Start server
	srv, idxURL := startRepoServer(t, repoDir)
	defer srv.Close()

	// Write config
	cfgPath := filepath.Join(tempDir, "config.yaml")
	cacheDir := filepath.Join(tempDir, "cache")
	writeTempConfig(t, cfgPath, "testrepo", idxURL, cacheDir)

	// Sync
	syncCmd := newRootCmd()
	syncCmd.SetArgs([]string{"--config", cfgPath, "sync"})
	require.NoError(t, syncCmd.ExecuteContext(context.Background()))

	// Install v1.0.0
	installCmd := newRootCmd()
	installCmd.SetArgs([]string{"--config", cfgPath, "install", "testapp:1.0.0"})
	require.NoError(t, installCmd.ExecuteContext(context.Background()))

	// Update to v2.0.0
	updateCmd := newRootCmd()
	updateCmd.SetArgs([]string{"--config", cfgPath, "update", "testapp"})
	require.NoError(t, updateCmd.ExecuteContext(context.Background()))

	preUpdateMarker := filepath.Join(scriptOut, "pre-update-executed.txt")
	assert.FileExists(t, preUpdateMarker, "pre-update hook should have created marker file")

	// Verify the post-update hook was executed
	postUpdateMarker := filepath.Join(scriptOut, "post-update-executed.txt")
	assert.FileExists(t, postUpdateMarker, "post-update hook should have created marker file")
}

// TestUninstall_WithUninstallHooks verifies that uninstall hooks are executed during uninstallation
func TestUninstall_WithUninstallHooks(t *testing.T) {
	tempDir := t.TempDir()

	scriptOut := filepath.Join(tempDir, "script_out")
	t.Setenv("SCRIPT_OUT_DIR", scriptOut)
	require.NoError(t, os.MkdirAll(scriptOut, 0o755))

	// Create artifact source with pre-uninstall hook that creates a marker in a persistent location
	src := createArtifactSourceWithHook(t, tempDir, "pre-uninstall", `
// Pre-uninstall hook - creates a marker file in temp directory
os := import("os")
context := import("context")

marker_path := os.getenv("SCRIPT_OUT_DIR") + "/pre-uninstall-executed.txt"

file := os.create(marker_path)
file.write_string("pre-uninstall hook executed")
file.close()
`)

	// Build repo with the artifact
	repoDir := filepath.Join(tempDir, "repo")
	artifactsDir := filepath.Join(repoDir, "artifacts")
	require.NoError(t, os.MkdirAll(artifactsDir, 0o755))

	path := createArtifactViaCLI(t, src, "testapp", "1.0.0", artifactsDir, nil, []string{"pre-uninstall=pre-uninstall.tengo"})
	require.NotEmpty(t, path)

	// Generate index
	outFile := filepath.Join(repoDir, "index.json")
	generateIndexViaCLI(t, artifactsDir, outFile, "artifacts", true)

	// Start server
	srv, idxURL := startRepoServer(t, repoDir)
	defer srv.Close()

	// Write config
	cfgPath := filepath.Join(tempDir, "config.yaml")
	cacheDir := filepath.Join(tempDir, "cache")
	writeTempConfig(t, cfgPath, "testrepo", idxURL, cacheDir)

	// Sync and install
	syncCmd := newRootCmd()
	syncCmd.SetArgs([]string{"--config", cfgPath, "sync"})
	require.NoError(t, syncCmd.ExecuteContext(context.Background()))

	installCmd := newRootCmd()
	installCmd.SetArgs([]string{"--config", cfgPath, "install", "testapp"})
	require.NoError(t, installCmd.ExecuteContext(context.Background()))

	// Uninstall
	uninstallCmd := newRootCmd()
	uninstallCmd.SetArgs([]string{"--config", cfgPath, "uninstall", "testapp", "--purge"})
	require.NoError(t, uninstallCmd.ExecuteContext(context.Background()))

	// Verify the pre-uninstall hook was executed
	markerFile := filepath.Join(scriptOut, "pre-uninstall-executed.txt")
	defer os.Remove(markerFile) // Clean up
	assert.FileExists(t, markerFile, "pre-uninstall hook should have created marker file")

	content, err := os.ReadFile(markerFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "pre-uninstall hook executed")
}

// TestInstall_FailingPreInstallHook verifies that a failing pre-install hook prevents installation
func TestInstall_FailingPreInstallHook(t *testing.T) {
	tempDir := t.TempDir()

	// Create artifact source with a failing pre-install hook
	src := createArtifactSourceWithHook(t, tempDir, "pre-install", `
// Failing pre-install hook
ohno
`)

	// Build repo with the artifact
	repoDir := filepath.Join(tempDir, "repo")
	artifactsDir := filepath.Join(repoDir, "artifacts")
	require.NoError(t, os.MkdirAll(artifactsDir, 0o755))

	path := createArtifactViaCLI(t, src, "testapp", "1.0.0", artifactsDir, nil, []string{"pre-install=pre-install.tengo"})
	require.NotEmpty(t, path)

	// Generate index
	outFile := filepath.Join(repoDir, "index.json")
	generateIndexViaCLI(t, artifactsDir, outFile, "artifacts", true)

	// Start server
	srv, idxURL := startRepoServer(t, repoDir)
	defer srv.Close()

	// Write config
	cfgPath := filepath.Join(tempDir, "config.yaml")
	cacheDir := filepath.Join(tempDir, "cache")
	writeTempConfig(t, cfgPath, "testrepo", idxURL, cacheDir)

	// Sync
	syncCmd := newRootCmd()
	syncCmd.SetArgs([]string{"--config", cfgPath, "sync"})
	require.NoError(t, syncCmd.ExecuteContext(context.Background()))

	// Install should fail due to failing hook
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "install", "testapp"})
	err := cmd.ExecuteContext(context.Background())
	require.Error(t, err, "install should fail when pre-install hook fails")

	// Verify the artifact was NOT installed
	installed := getInstalledArtifactsFromDB(t, cfgPath)
	for _, a := range installed {
		assert.NotEqual(t, "testapp", a.Name, "testapp should not be installed after hook failure")
	}
}

// createArtifactSourceWithHook creates an artifact source directory with a hook script
func createArtifactSourceWithHook(t *testing.T, root, hookName, hookScript string) string {
	t.Helper()
	src := filepath.Join(root, "src")
	require.NoError(t, os.MkdirAll(filepath.Join(src, "meta"), 0o755))

	// Write the hook script
	hookPath := filepath.Join(src, "meta", hookName+".tengo")
	require.NoError(t, os.WriteFile(hookPath, []byte(hookScript), 0o644))

	return src
}
