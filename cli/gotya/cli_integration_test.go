//go:build integration

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

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
	_ = w.Close()
	os.Stdout = oldStdout

	// Check results
	require.NoError(t, err, "version command should not return an error")

	// Read the captured output
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
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
	_ = w.Close()
	os.Stdout = oldStdout

	// Check results
	require.NoError(t, err, "help command should not return an error")

	// Read the captured output
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
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
	t.Setenv("GOTYA_CONFIG_DIR", tempDir)

	// Redirect stdout to capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Execute the config show command
	cmd := newRootCmd()
	cmd.SetArgs([]string{"config", "show"})
	err := cmd.ExecuteContext(context.Background())

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout

	// Check results
	require.NoError(t, err, "config show command should not return an error")

	// Read the captured output
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
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
	_ = w.Close()
	os.Stdout = oldStdout

	// Check results
	require.NoError(t, err, "config show with custom config should not return an error")

	// Read the captured output
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Check for expected output from custom config
	assert.Contains(t, output, "test-repo", "output should contain custom repository")
	assert.Contains(t, output, "http://example.com/repo", "output should contain custom repository URL")
	assert.Contains(t, output, "output_format", "output should contain custom output format")
	assert.Contains(t, output, "log_level", "output should contain custom log level")
}

func TestConfigSetAndGet(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()

	// Set up environment for the test
	t.Setenv("GOTYA_CONFIG_DIR", tempDir)

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
		_ = w.Close()
		os.Stdout = oldStdout

		// Check results
		require.NoError(t, err, "config get should not return an error")

		// Read the captured output
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
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
		_ = w.Close()
		os.Stdout = oldStdout

		// Check results
		require.NoError(t, err, "config get should not return an error")

		// Read the captured output
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
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
		_ = w.Close()
		os.Stdout = oldStdout

		// Check results
		require.NoError(t, err, "config init should not return an error")

		// Verify the config file was created
		_, err = os.Stat(configPath)
		require.NoError(t, err, "config file should be created")

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
