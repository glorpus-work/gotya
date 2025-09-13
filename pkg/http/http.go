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
)

// ClientImpl handles HTTP operations for repositories.
type ClientImpl struct {
	client    *http.Client
	userAgent string
}

// NewHTTPClient creates a new HTTP client for index operations.
func NewHTTPClient(timeout time.Duration) *ClientImpl {
	return &ClientImpl{
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
func (hc *ClientImpl) DownloadIndex(ctx context.Context, repoURL *url.URL, filePath string) error {
	indexURL, err := hc.buildIndexURL(repoURL)
	if err != nil {
		return errors.Wrapf(err, "failed to build index URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexURL.String(), http.NoBody)
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

	if err := os.MkdirAll(filepath.Dir(filePath), fsutil.DirModeSecure); err != nil {
		return errors.Wrap(err, "could not create directory for index")
	}

	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fsutil.FileModeSecure)
	if err != nil {
		return errors.Wrap(err, "could not open index file")
	}

	_, err = io.Copy(file, resp.Body)
	if err != nil {
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

// DownloadArtifact downloads a artifact file from the index.
func (hc *ClientImpl) DownloadArtifact(ctx context.Context, packageURL *url.URL, filePath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, packageURL.String(), http.NoBody)
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	req.Header.Set("User-Agent", hc.userAgent)

	resp, err := hc.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to download artifact")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(filePath), fsutil.DirModeSecure); err != nil {
		return err
	}

	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fsutil.FileModeSecure)
	if err != nil {
		return errors.Wrap(err, "could not open index file")
	}

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return errors.Wrap(err, "could not write index")
	}

	return nil
}

// buildIndexURL constructs the index URL from a index base URL.
func (hc *ClientImpl) buildIndexURL(repoURL *url.URL) (*url.URL, error) {
	// Use path.Join for URL paths (always uses forward slashes)
	var err error
	repoURL.Path, err = url.JoinPath(repoURL.Path, "index.json")
	if err != nil {
		return nil, errors.Wrap(err, "failed to build index URL")
	}

	return repoURL, nil
}
