// Package config provides configuration structures and utilities for the application.
package config

import "github.com/glorpus-work/gotya/pkg/auth"

// AuthConfigContainer defines the interface for authentication configuration types that can be converted to an Authenticator.
type AuthConfigContainer interface {
	ToAuthenticator() auth.Authenticator
}

// AuthConfig holds various authentication configurations for a repository.
type AuthConfig struct {
	BasicAuth  *BasicAuth  `yaml:"basic,omitempty"`
	HeaderAuth *HeaderAuth `yaml:"header,omitempty"`
	BearerAuth *BearerAuth `yaml:"bearer,omitempty"`
}

// BasicAuth holds configuration for HTTP Basic Authentication.
type BasicAuth struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// HeaderAuth holds configuration for custom header-based authentication.
type HeaderAuth struct {
	Headers map[string]string `yaml:"headers"`
}

// BearerAuth holds configuration for Bearer token authentication.
type BearerAuth struct {
	Token string `yaml:"token"`
}

// ToAuthenticator converts the BasicAuth configuration to an Authenticator.
func (b *BasicAuth) ToAuthenticator() auth.Authenticator {
	return &auth.BasicAuth{
		Username: b.Username,
		Password: b.Password,
	}
}

// ToAuthenticator converts the HeaderAuth configuration to an Authenticator.
func (h *HeaderAuth) ToAuthenticator() auth.Authenticator {
	return &auth.HeaderAuth{
		Headers: h.Headers,
	}
}

// ToAuthenticator converts the BearerAuth configuration to an Authenticator.
func (b *BearerAuth) ToAuthenticator() auth.Authenticator {
	return &auth.BearerAuth{
		Token: b.Token,
	}
}

// ToAuthMap converts the repository authentication configurations to a map of repository names to Authenticators.
// Returns nil if no authentication configurations are found.
func (c *Config) ToAuthMap() map[string]auth.Authenticator {
	results := make(map[string]auth.Authenticator, len(c.Repositories))
	for _, repo := range c.Repositories {
		if repo.Auth == nil {
			continue
		}
		switch {
		case repo.Auth.BasicAuth != nil:
			results[repo.Name] = repo.Auth.BasicAuth.ToAuthenticator()
		case repo.Auth.HeaderAuth != nil:
			results[repo.Name] = repo.Auth.HeaderAuth.ToAuthenticator()
		case repo.Auth.BearerAuth != nil:
			results[repo.Name] = repo.Auth.BearerAuth.ToAuthenticator()
		default:
			return nil
		}

	}

	if len(results) == 0 {
		return nil
	}
	return results
}
