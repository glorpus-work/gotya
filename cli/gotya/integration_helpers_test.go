//go:build integration

package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cperrin88/gotya/pkg/artifact/database"
	"github.com/cperrin88/gotya/pkg/config"
	"github.com/cperrin88/gotya/pkg/model"
	"github.com/stretchr/testify/require"
)

// buildRepoDirWithArtifacts creates a temporary repo directory structure containing artifacts and an index.json.
// Returns repoDir and the list of created artifact file paths.
func buildRepoDirWithArtifacts(t *testing.T, root string, artifactDefs [][2]string) (string, []string) {
	t.Helper()
	repoDir := filepath.Join(root, "repo")
	artifactsDir := filepath.Join(repoDir, "artifacts")
	require.NoError(t, os.MkdirAll(artifactsDir, 0o755))

	// Create each artifact
	created := make([]string, 0, len(artifactDefs))
	for _, def := range artifactDefs {
		name, version := def[0], def[1]
		src := createSampleArtifactSource(t, root)
		path := createArtifactViaCLI(t, src, name, version, artifactsDir, nil) // No dependencies for simple artifacts
		created = append(created, path)
	}

	// Generate index.json at repoDir with base-path "artifacts"
	outFile := filepath.Join(repoDir, "index.json")
	generateIndexViaCLI(t, artifactsDir, outFile, "artifacts", true)
	return repoDir, created
}

// buildRepoDirWithArtifactsAndDeps creates a repository directory with artifacts and allows
// specifying dependencies per artifact via a map: deps[artifactName] = []"name:version_constraint".
func buildRepoDirWithArtifactsAndDeps(t *testing.T, root string, artifactDefs [][2]string, deps map[string][]string) (string, []string) {
	t.Helper()
	repoDir := filepath.Join(root, "repo")
	artifactsDir := filepath.Join(repoDir, "artifacts")
	require.NoError(t, os.MkdirAll(artifactsDir, 0o755))

	created := make([]string, 0, len(artifactDefs))
	for _, def := range artifactDefs {
		name, version := def[0], def[1]
		src := createSampleArtifactSource(t, root)
		dependencies := deps[name]
		path := createArtifactViaCLI(t, src, name, version, artifactsDir, dependencies)
		created = append(created, path)
	}

	outFile := filepath.Join(repoDir, "index.json")
	generateIndexViaCLI(t, artifactsDir, outFile, "artifacts", true)
	return repoDir, created
}

// getInstalledArtifactsFromDB loads the installed database using the config at cfgPath
// and returns the list of installed artifacts.
func getInstalledArtifactsFromDB(t *testing.T, cfgPath string) []*model.InstalledArtifact {
	t.Helper()
	cfg, err := config.LoadConfig(cfgPath)
	require.NoError(t, err)

	db := database.NewInstalledDatabase()
	require.NoError(t, db.LoadDatabase(cfg.GetDatabasePath()))
	return db.GetInstalledArtifacts()
}

// buildRepoDirWithArtifactsWithDeps creates a repository directory with artifacts that have dependencies.
func buildRepoDirWithArtifactsWithDeps(t *testing.T, root string, artifactDefs [][2]string, depForFirst string) (string, []string) {
	t.Helper()
	repoDir := filepath.Join(root, "repo")
	artifactsDir := filepath.Join(repoDir, "artifacts")
	require.NoError(t, os.MkdirAll(artifactsDir, 0o755))

	// Create each artifact
	created := make([]string, 0, len(artifactDefs))
	for i, def := range artifactDefs {
		name, version := def[0], def[1]
		src := createSampleArtifactSource(t, root)

		// Add dependency only for the first artifact if specified
		var dependencies []string
		if i == 0 && depForFirst != "" {
			dependencies = []string{depForFirst}
		}

		path := createArtifactViaCLI(t, src, name, version, artifactsDir, dependencies)
		created = append(created, path)
	}

	// Generate index.json at repoDir with base-path "artifacts"
	outFile := filepath.Join(repoDir, "index.json")
	generateIndexViaCLI(t, artifactsDir, outFile, "artifacts", true)
	return repoDir, created
}

// startRepoServer serves the given repo directory (containing index.json and artifacts/).
// Returns the server and a fully-qualified URL to the index.json.
func startRepoServer(t *testing.T, repoDir string) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewServer(http.FileServer(http.Dir(repoDir)))
	indexURL := srv.URL + "/index.json"
	return srv, indexURL
}

// writeTempConfig writes a minimal config YAML to path with a single repository and cache dir.
// If indexURL is empty, writes an empty repositories list.
func writeTempConfig(t *testing.T, path, repoName, indexURL, cacheDir string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))

	// Create temporary directories for installation paths
	tempDir := filepath.Dir(path)
	installDir := filepath.Join(tempDir, "install")
	metaDir := filepath.Join(tempDir, "meta")
	stateDir := filepath.Join(tempDir, "state")

	// Create the full state directory structure
	require.NoError(t, os.MkdirAll(filepath.Join(stateDir, "gotya", "state"), 0o755))

	yamlContent := "settings:\n" +
		"  cache_dir: " + strings.ReplaceAll(cacheDir, "\\", "\\\\") + "\n" +
		"  install_dir: " + strings.ReplaceAll(installDir, "\\", "\\\\") + "\n" +
		"  meta_dir: " + strings.ReplaceAll(metaDir, "\\", "\\\\") + "\n" +
		"  state_dir: " + strings.ReplaceAll(stateDir, "\\", "\\\\") + "\n" +
		"  http_timeout: 5s\n" +
		"  max_concurrent_syncs: 2\n"
	if indexURL != "" {
		yamlContent += "repositories:\n" +
			"  - name: " + repoName + "\n" +
			"    url: " + indexURL + "\n" +
			"    enabled: true\n" +
			"    priority: 1\n"
	} else {
		yamlContent += "repositories: []\n"
	}
	require.NoError(t, os.WriteFile(path, []byte(yamlContent), 0o600))
}

// createSampleArtifactSource creates a minimal valid source directory for artifact packing.
// Layout:
//
//	<root>/data/hello.txt
//
// No meta directory is needed for the minimal case.
func createSampleArtifactSource(t *testing.T, root string) string {
	t.Helper()
	src := filepath.Join(root, "src")
	require.NoError(t, os.MkdirAll(filepath.Join(src, "data"), 0o755))

	// Write a simple file into data/
	content := []byte("hello gotya\n")
	require.NoError(t, os.WriteFile(filepath.Join(src, "data", "hello.txt"), content, 0o644))
	return src
}

// parseCreatedArtifactPath extracts the output path from the create command stdout.
// Expected line: "Successfully created artifact: <path>\n"
func parseCreatedArtifactPath(t *testing.T, out string) string {
	t.Helper()
	idx := strings.LastIndex(out, ": ")
	require.Greater(t, idx, -1, "create output should contain colon with path")
	p := strings.TrimSpace(out[idx+2:])
	require.NotEmpty(t, p, "parsed artifact path must not be empty")
	return p
}

// createArtifactViaCLI packs a source directory into a .gotya file using the CLI and returns the file path.
// Dependencies should be specified as a slice of strings in the format "name:version" (e.g., "testlib:1.0.0").
func createArtifactViaCLI(t *testing.T, src, name, version, outDir string, dependencies []string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(outDir, 0o755))

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newRootCmd()
	args := []string{
		"artifact", "create",
		"--source", src,
		"--name", name,
		"--version", version,
		"--os", runtime.GOOS,
		"--arch", runtime.GOARCH,
		"--output", outDir,
	}

	// Add dependencies if specified
	for _, dep := range dependencies {
		args = append(args, "--depends", dep)
	}

	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.Background())

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()

	require.NoError(t, err, "artifact create should succeed for %s@%s", name, version)
	return parseCreatedArtifactPath(t, out)
}

// generateIndexViaCLI runs `gotya index generate` with optional base-path and force.
func generateIndexViaCLI(t *testing.T, srcDir, outFile, basePath string, force bool) {
	t.Helper()
	args := []string{"index", "generate", srcDir, outFile}
	if basePath != "" {
		args = append(args, "--base-path", basePath)
	}
	if force {
		args = append(args, "--force")
	}
	cmd := newRootCmd()
	cmd.SetArgs(args)
	require.NoError(t, cmd.ExecuteContext(context.Background()))
}
