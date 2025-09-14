//go:build integration
// +build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cperrin88/gotya/test/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildTestBinary(t *testing.T) string {
	t.Helper()

	// Create a temporary directory for the test binary
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "gotya")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	// Build the test binary from the project root
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cli/gotya")
	cmd.Dir = filepath.Clean(filepath.Join("..", ".."))

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed to build test binary: %s", string(output))

	return binaryPath
}

type cliTest struct {
	name           string
	args           []string
	expectedOutput string
	expectedError  string
	skip           bool
	skipOnCI       bool                                  // Skip this test when running in CI environment
	setup          func(t *testing.T, configPath string) // Optional setup function
}

func runCLITest(t *testing.T, binaryPath string, test cliTest) {
	t.Helper()

	if test.skip || (os.Getenv("CI") != "" && test.skipOnCI) {
		t.Skip("Skipping test: " + test.name)
	}

	t.Run(test.name, func(t *testing.T) {
		// Create a temporary directory for this test run
		tempDir := t.TempDir()

		// Set up test environment
		envVars := []string{
			"GOTYA_CONFIG_DIR=" + tempDir,
			"GOTYA_CACHE_DIR=" + filepath.Join(tempDir, "cache"),
			"GOTYA_INSTALL_DIR=" + filepath.Join(tempDir, "installed"),
			"NO_COLOR=true", // Disable color output for consistent test results
		}

		// Run setup function if provided
		if test.setup != nil {
			test.setup(t, tempDir)
		}

		// Prepare the command
		cmd := exec.Command(binaryPath, test.args...)

		// Capture output
		var stdout, stderr strings.Builder
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		// Set up environment
		cmd.Env = append(os.Environ(), envVars...)

		// Run the command with a timeout
		done := make(chan error, 1)
		go func() {
			done <- cmd.Run()
		}()

		// Wait for command to complete or timeout
		select {
		case err := <-done:
			// Check error expectations
			if test.expectedError != "" {
				require.Error(t, err, "expected error but got none")
				assert.Contains(t, stderr.String(), test.expectedError, "stderr should contain expected error")
			} else {
				assert.NoError(t, err, "unexpected error: %v\nstderr: %s", err, stderr.String())
			}

			// Check output expectations
			if test.expectedOutput != "" {
				assert.Contains(t, stdout.String(), test.expectedOutput, "stdout should contain expected output")
			}

		case <-time.After(30 * time.Second):
			t.Fatal("Test timed out after 30 seconds")
		}
	})
}

func TestCLIIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Start test server
	ts := testutil.NewTestServer(52221, testutil.GetTestRepoPath())
	defer ts.Stop(t) // Pass the testing.T to Stop()

	// Create test config
	_ = testutil.SetupTestConfig(t, ts.URL) // Ignore unused configPath

	// Build test binary
	binaryPath := buildTestBinary(t)

	tests := []cliTest{
		// Basic commands
		{
			name:           "help command",
			args:           []string{"help"},
			expectedOutput: "gotya is a lightweight personal artifact manager",
		},

		// Version command
		{
			name:           "version command",
			args:           []string{"version"},
			expectedOutput: "gotya version",
		},

		// Config command
		{
			name:           "config help",
			args:           []string{"config", "--help"},
			expectedOutput: "View and modify gotya configuration settings",
		},

		// Cache commands
		{
			name:           "cache help",
			args:           []string{"cache", "--help"},
			expectedOutput: "Clean, show information about, and manage the artifact cache",
		},

		// Install command - basic test without actual installation
		{
			name:           "install help",
			args:           []string{"install", "--help"},
			expectedOutput: "Install one or more packages from the configured repositories",
		},

		// Search command
		{
			name:           "search help",
			args:           []string{"search", "--help"},
			expectedOutput: "Search for packages in the configured repositories",
		},
		{
			name:          "search with no query",
			args:          []string{"search"},
			expectedError: "accepts 1 arg(s), received 0",
		},

		// List command
		{
			name:           "list help",
			args:           []string{"list", "--help"},
			expectedOutput: "List installed or available packages",
		},

		// Error cases
		{
			name:          "unknown command",
			args:          []string{"nonexistent-command"},
			expectedError: "unknown command",
		},

		// Config command
		{
			name:           "config show",
			args:           []string{"config", "show"},
			expectedOutput: "SETTING",
		},

		// Sync command
		{
			name:           "sync help",
			args:           []string{"sync", "--help"},
			expectedOutput: "Synchronize artifact index indexes by downloading the latest",
		},
		// Add more test cases for other commands as needed
	}

	for _, tt := range tests {
		tt := tt
		runCLITest(t, binaryPath, tt)
	}
}
