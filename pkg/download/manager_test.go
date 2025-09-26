package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	tests := []struct {
		name       string
		timeout    time.Duration
		userAgent  string
		expectedUA string
	}{
		{
			name:       "default user agent",
			timeout:    time.Second,
			expectedUA: "gotya/1.0",
		},
		{
			name:       "custom user agent",
			timeout:    2 * time.Second,
			userAgent:  "test-agent/1.0",
			expectedUA: "test-agent/1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(tt.timeout, tt.userAgent)
			require.NotNil(t, m)
			assert.Equal(t, tt.timeout, m.client.Timeout)
			assert.Equal(t, tt.expectedUA, m.userAgent)
		})
	}
}

func TestFetch_SingleFile(t *testing.T) {
	tests := []struct {
		name        string
		setupServer func(*testing.T) *httptest.Server
		item        Item
		expectError bool
		checkFile   bool
	}{
		{
			name: "successful download",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("test content"))
				}))
			},
			item: Item{
				ID:  "test1",
				URL: &url.URL{},
			},
			expectError: false,
			checkFile:   true,
		},
		{
			name: "not found",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			item: Item{
				ID:  "test2",
				URL: &url.URL{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer(t)
			defer server.Close()

			// Update the URL to point to our test server
			url, err := url.Parse(server.URL)
			require.NoError(t, err)
			tt.item.URL = url

			tempDir := t.TempDir()
			m := NewManager(time.Second, "test")

			path, err := m.Fetch(context.Background(), tt.item, Options{Dir: tempDir})

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, path)

			if tt.checkFile {
				content, err := os.ReadFile(path)
				require.NoError(t, err)
				assert.Equal(t, "test content", string(content))
			}
		})
	}
}

func TestFetch_WithChecksum(t *testing.T) {
	// Calculate SHA-256 of test content
	h := sha256.New()
	_, err := h.Write([]byte("test content"))
	require.NoError(t, err)
	checksum := hex.EncodeToString(h.Sum(nil))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test content"))
	}))
	defer server.Close()

	tests := []struct {
		name        string
		checksum    string
		expectError bool
	}{
		{
			name:        "valid checksum",
			checksum:    checksum,
			expectError: false,
		},
		{
			name:        "invalid checksum",
			checksum:    "invalidchecksum1234567890abcdef1234567890abcdef1234567890abcdef12345678",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := url.Parse(server.URL)
			require.NoError(t, err)

			item := Item{
				ID:       "test-checksum",
				URL:      url,
				Checksum: tt.checksum,
			}

			tempDir := t.TempDir()
			m := NewManager(time.Second, "test")

			_, err = m.Fetch(context.Background(), item, Options{Dir: tempDir})

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFetchAll_Concurrent(t *testing.T) {
	const numItems = 5
	var serverResponses = make(map[string]string)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract the item ID from the URL path
		id := r.URL.Path[1:] // remove leading slash
		content, exists := serverResponses[id]
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(content))
	}))

	defer server.Close()

	// Prepare test data
	var items []Item
	for i := 0; i < numItems; i++ {
		id := string(rune('a' + i)) // a, b, c, ...
		content := "content for " + id
		serverResponses[id] = content

		url, err := url.Parse(server.URL + "/" + id)
		require.NoError(t, err)

		items = append(items, Item{
			ID:  id,
			URL: url,
		})
	}

	tests := []struct {
		name       string
		concurrent bool
	}{
		{
			name:       "sequential",
			concurrent: false,
		},
		{
			name:       "concurrent",
			concurrent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			m := NewManager(5*time.Second, "test")

			opts := Options{
				Dir: tempDir,
			}
			if tt.concurrent {
				opts.Concurrency = 3 // Test with 3 concurrent workers
			}

			results, err := m.FetchAll(context.Background(), items, opts)
			require.NoError(t, err)
			require.Len(t, results, numItems)

			// Verify all files were downloaded correctly
			for i, item := range items {
				path, ok := results[item.ID]
				require.True(t, ok, "missing result for item %d", i)
				require.NotEmpty(t, path, "empty path for item %d", i)

				content, err := os.ReadFile(path)
				require.NoError(t, err, "failed to read file for item %d", i)
				require.Equal(t, serverResponses[item.ID], string(content), "content mismatch for item %d", i)
			}
		})
	}
}

func TestFetch_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		setupServer func(*testing.T) *httptest.Server
		item        Item
		expectError string
	}{
		{
			name: "invalid URL",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte("bad request"))
				}))
			},
			item: Item{
				ID:  "bad-request",
				URL: &url.URL{},
			},
			expectError: "unexpected status code: 400",
		},
		{
			name: "server error",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
			},
			item: Item{
				ID:  "server-error",
				URL: &url.URL{},
			},
			expectError: "unexpected status code: 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer(t)
			defer server.Close()

			if tt.item.URL.Host == "" {
				url, err := url.Parse(server.URL)
				require.NoError(t, err)
				tt.item.URL = url
			}

			tempDir := t.TempDir()
			m := NewManager(time.Second, "test")

			_, err := m.Fetch(context.Background(), tt.item, Options{Dir: tempDir})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

func TestFetch_InvalidDirectory(t *testing.T) {
	m := NewManager(time.Second, "test")

	// Create a temp file to use as an invalid directory
	tempFile, err := os.CreateTemp("", "test-*")
	require.NoError(t, err)
	defer func() {
		_ = os.Remove(tempFile.Name())
		_ = tempFile.Close()
	}()

	item := Item{
		ID:  "test",
		URL: &url.URL{Scheme: "http", Host: "example.com"},
	}

	_, err = m.Fetch(context.Background(), item, Options{Dir: tempFile.Name()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "could not create download dir")
}

func TestFetch_WriteError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping write error simulation on Windows; chmod does not reliably block writes")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test content"))
	}))
	defer server.Close()

	url, err := url.Parse(server.URL)
	require.NoError(t, err)

	item := Item{
		ID:  "test-write-error",
		URL: url,
	}

	// Create a read-only directory
	tempDir := t.TempDir()
	err = os.Chmod(tempDir, 0500) // read and execute, no write
	require.NoError(t, err)

	m := NewManager(time.Second, "test")
	_, err = m.Fetch(context.Background(), item, Options{Dir: tempDir})
	assert.Error(t, err)
}
