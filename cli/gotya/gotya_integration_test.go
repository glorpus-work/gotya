package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestServer(t *testing.T, dir string) *httptest.Server {
	handler := http.FileServer(http.Dir(dir))
	return httptest.NewServer(handler)
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
