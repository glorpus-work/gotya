package index

import "net/url"

// Repository represents a package repository with a name, URL, priority, and enabled status.
type Repository struct {
	Name     string
	URL      *url.URL
	Priority uint
	Enabled  bool
}
