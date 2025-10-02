package config

import "net/url"

// GetURL parses and returns the repository URL.
// Returns nil if the URL is invalid or empty.
func (rc *RepositoryConfig) GetURL() *url.URL {
	if rc.URL == "" {
		return nil
	}

	parse, err := url.Parse(rc.URL)
	if err != nil {
		return nil
	}

	// Check if the URL has a valid scheme
	if parse.Scheme == "" {
		return nil
	}

	return parse
}
