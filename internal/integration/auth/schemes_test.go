package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/httpx"
	"github.com/coldsmirk/vef-framework-go/integration"
)

// applyScheme runs one request through a client configured by the scheme and
// returns what the server observed.
func applyScheme(t *testing.T, scheme integration.AuthScheme, params map[string]string) *http.Request {
	t.Helper()

	var observed *http.Request

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clone := *r
		clone.URL = r.URL
		observed = &clone

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	opts, err := scheme.Apply(params)
	require.NoError(t, err, "Scheme should accept its params")

	client, err := httpx.New(append(opts, httpx.WithBaseURL(srv.URL))...)
	require.NoError(t, err, "Client construction should succeed")

	_, err = client.NewRequest().Get(t.Context(), "/probe")
	require.NoError(t, err, "Probe request should succeed")
	require.NotNil(t, observed, "Server should observe the request")

	return observed
}

func TestBuiltinSchemes(t *testing.T) {
	registry := NewRegistry(nil)

	t.Run("None", func(t *testing.T) {
		scheme, ok := registry.Get(SchemeNone)
		require.True(t, ok, "none scheme should be registered")

		req := applyScheme(t, scheme, nil)
		assert.Empty(t, req.Header.Get("Authorization"), "none scheme should send no credentials")
		assert.Empty(t, scheme.SensitiveParams(), "none scheme should declare no sensitive params")
	})

	t.Run("Basic", func(t *testing.T) {
		scheme, ok := registry.Get(SchemeBasic)
		require.True(t, ok, "basic scheme should be registered")

		req := applyScheme(t, scheme, map[string]string{"username": "u", "password": "p"})
		user, pass, hasAuth := req.BasicAuth()
		require.True(t, hasAuth, "basic scheme should send Basic credentials")
		assert.Equal(t, "u", user, "Username should reach the server")
		assert.Equal(t, "p", pass, "Password should reach the server")
		assert.Equal(t, []string{"password"}, scheme.SensitiveParams(), "Only the password should be sensitive")
	})

	t.Run("Bearer", func(t *testing.T) {
		scheme, ok := registry.Get(SchemeBearer)
		require.True(t, ok, "bearer scheme should be registered")

		req := applyScheme(t, scheme, map[string]string{"token": "tok"})
		assert.Equal(t, "Bearer tok", req.Header.Get("Authorization"), "Bearer token should reach the server")
		assert.Equal(t, []string{"token"}, scheme.SensitiveParams(), "The token should be sensitive")
	})

	t.Run("Header", func(t *testing.T) {
		scheme, ok := registry.Get(SchemeHeader)
		require.True(t, ok, "header scheme should be registered")

		req := applyScheme(t, scheme, map[string]string{"name": "X-Api-Key", "value": "k"})
		assert.Equal(t, "k", req.Header.Get("X-Api-Key"), "Credential header should reach the server")
		assert.Equal(t, []string{"value"}, scheme.SensitiveParams(), "The header value should be sensitive")
	})

	t.Run("Query", func(t *testing.T) {
		scheme, ok := registry.Get(SchemeQuery)
		require.True(t, ok, "query scheme should be registered")

		req := applyScheme(t, scheme, map[string]string{"name": "apikey", "value": "k"})
		assert.Equal(t, "k", req.URL.Query().Get("apikey"), "Credential query parameter should reach the server")
		assert.Equal(t, []string{"value"}, scheme.SensitiveParams(), "The parameter value should be sensitive")
	})
}

func TestSchemeParamValidation(t *testing.T) {
	registry := NewRegistry(nil)

	tests := []struct {
		scheme string
		params map[string]string
	}{
		{scheme: SchemeBasic, params: map[string]string{"password": "p"}},
		{scheme: SchemeBasic, params: map[string]string{"username": "u"}},
		{scheme: SchemeBearer, params: nil},
		{scheme: SchemeHeader, params: map[string]string{"name": "X"}},
		{scheme: SchemeQuery, params: map[string]string{"value": "v"}},
	}

	for _, tt := range tests {
		t.Run(tt.scheme, func(t *testing.T) {
			scheme, ok := registry.Get(tt.scheme)
			require.True(t, ok, "Scheme should be registered")

			_, err := scheme.Apply(tt.params)
			require.Error(t, err, "Missing required param should be rejected")
			assert.ErrorIs(t, err, ErrMissingParam, "Error should be the missing-param sentinel")
		})
	}
}

// OverrideScheme replaces the built-in bearer scheme in registry tests.
type OverrideScheme struct{}

func (*OverrideScheme) Name() string { return SchemeBearer }

func (*OverrideScheme) Apply(map[string]string) ([]httpx.Option, error) { return nil, nil }

func (*OverrideScheme) SensitiveParams() []string { return nil }

func TestRegistry(t *testing.T) {
	t.Run("ResolveNilAuthYieldsNone", func(t *testing.T) {
		scheme, ok := NewRegistry(nil).Resolve(nil)
		require.True(t, ok, "Nil auth should resolve")
		assert.Equal(t, SchemeNone, scheme.Name(), "Nil auth should resolve to the none scheme")
	})

	t.Run("ResolveEmptySchemeYieldsNone", func(t *testing.T) {
		scheme, ok := NewRegistry(nil).Resolve(&integration.AuthConfig{})
		require.True(t, ok, "Empty scheme should resolve")
		assert.Equal(t, SchemeNone, scheme.Name(), "Empty scheme should resolve to the none scheme")
	})

	t.Run("UnknownSchemeReportsNotOK", func(t *testing.T) {
		_, ok := NewRegistry(nil).Resolve(&integration.AuthConfig{Scheme: "kerberos"})
		assert.False(t, ok, "Unknown scheme should not resolve")
	})

	t.Run("ApplicationSchemeOverridesBuiltin", func(t *testing.T) {
		registry := NewRegistry([]integration.AuthScheme{new(OverrideScheme)})

		scheme, ok := registry.Get(SchemeBearer)
		require.True(t, ok, "Overridden scheme should stay registered")
		assert.IsType(t, new(OverrideScheme), scheme, "Application scheme should replace the built-in by name")
	})
}
