package config

import "net/url"

func (rc *RepositoryConfig) GetURL() *url.URL {
	parse, err := url.Parse(rc.URL)
	if err != nil {
		return nil
	}
	return parse
}
