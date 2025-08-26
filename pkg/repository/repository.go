package repository

import "net/url"

type Repository struct {
	Name     string
	Url      *url.URL
	Priority uint
	Enabled  bool
}
