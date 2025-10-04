package auth_test

import (
	"net/http"
	"testing"

	"github.com/glorpus-work/gotya/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasicAuth(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		expected string
	}{
		{
			name:     "valid credentials",
			username: "user",
			password: "pass",
			expected: "Basic dXNlcjpwYXNz", // base64("user:pass")
		},
		{
			name:     "empty credentials",
			username: "",
			password: "",
			expected: "Basic Og==", // base64(":")
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://example.com", nil)
			basicAuth := auth.BasicAuth{
				Username: tt.username,
				Password: tt.password,
			}

			err := basicAuth.Apply(req)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, req.Header.Get("Authorization"))
			assert.Equal(t, auth.BasicAuthType, basicAuth.Type())
		})
	}
}

func TestHeaderAuth(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		expect  map[string]string
	}{
		{
			name: "single header",
			headers: map[string]string{
				"X-API-Key": "test-key",
			},
			expect: map[string]string{
				"X-Api-Key": "test-key", // http.Header canonicalizes headers
			},
		},
		{
			name: "multiple headers",
			headers: map[string]string{
				"X-API-Key":    "test-key",
				"X-Client-ID":  "client-123",
				"X-API-Secret": "secret-456",
			},
			expect: map[string]string{
				"X-Api-Key":    "test-key",
				"X-Client-Id":  "client-123",
				"X-Api-Secret": "secret-456",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://example.com", nil)
			headerAuth := auth.HeaderAuth{
				Headers: tt.headers,
			}

			err := headerAuth.Apply(req)
			require.NoError(t, err)

			for k, v := range tt.expect {
				assert.Equal(t, v, req.Header.Get(k))
			}
			assert.Equal(t, auth.HeaderAuthType, headerAuth.Type())
		})
	}
}

func TestBearerAuth(t *testing.T) {
	tests := []struct {
		name   string
		token  string
		expect string
	}{
		{
			name:   "valid token",
			token:  "test-token-123",
			expect: "Bearer test-token-123",
		},
		{
			name:   "empty token",
			token:  "",
			expect: "Bearer ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://example.com", nil)
			bearerAuth := auth.BearerAuth{
				Token: tt.token,
			}

			err := bearerAuth.Apply(req)
			require.NoError(t, err)
			assert.Equal(t, tt.expect, req.Header.Get("Authorization"))
			assert.Equal(t, auth.BearerAuthType, bearerAuth.Type())
		})
	}
}
