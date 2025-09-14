//go:build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

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
}

func runCLITest(t *testing.T, binaryPath string, test cliTest) {
	t.Helper()

	if test.skip {
		t.Skip("Skipping test: " + test.name)
	}

	t.Run(test.name, func(t *testing.T) {
		cmd := exec.Command(binaryPath, test.args...)

		// Capture output
		var stdout, stderr strings.Builder
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		// Set up a clean environment
		cmd.Env = append(os.Environ(),
			"GOTYA_CONFIG_DIR="+t.TempDir(),
			"NO_COLOR=true", // Disable color output for consistent test results
		)

		err := cmd.Run()

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
	})
}

func TestCLIIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binaryPath := buildTestBinary(t)

	tests := []cliTest{
		// Basic commands
		{
			name:           "help command",
			args:           []string{"help"},
			expectedOutput: "gotya is a lightweight personal artifact manager",
		},
		{
			name:           "version command",
			args:           []string{"version"},
			expectedOutput: "gotya version",
		},
		{
			name:          "unknown command",
			args:          []string{"nonexistent-command"},
			expectedError: "unknown command",
		},

		// Install command
		{
			name:           "install help",
			args:           []string{"install", "--help"},
			expectedOutput: "Install one or more packages from the configured repositories",
		},
		{
			name:          "install with no arguments",
			args:          []string{"install"},
			expectedError: "requires at least 1 arg(s), only received 0",
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

		// Cache command
		{
			name:           "cache help",
			args:           []string{"cache", "--help"},
			expectedOutput: "Clean, show information about, and manage the artifact cache",
		},
		{
			name:           "cache clean help",
			args:           []string{"cache", "clean", "--help"},
			expectedOutput: "Remove cached files to free up disk space",
		},

		// Config command
		{
			name:           "config help",
			args:           []string{"config", "--help"},
			expectedOutput: "View and modify gotya configuration settings",
		},
		{
			name:           "config show",
			args:           []string{"config", "show"},
			expectedOutput: "SETTING",
		},

		// Sync command
		{
			name:           "sync help",
			args:           []string{"sync", "--help"},
			expectedOutput: "Synchronize artifact index indexes",
		},
		// Add more test cases for other commands as needed
	}

	for _, tt := range tests {
		tt := tt
		runCLITest(t, binaryPath, tt)
	}
}
