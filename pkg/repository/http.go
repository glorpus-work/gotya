package repository

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// HTTPClient handles HTTP operations for repositories.
type HTTPClient struct {
	client    *http.Client
	userAgent string
}

// NewHTTPClient creates a new HTTP client for repository operations.
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

// DownloadIndex downloads the repository index from the given URL.
// If lastModified is not zero, it will be used to make a conditional request.
// Returns the index and its last modified time, or ErrNotModified if the index hasn't changed.
func (hc *HTTPClient) DownloadIndex(ctx context.Context, repoURL string, lastModified time.Time) (*IndexImpl, time.Time, error) {
	indexURL, err := hc.buildIndexURL(repoURL)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to build index URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexURL, http.NoBody)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", hc.userAgent)
	req.Header.Set("Accept", "application/json")

	// Add If-Modified-Since header if we have a last modified time
	if !lastModified.IsZero() {
		req.Header.Set("If-Modified-Since", lastModified.UTC().Format(http.TimeFormat))
	}

	resp, err := hc.client.Do(req)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to download index: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		return nil, lastModified, ErrNotModified
	case http.StatusOK:
		// Continue processing
	default:
		return nil, time.Time{}, fmt.Errorf("failed to download index: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to read response body: %w", err)
	}

	// Get the Last-Modified header if available
	var modifiedTime time.Time
	if lastModifiedStr := resp.Header.Get("Last-Modified"); lastModifiedStr != "" {
		modifiedTime, err = http.ParseTime(lastModifiedStr)
		if err != nil {
			modifiedTime = time.Now()
		}
	} else {
		modifiedTime = time.Now()
	}

	index, err := ParseIndex(data)
	if err != nil {
		return nil, time.Time{}, err
	}

	return index, modifiedTime, nil
}

// DownloadPackage downloads a package file from the repository.
func (hc *HTTPClient) DownloadPackage(ctx context.Context, packageURL string, writer io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, packageURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", hc.userAgent)

	resp, err := hc.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download package: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download package: HTTP %d", resp.StatusCode)
	}

	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write package data: %w", err)
	}

	return nil
}

// CheckRepositoryHealth checks if a repository is accessible.
func (hc *HTTPClient) CheckRepositoryHealth(ctx context.Context, repoURL string) error {
	indexURL, err := hc.buildIndexURL(repoURL)
	if err != nil {
		return fmt.Errorf("failed to build index URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, indexURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", hc.userAgent)

	resp, err := hc.client.Do(req)
	if err != nil {
		return fmt.Errorf("repository not accessible: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("repository not healthy: HTTP %d", resp.StatusCode)
	}

	return nil
}

// buildIndexURL constructs the index URL from a repository base URL.
func (hc *HTTPClient) buildIndexURL(repoURL string) (string, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("invalid repository URL: %w", err)
	}

	// Use path.Join for URL paths (always uses forward slashes)
	u.Path, err = url.JoinPath(u.Path, "index.json")
	if err != nil {
		return "", fmt.Errorf("failed to build index URL: %w", err)
	}

	return u.String(), nil
}
