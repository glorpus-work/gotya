package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	idx "github.com/cperrin88/gotya/pkg/index"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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
	yaml := "settings:\n" +
		"  cache_dir: " + strings.ReplaceAll(cacheDir, "\\", "\\\\") + "\n" +
		"  http_timeout: 5s\n" +
		"  max_concurrent_syncs: 2\n"
	if indexURL != "" {
		yaml += "repositories:\n" +
			"  - name: " + repoName + "\n" +
			"    url: " + indexURL + "\n" +
			"    enabled: true\n" +
			"    priority: 1\n"
	} else {
		yaml += "repositories: []\n"
	}
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))
}

func TestSync_SuccessDownloadsIndex(t *testing.T) {
	tempDir := t.TempDir()
	// Build a repo with two artifacts and serve it
	defs := [][2]string{{"alpha", "1.0.0"}, {"beta", "2.1.0"}}
	repoDir, _ := buildRepoDirWithArtifacts(t, tempDir, defs)
	srv, idxURL := startRepoServer(t, repoDir)
	defer srv.Close()

	// Write a temporary config pointing to our served index.json
	cfgPath := filepath.Join(tempDir, "config.yaml")
	cacheDir := filepath.Join(tempDir, "cache")
	writeTempConfig(t, cfgPath, "testrepo", idxURL, cacheDir)

	// Run sync
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "sync"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Verify index was downloaded to cacheDir/indexes/testrepo.json
	downloaded := filepath.Join(cacheDir, "indexes", "testrepo.json")
	if _, err := os.Stat(downloaded); err != nil {
		t.Fatalf("expected downloaded index at %s: %v", downloaded, err)
	}

	// Parse and validate content
	parsed, err := idx.ParseIndexFromFile(downloaded)
	require.NoError(t, err)
	require.NotNil(t, parsed)
	assert.GreaterOrEqual(t, len(parsed.Artifacts), 2)
}

func TestSync_FailsForMissingIndex(t *testing.T) {
	tempDir := t.TempDir()
	// Serve an empty directory; index.json will be missing and return 404
	emptyDir := filepath.Join(tempDir, "empty")
	require.NoError(t, os.MkdirAll(emptyDir, 0o755))
	srv := httptest.NewServer(http.FileServer(http.Dir(emptyDir)))
	defer srv.Close()

	// Point repo URL to missing index.json
	cfgPath := filepath.Join(tempDir, "config.yaml")
	cacheDir := filepath.Join(tempDir, "cache")
	writeTempConfig(t, cfgPath, "badrepo", srv.URL+"/index.json", cacheDir)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "sync"})
	err := cmd.ExecuteContext(context.Background())
	require.Error(t, err)
}

func TestSync_NoRepositoriesDoesNothing(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.yaml")
	cacheDir := filepath.Join(tempDir, "cache")
	// Write config with empty repositories
	writeTempConfig(t, cfgPath, "", "", cacheDir)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "sync"})
	require.NoError(t, cmd.ExecuteContext(context.Background()))

	// Ensure no indexes directory is created (or empty)
	idxDir := filepath.Join(cacheDir, "indexes")
	if fi, err := os.Stat(idxDir); err == nil && fi.IsDir() {
		entries, _ := os.ReadDir(idxDir)
		assert.Equal(t, 0, len(entries))
	}
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

func TestArtifact_CreateAndVerify_Success(t *testing.T) {
	tempDir := t.TempDir()
	src := createSampleArtifactSource(t, tempDir)
	outDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(outDir, 0o755))

	name := "sample"
	version := "1.2.3"
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Capture stdout for create
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"artifact", "create",
		"--source", src,
		"--name", name,
		"--version", version,
		"--os", goos,
		"--arch", goarch,
		"--output", outDir,
	})
	err := cmd.ExecuteContext(context.Background())

	// Restore stdout and read output
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()

	require.NoError(t, err, "artifact create should succeed")

	// Parse created file path and assert it exists
	createdPath := parseCreatedArtifactPath(t, out)
	if _, statErr := os.Stat(createdPath); statErr != nil {
		t.Fatalf("expected created artifact to exist at %s: %v\ncreate output: %s", createdPath, statErr, out)
	}

	// Verify using flag -f
	verifyCmd := newRootCmd()
	verifyCmd.SetArgs([]string{"artifact", "verify", "-f", createdPath})
	require.NoError(t, verifyCmd.ExecuteContext(context.Background()))

	// Also verify using positional argument
	verifyCmd2 := newRootCmd()
	verifyCmd2.SetArgs([]string{"artifact", "verify", createdPath})
	require.NoError(t, verifyCmd2.ExecuteContext(context.Background()))
}

func TestArtifact_Verify_FailsForMissingFile(t *testing.T) {
	// Use a path we know does not exist
	missing := filepath.Join(t.TempDir(), "does-not-exist.gotya")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"artifact", "verify", "-f", missing})
	err := cmd.ExecuteContext(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestArtifact_Create_FailsForInvalidInput(t *testing.T) {
	tempDir := t.TempDir()
	// Build a source directory with an invalid top-level file
	src := filepath.Join(tempDir, "badsrc")
	require.NoError(t, os.MkdirAll(src, 0o755))
	// Add a forbidden file at the root (only meta/ and data/ are allowed)
	require.NoError(t, os.WriteFile(filepath.Join(src, "FORBIDDEN.txt"), []byte("nope"), 0o644))

	outDir := filepath.Join(tempDir, "out")
	require.NoError(t, os.MkdirAll(outDir, 0o755))

	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"artifact", "create",
		"--source", src,
		"--name", "bad",
		"--version", "0.0.1",
		"--os", runtime.GOOS,
		"--arch", runtime.GOARCH,
		"--output", outDir,
	})
	err := cmd.ExecuteContext(context.Background())
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "not allowed")

	// Also validate that nothing got created in outDir matching the expected name
	expected := filepath.Join(outDir, fmt.Sprintf("%s_%s_%s_%s.gotya", "bad", "0.0.1", runtime.GOOS, runtime.GOARCH))
	if _, statErr := os.Stat(expected); !os.IsNotExist(statErr) {
		t.Fatalf("expected no artifact to be created at %s; statErr=%v", expected, statErr)
	}
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

	w.Close()
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

func TestIndex_Generate_SimpleSuccess(t *testing.T) {
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	repoDir := filepath.Join(tempDir, "repo")
	outFile := filepath.Join(repoDir, "index.json")

	// Create artifacts
	src1 := createSampleArtifactSource(t, tempDir)
	_ = createArtifactViaCLI(t, src1, "alpha", "1.0.0", artifactsDir)

	src2 := createSampleArtifactSource(t, tempDir)
	_ = createArtifactViaCLI(t, src2, "beta", "2.0.0", artifactsDir)

	// Generate index
	generateIndexViaCLI(t, artifactsDir, outFile, "packages", true)

	// Validate index file exists and basic content
	if _, err := os.Stat(outFile); err != nil {
		t.Fatalf("expected index file to be created at %s: %v", outFile, err)
	}

	indexFile, err := idx.ParseIndexFromFile(outFile)
	require.NoError(t, err)
	require.NotNil(t, indexFile)
	require.GreaterOrEqual(t, len(indexFile.Artifacts), 2)

	names := map[string]bool{}
	for _, it := range indexFile.Artifacts {
		names[it.Name] = true
		assert.NotEmpty(t, it.URL)
		// URL should be a relative path (not absolute URL or filesystem root)
		assert.False(t, strings.HasPrefix(it.URL, "http"), "url should be relative, not absolute")
		assert.False(t, strings.HasPrefix(it.URL, "/"), "url should be relative, not absolute")
		assert.True(t, strings.HasSuffix(strings.ToLower(it.URL), ".gotya"))
		assert.Equal(t, 64, len(it.Checksum))
		assert.NotZero(t, it.Size)
		assert.NotEmpty(t, it.OS)
		assert.NotEmpty(t, it.Arch)
	}
	assert.True(t, names["alpha"])
	assert.True(t, names["beta"])
}

func TestIndex_Generate_FailsForMissingSourceDir(t *testing.T) {
	tempDir := t.TempDir()
	outFile := filepath.Join(tempDir, "index.json")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"index", "generate", filepath.Join(tempDir, "nope"), outFile})
	err := cmd.ExecuteContext(context.Background())
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "source directory")
}

func TestIndex_Generate_OverwriteBehavior(t *testing.T) {
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	repoDir := filepath.Join(tempDir, "repo")
	outFile := filepath.Join(repoDir, "index.json")

	src := createSampleArtifactSource(t, tempDir)
	_ = createArtifactViaCLI(t, src, "gamma", "0.1.0", artifactsDir)

	require.NoError(t, os.MkdirAll(filepath.Dir(outFile), 0o755))
	require.NoError(t, os.WriteFile(outFile, []byte("{}"), 0o644))

	// Without --force should fail
	cmd := newRootCmd()
	cmd.SetArgs([]string{"index", "generate", artifactsDir, outFile})
	err := cmd.ExecuteContext(context.Background())
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "output file exists")

	// With --force succeeds and produces valid index
	generateIndexViaCLI(t, artifactsDir, outFile, "", true)
	parsed, perr := idx.ParseIndexFromFile(outFile)
	require.NoError(t, perr)
	require.NotNil(t, parsed)
	assert.GreaterOrEqual(t, len(parsed.Artifacts), 1)
}

func TestMain(m *testing.M) {
	// Setup test environment
	code := m.Run()
	os.Exit(code)
}

func TestVersionCommand(t *testing.T) {
	// Redirect stdout to capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute the version command
	cmd := newRootCmd()
	cmd.SetArgs([]string{"version"})
	err := cmd.ExecuteContext(context.Background())

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Check results
	assert.NoError(t, err, "version command should not return an error")

	// Read the captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Basic check that the output contains version-like information
	assert.Contains(t, output, "gotya version", "version output should contain 'gotya version'")
}

func TestHelpCommand(t *testing.T) {
	// Redirect stdout to capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute the help command
	cmd := newRootCmd()
	cmd.SetArgs([]string{"help"})
	err := cmd.ExecuteContext(context.Background())

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Check results
	assert.NoError(t, err, "help command should not return an error")

	// Read the captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check for expected help text
	expectedDescription := "gotya is a lightweight personal artifact manager (like apt) with:"
	assert.Contains(t, output, expectedDescription, "help output should contain description")
	assert.Contains(t, output, "Available Commands", "help output should list available commands")
}

func TestConfigShowDefault(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()

	// Set up environment for the test
	os.Setenv("GOTYA_CONFIG_DIR", tempDir)
	defer os.Unsetenv("GOTYA_CONFIG_DIR")

	// Redirect stdout to capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute the config show command
	cmd := newRootCmd()
	cmd.SetArgs([]string{"config", "show"})
	err := cmd.ExecuteContext(context.Background())

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Check results
	assert.NoError(t, err, "config show command should not return an error")

	// Read the captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check for expected output
	assert.Contains(t, output, "SETTING", "output should contain settings section")
	assert.Contains(t, output, "VALUE", "output should contain settings section")
	assert.Contains(t, output, "Repositories", "output should contain repositories section")
}

func TestConfigWithCustomFile(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "custom-config.yaml")

	// Create a custom config file
	customConfig := `repositories:
  - name: test-repo
    url: http://example.com/repo
    enabled: true
    priority: 1
settings:
  http_timeout: 60s
  output_format: json
  log_level: debug`

	err := os.WriteFile(configPath, []byte(customConfig), 0o600)
	require.NoError(t, err, "failed to create custom config file")

	// Redirect stdout to capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute the config show command with custom config
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--config", configPath, "config", "show"})
	err = cmd.ExecuteContext(context.Background())

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Check results
	assert.NoError(t, err, "config show with custom config should not return an error")

	// Read the captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check for expected output from custom config
	assert.Contains(t, output, "test-repo", "output should contain custom repository")
	assert.Contains(t, output, "http://example.com/repo", "output should contain custom repository URL")
	assert.Contains(t, output, "Output_Format    json", "output should contain custom output format")
	assert.Contains(t, output, "Log_Level        debug", "output should contain custom log level")
}

func TestConfigSetAndGet(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()

	// Set up environment for the test
	os.Setenv("GOTYA_CONFIG_DIR", tempDir)
	defer os.Unsetenv("GOTYA_CONFIG_DIR")

	// Test setting a value
	t.Run("set value", func(t *testing.T) {
		cmd := newRootCmd()
		cmd.SetArgs([]string{"config", "set", "log_level", "debug"})
		require.NoError(t, cmd.ExecuteContext(context.Background()), "config set should not return an error")

		// Verify the value was set by using the config get command
		// Redirect stdout to capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Get the value we just set
		getCmd := newRootCmd()
		getCmd.SetArgs([]string{"config", "get", "log_level"})
		err := getCmd.ExecuteContext(context.Background())

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Check results
		require.NoError(t, err, "config get should not return an error")

		// Read the captured output
		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := strings.TrimSpace(buf.String())

		// Check the output contains the expected value
		assert.Equal(t, "debug", output, "should return the correct log level")
	})

	// Test getting a value
	t.Run("get value", func(t *testing.T) {
		// Redirect stdout to capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Get the value we just set
		cmd := newRootCmd()
		cmd.SetArgs([]string{"config", "get", "log_level"})
		err := cmd.ExecuteContext(context.Background())

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Check results
		assert.NoError(t, err, "config get should not return an error")

		// Read the captured output
		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := strings.TrimSpace(buf.String())

		// Check the output contains the expected value
		assert.Equal(t, "debug", output, "should return the correct log level")
	})
}

func TestConfigInit(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "new-config.yaml")

	// Test config init
	t.Run("init config", func(t *testing.T) {
		// Redirect stdout to capture output
		oldStdout := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		// Initialize a new config file
		cmd := newRootCmd()
		cmd.SetArgs([]string{"--config", configPath, "config", "init"})
		err := cmd.ExecuteContext(context.Background())

		// Restore stdout
		w.Close()
		os.Stdout = oldStdout

		// Check results
		assert.NoError(t, err, "config init should not return an error")

		// Verify the config file was created
		_, err = os.Stat(configPath)
		assert.NoError(t, err, "config file should be created")

		// Read the config file
		configData, err := os.ReadFile(configPath)
		require.NoError(t, err, "failed to read config file")

		// Parse the YAML to verify it's valid
		var cfg map[string]interface{}
		require.NoError(t, yaml.Unmarshal(configData, &cfg), "config file should be valid YAML")

		// Check for required sections
		_, hasRepos := cfg["repositories"]
		_, hasSettings := cfg["settings"]
		assert.True(t, hasRepos, "config should have repositories section")
		assert.True(t, hasSettings, "config should have settings section")
	})
}
