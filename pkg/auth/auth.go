// Package auth provides authentication support for HTTP requests.
//
//go:generate mockgen -destination=./mocks/auth.go . Authenticator
package auth

import "net/http"

// Authenticator defines the interface for applying authentication to HTTP requests.
type Authenticator interface {
	Apply(req *http.Request) error
	Type() Type
}

// BasicAuth represents HTTP Basic Authentication credentials.
type BasicAuth struct {
	Username string
	Password string
}

// HeaderAuth represents authentication via custom HTTP headers.
type HeaderAuth struct {
	Headers map[string]string
}

// BearerAuth represents Bearer token authentication.
type BearerAuth struct {
	Token string
}

// Type represents the type of authentication.
type Type string

// Authentication types.
const (
	// BasicAuthType represents HTTP Basic Authentication.
	BasicAuthType Type = "basic"
	// HeaderAuthType represents custom header-based authentication.
	HeaderAuthType Type = "header"
	// BearerAuthType represents Bearer token authentication.
	BearerAuthType Type = "bearer"
)

// Apply adds Basic Authentication headers to the HTTP request.
func (b BasicAuth) Apply(req *http.Request) error {
	req.SetBasicAuth(b.Username, b.Password)
	return nil
}

// Type returns the authentication type (BasicAuthType).
func (b BasicAuth) Type() Type { return BasicAuthType }

// Apply adds custom headers to the HTTP request.
func (h HeaderAuth) Apply(req *http.Request) error {
	for k, v := range h.Headers {
		req.Header.Set(k, v)
	}
	return nil
}

// Type returns the authentication type (HeaderAuthType).
func (h HeaderAuth) Type() Type { return HeaderAuthType }

// Apply adds a Bearer token to the Authorization header of the HTTP request.
func (b BearerAuth) Apply(req *http.Request) error {
	req.Header.Set("Authorization", "Bearer "+b.Token)
	return nil
}

// Type returns the authentication type (BearerAuthType).
func (b BearerAuth) Type() Type { return BearerAuthType }
