package index

import "time"

type Index struct {
	FormatVersion string     `json:"format_version"`
	LastUpdate    time.Time  `json:"last_update"`
	Packages      []*Package `json:"packages"`
}

// Info represents index information.
type Info struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Enabled  bool   `json:"enabled"`
	Priority int    `json:"priority"`
}

// Package represents a pkg in a index.
type Package struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description"`
	URL          string            `json:"url"`
	Checksum     string            `json:"checksum"`
	Size         int64             `json:"size"`
	OS           string            `json:"os,omitempty"`
	Arch         string            `json:"arch,omitempty"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}
