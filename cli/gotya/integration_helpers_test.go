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

	"github.com/stretchr/testify/require"
)

// buildRepoDirWithArtifacts creates a temporary repo directory structure containing artifacts and an index.json.
// Returns repoDir and the list of created artifact file paths.
func buildRepoDirWithArtifacts(t *testing.T, root string, artifactDefs [][2]string) (string, []string) {
	t.Helper()
	repoDir := filepath.Join(root, "repo")
	artifactsDir := filepath.Join(repoDir, "packages")
	require.NoError(t, os.MkdirAll(artifactsDir, 0o755))

	// Create each artifact
	created := make([]string, 0, len(artifactDefs))
	for _, def := range artifactDefs {
		name, version := def[0], def[1]
		src := createSampleArtifactSource(t, root)
		path := createArtifactViaCLI(t, src, name, version, artifactsDir)
		created = append(created, path)
	}

	// Generate index.json at repoDir with base-path "packages"
	outFile := filepath.Join(repoDir, "index.json")
	generateIndexViaCLI(t, artifactsDir, outFile, "packages", true)
	return repoDir, created
}

// startRepoServer serves the given repo directory (containing index.json and packages/).
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
	yamlContent := "settings:\n" +
		"  cache_dir: " + strings.ReplaceAll(cacheDir, "\\", "\\\\") + "\n" +
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
func createArtifactViaCLI(t *testing.T, src, name, version, outDir string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(outDir, 0o755))

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"artifact", "create",
		"--source", src,
		"--name", name,
		"--version", version,
		"--os", runtime.GOOS,
		"--arch", runtime.GOARCH,
		"--output", outDir,
	})
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
