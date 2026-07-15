package auth

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/security"
)

// stubAPIKeyLoader is a test double for security.APIKeyLoader.
type stubAPIKeyLoader struct {
	principal *security.Principal
	err       error
	seenKey   string
}

func (s *stubAPIKeyLoader) LoadByKey(_ context.Context, key string) (*security.Principal, error) {
	s.seenKey = key

	return s.principal, s.err
}

// runAPIKeyRequest exercises the strategy through a real Fiber request with
// the given headers.
func runAPIKeyRequest(t *testing.T, strategy api.AuthStrategy, options map[string]any, headers map[string]string) (*security.Principal, error) {
	t.Helper()

	var (
		gotPrincipal *security.Principal
		gotErr       error
	)

	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		gotPrincipal, gotErr = strategy.Authenticate(c, options)

		return nil
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	_, err := app.Test(req)
	require.NoError(t, err, "Fiber test request should not fail")

	return gotPrincipal, gotErr
}

func newAPIKeyStrategy(t *testing.T, loader security.APIKeyLoader) api.AuthStrategy {
	t.Helper()

	strategy, err := NewAPIKey(loader, new(config.SecurityConfig))
	require.NoError(t, err, "NewAPIKey with an explicit loader should not fail")

	return strategy
}

func TestAPIKeyStrategy(t *testing.T) {
	t.Run("Name", func(t *testing.T) {
		strategy := newAPIKeyStrategy(t, &stubAPIKeyLoader{})
		assert.Equal(t, api.AuthStrategyAPIKey, strategy.Name(), "Strategy must register under the api_key name")
	})

	t.Run("DefaultHeaderResolvesPrincipal", func(t *testing.T) {
		want := security.NewExternalApp("api_key:reporting", "reporting")
		loader := &stubAPIKeyLoader{principal: want}

		principal, err := runAPIKeyRequest(t, newAPIKeyStrategy(t, loader), nil,
			map[string]string{api.HeaderXAPIKey: "sk-123"})
		require.NoError(t, err, "A resolvable key should authenticate")
		assert.Same(t, want, principal, "The loader's principal should be returned")
		assert.Equal(t, "sk-123", loader.seenKey, "The presented key should be passed to the loader verbatim")
	})

	t.Run("CustomHeaderOption", func(t *testing.T) {
		loader := &stubAPIKeyLoader{principal: security.NewExternalApp("api_key:x", "x")}
		options := api.APIKeyAuth("X-Vendor-Token").Options

		_, err := runAPIKeyRequest(t, newAPIKeyStrategy(t, loader), options,
			map[string]string{"X-Vendor-Token": "vt-9"})
		require.NoError(t, err, "The key should be read from the configured header")
		assert.Equal(t, "vt-9", loader.seenKey, "The custom header value should reach the loader")
	})

	t.Run("MissingHeaderDenies", func(t *testing.T) {
		loader := &stubAPIKeyLoader{principal: security.NewExternalApp("api_key:x", "x")}

		principal, err := runAPIKeyRequest(t, newAPIKeyStrategy(t, loader), nil, nil)
		require.ErrorIs(t, err, security.ErrAPIKeyInvalid, "A missing key must deny fail-closed")
		assert.Nil(t, principal, "No principal on denial")
		assert.Empty(t, loader.seenKey, "The loader must not be consulted without a presented key")
	})

	t.Run("UnmatchedKeyDenies", func(t *testing.T) {
		principal, err := runAPIKeyRequest(t, newAPIKeyStrategy(t, &stubAPIKeyLoader{}), nil,
			map[string]string{api.HeaderXAPIKey: "sk-wrong"})
		require.ErrorIs(t, err, security.ErrAPIKeyInvalid, "An unmatched key must deny with the uniform sentinel")
		assert.Nil(t, principal, "No principal on denial")
	})

	t.Run("LoaderErrorPropagates", func(t *testing.T) {
		wantErr := errors.New("store unavailable")

		_, err := runAPIKeyRequest(t, newAPIKeyStrategy(t, &stubAPIKeyLoader{err: wantErr}), nil,
			map[string]string{api.HeaderXAPIKey: "sk-123"})
		require.ErrorIs(t, err, wantErr, "Infrastructure faults must propagate, not masquerade as 401")
	})

	t.Run("NilLoaderFallsBackToConfig", func(t *testing.T) {
		strategy, err := NewAPIKey(nil, &config.SecurityConfig{
			APIKeys: map[string]config.APIKeyConfig{"ops": {Key: "sk-ops"}},
		})
		require.NoError(t, err, "NewAPIKey should build the config-backed loader when none is registered")

		principal, err := runAPIKeyRequest(t, strategy, nil, map[string]string{api.HeaderXAPIKey: "sk-ops"})
		require.NoError(t, err, "The config-backed key should authenticate")
		require.NotNil(t, principal, "The config-backed key should resolve a principal")
		assert.Equal(t, "api_key:ops", principal.ID, "The synthesized principal should carry the key name")
	})

	t.Run("InvalidConfigFailsConstruction", func(t *testing.T) {
		_, err := NewAPIKey(nil, &config.SecurityConfig{
			APIKeys: map[string]config.APIKeyConfig{"ops": {}},
		})
		require.Error(t, err, "A blank configured key must fail strategy construction")
	})
}
