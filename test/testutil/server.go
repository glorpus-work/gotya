package testutil

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cperrin88/gotya/internal/logger"
)

// TestServer represents a test HTTP server for testing
type TestServer struct {
	Server *http.Server
	URL    string
}

// NewTestServer creates a new test server that serves files from the given directory
func NewTestServer(port int, dir string) *TestServer {
	handler := http.FileServer(http.Dir(dir))
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}

	return &TestServer{
		Server: server,
		URL:    fmt.Sprintf("http://localhost:%d", port),
	}
}

// Start starts the test server
func (ts *TestServer) Start(t *testing.T) {
	t.Helper()
	go func() {
		if err := ts.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("Test server error: %v", err)
		}
	}()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)
}

// Stop stops the test server
func (ts *TestServer) Stop(t *testing.T) {
	t.Helper()
	if ts.Server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		if err := ts.Server.Shutdown(ctx); err != nil {
			t.Logf("Error shutting down test server: %v", err)
		}
	}
}

// getProjectRoot returns the absolute path to the project root directory
func getProjectRoot() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("Failed to get current file path")
	}
	// Navigate up to the project root (3 levels up from test/testutil)
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

// GetTestRepoPath returns the path to the test repository
func GetTestRepoPath() string {
	projectRoot := getProjectRoot()
	repoPath := filepath.Join(projectRoot, "test", "repo")
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		panic(fmt.Sprintf("Test repository not found at: %s", repoPath))
	}
	return repoPath
}

// SetupTestConfig creates a test configuration file in a temporary directory
func SetupTestConfig(t *testing.T, repoURL string) string {
	t.Helper()

	// Create a temporary directory for the test config
	tempDir := t.TempDir()

	// Get the absolute path to the test config file
	projectRoot := getProjectRoot()
	testConfigPath := filepath.Join(projectRoot, "test", "config.yaml")
	logger.Debugf("Reading test config from: %s", testConfigPath)

	configData, err := os.ReadFile(testConfigPath)
	if err != nil {
		t.Fatalf("Failed to read test config from %s: %v", testConfigPath, err)
	}

	// Update the repository URL in the config
	configStr := string(configData)
	// Only format if the config contains a format verb
	if strings.Contains(configStr, "%s") {
		configStr = fmt.Sprintf(configStr, repoURL)
	}

	// Write the updated config to the temp directory
	configPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configStr), 0o600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	return configPath
}
