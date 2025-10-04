package config

import (
	"testing"

	"github.com/glorpus-work/gotya/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to compare authenticators by their string representation
func authenticatorsEqual(a, b auth.Authenticator) bool {
	switch aVal := a.(type) {
	case *auth.BasicAuth:
		bVal, ok := b.(*auth.BasicAuth)
		if !ok {
			return false
		}
		return aVal.Username == bVal.Username && aVal.Password == bVal.Password
	case *auth.HeaderAuth:
		bVal, ok := b.(*auth.HeaderAuth)
		if !ok {
			return false
		}
		if len(aVal.Headers) != len(bVal.Headers) {
			return false
		}
		for k, v := range aVal.Headers {
			if bVal.Headers[k] != v {
				return false
			}
		}
		return true
	case *auth.BearerAuth:
		bVal, ok := b.(*auth.BearerAuth)
		if !ok {
			return false
		}
		return aVal.Token == bVal.Token
	default:
		return false
	}
}

func TestToAuthMap(t *testing.T) {
	tests := []struct {
		name     string
		repos    []*RepositoryConfig
		expected map[string]auth.Authenticator
	}{
		{
			name:     "no repositories",
			repos:    []*RepositoryConfig{},
			expected: nil,
		},
		{
			name: "repository with no auth",
			repos: []*RepositoryConfig{
				{
					URL: "https://example.com/repo",
				},
			},
			expected: nil,
		},
		{
			name: "repository with basic auth",
			repos: []*RepositoryConfig{
				{
					URL: "https://example.com/repo",
					Auth: &AuthConfig{
						BasicAuth: &BasicAuth{
							Username: "user",
							Password: "pass",
						},
					},
				},
			},
			expected: map[string]auth.Authenticator{
				"https://example.com/repo": &auth.BasicAuth{
					Username: "user",
					Password: "pass",
				},
			},
		},
		{
			name: "repository with header auth",
			repos: []*RepositoryConfig{
				{
					URL: "https://example.com/repo",
					Auth: &AuthConfig{
						HeaderAuth: &HeaderAuth{
							Headers: map[string]string{
								"X-API-Key": "secret-key",
							},
						},
					},
				},
			},
			expected: map[string]auth.Authenticator{
				"https://example.com/repo": &auth.HeaderAuth{
					Headers: map[string]string{
						"X-API-Key": "secret-key",
					},
				},
			},
		},
		{
			name: "repository with bearer auth",
			repos: []*RepositoryConfig{
				{
					URL: "https://example.com/repo",
					Auth: &AuthConfig{
						BearerAuth: &BearerAuth{
							Token: "token123",
						},
					},
				},
			},
			expected: map[string]auth.Authenticator{
				"https://example.com/repo": &auth.BearerAuth{
					Token: "token123",
				},
			},
		},
		{
			name: "multiple repositories with different auth types",
			repos: []*RepositoryConfig{
				{
					URL: "https://example.com/repo1",
					Auth: &AuthConfig{
						BasicAuth: &BasicAuth{
							Username: "user1",
							Password: "pass1",
						},
					},
				},
				{
					URL: "https://example.com/repo2",
					Auth: &AuthConfig{
						BearerAuth: &BearerAuth{
							Token: "token123",
						},
					},
				},
			},
			expected: map[string]auth.Authenticator{
				"https://example.com/repo1": &auth.BasicAuth{
					Username: "user1",
					Password: "pass1",
				},
				"https://example.com/repo2": &auth.BearerAuth{
					Token: "token123",
				},
			},
		},
		{
			name: "repository with index.json URL",
			repos: []*RepositoryConfig{
				{
					URL: "https://example.com/repo/index.json",
					Auth: &AuthConfig{
						BasicAuth: &BasicAuth{
							Username: "user",
							Password: "pass",
						},
					},
				},
			},
			expected: map[string]auth.Authenticator{
				"https://example.com/repo": &auth.BasicAuth{
					Username: "user",
					Password: "pass",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Repositories: tt.repos,
			}

			result := cfg.ToAuthMap()

			if tt.expected == nil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			assert.Len(t, result, len(tt.expected), "number of authenticators should match")

			// Compare each expected authenticator
			for url, expectedAuth := range tt.expected {
				actualAuth, exists := result[url]
				assert.True(t, exists, "expected URL %s in result", url)
				assert.True(t, authenticatorsEqual(expectedAuth, actualAuth),
					"authenticator for %s should match. Expected: %+v, Got: %+v",
					url, expectedAuth, actualAuth)
			}
		})
	}
}
