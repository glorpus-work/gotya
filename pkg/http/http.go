package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/cperrin88/gotya/pkg/errors"
	"github.com/cperrin88/gotya/pkg/fsutil"
	"github.com/cperrin88/gotya/pkg/index"
)

// HTTPClient handles HTTP operations for repositories.
type HTTPClient struct {
	client    *http.Client
	userAgent string
}

// NewHTTPClient creates a new HTTP client for index operations.
func NewHTTPClient(timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: timeout,
		},
		userAgent: "gotya/1.0",
	}
}

// ErrNotModified is returned when the index hasn't been modified since the last request.
var ErrNotModified = fmt.Errorf("index not modified")

// DownloadIndex downloads the index index from the given URL.
// If lastModified is not zero, it will be used to make a conditional request.
// Returns the index and its last modified time, or ErrNotModified if the index hasn't changed.
func (hc *HTTPClient) DownloadIndex(ctx context.Context, repoURL string, filePath string) error {
	indexURL, err := hc.buildIndexURL(repoURL)
	if err != nil {
		return errors.Wrapf(err, "failed to build index URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexURL, http.NoBody)
	if err != nil {
		return errors.Wrapf(err, "failed to create request")
	}

	req.Header.Set("User-Agent", hc.userAgent)
	req.Header.Set("Accept", "application/json")

	// Add If-Modified-Since header if we have a last modified time
	//if !lastModified.IsZero() {
	//	req.Header.Set("If-Modified-Since", lastModified.UTC().Format(http.TimeFormat))
	//}

	resp, err := hc.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to download index")
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		return nil
	case http.StatusOK:
		// Continue processing
	default:
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response body")
	}

	if err := os.MkdirAll(filepath.Dir(filePath), fsutil.DirModeSecure); err != nil {
		return errors.Wrap(err, "could not create directory for index")
	}

	if err := os.WriteFile(filePath, data, fsutil.FileModeSecure); err != nil {
		return errors.Wrap(err, "could not write index")
	}

	// Get the Last-Modified header if available
	if lastModifiedStr := resp.Header.Get("Last-Modified"); lastModifiedStr != "" {
		modifiedTime, err := http.ParseTime(lastModifiedStr)
		if err == nil {
			if err := os.Chtimes(filePath, modifiedTime, modifiedTime); err != nil {
				return errors.Wrap(err, "could change times on index file")
			}
		}
	}

	return nil
}

// DownloadPackage downloads a pkg file from the index.
func (hc *HTTPClient) DownloadPackage(ctx context.Context, packageURL string, filePath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, packageURL, http.NoBody)
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	req.Header.Set("User-Agent", hc.userAgent)

	resp, err := hc.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to download pkg")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to write pkg data")
	}

	return nil
}

// CheckRepositoryHealth checks if a index is accessible.
func (hc *HTTPClient) CheckRepositoryHealth(ctx context.Context, repoURL string) error {
	indexURL, err := hc.buildIndexURL(repoURL)
	if err != nil {
		return errors.Wrap(err, "failed to build index URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, indexURL, http.NoBody)
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	req.Header.Set("User-Agent", hc.userAgent)

	resp, err := hc.client.Do(req)
	if err != nil {
		return fmt.Errorf("index not accessible: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("index not healthy: HTTP %d", resp.StatusCode)
	}

	return nil
}

// buildIndexURL constructs the index URL from a index base URL.
func (hc *HTTPClient) buildIndexURL(repoURL string) (string, error) {
	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return "", errors.Wrap(index.ErrRepositoryURLInvalid, "invalid index URL")
	}

	// Use path.Join for URL paths (always uses forward slashes)
	parsedURL.Path, err = url.JoinPath(parsedURL.Path, "index.json")
	if err != nil {
		return "", errors.Wrap(err, "failed to build index URL")
	}

	return parsedURL.String(), nil
}
