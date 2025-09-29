package config

import "net/url"

// GetURL parses and returns the repository URL.
func (rc *RepositoryConfig) GetURL() *url.URL {
	parse, err := url.Parse(rc.URL)
	if err != nil {
		return nil
	}
	return parse
}
