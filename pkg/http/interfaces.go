//go:generate mockgen -destination=mocks/http.go . Client
package http

import (
	"context"
	"net/url"
)

// Client defines the interface for HTTP operations.
type Client interface {
	// DownloadIndex downloads the index from the given repository URL to the specified file path.
	// It returns an error if the download fails or the index hasn't been modified.
	DownloadIndex(ctx context.Context, repoURL *url.URL, filePath string) error

	// DownloadArtifact downloads a package file from the given URL to the specified file path.
	// It returns an error if the download fails.
	DownloadArtifact(ctx context.Context, packageURL *url.URL, filePath string) error
}
