package repository

import "net/url"

type Repository struct {
	Name     string
	URL      *url.URL
	Priority uint
	Enabled  bool
}
