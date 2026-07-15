package auth

import (
	"context"
	"encoding/base64"
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

// stubBasicAccountLoader is a test double for security.BasicAccountLoader.
type stubBasicAccountLoader struct {
	principal    *security.Principal
	secret       string
	err          error
	seenUsername string
}

func (s *stubBasicAccountLoader) LoadByUsername(_ context.Context, username string) (*security.Principal, string, error) {
	s.seenUsername = username

	return s.principal, s.secret, s.err
}

// runBasicRequest exercises the strategy through a real Fiber request with
// the given Authorization header value ("" sends no header).
func runBasicRequest(t *testing.T, strategy api.AuthStrategy, authorization string) (*security.Principal, error) {
	t.Helper()

	var (
		gotPrincipal *security.Principal
		gotErr       error
	)

	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		gotPrincipal, gotErr = strategy.Authenticate(c, nil)

		return nil
	})

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	if authorization != "" {
		req.Header.Set(fiber.HeaderAuthorization, authorization)
	}

	_, err := app.Test(req)
	require.NoError(t, err, "Fiber test request should not fail")

	return gotPrincipal, gotErr
}

func newBasicStrategy(t *testing.T, loader security.BasicAccountLoader) api.AuthStrategy {
	t.Helper()

	strategy, err := NewHTTPBasic(loader, new(config.SecurityConfig))
	require.NoError(t, err, "NewHTTPBasic with an explicit loader should not fail")

	return strategy
}

func basicHeader(username, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
}

func TestHTTPBasicStrategy(t *testing.T) {
	t.Run("Name", func(t *testing.T) {
		strategy := newBasicStrategy(t, &stubBasicAccountLoader{})
		assert.Equal(t, api.AuthStrategyHTTPBasic, strategy.Name(), "Strategy must register under the http_basic name")
	})

	t.Run("ValidCredentialsResolvePrincipal", func(t *testing.T) {
		want := security.NewExternalApp("http_basic:svc", "svc")
		loader := &stubBasicAccountLoader{principal: want, secret: "s3cret"}

		principal, err := runBasicRequest(t, newBasicStrategy(t, loader), basicHeader("svc", "s3cret"))
		require.NoError(t, err, "Matching credentials should authenticate")
		assert.Same(t, want, principal, "The loader's principal should be returned")
		assert.Equal(t, "svc", loader.seenUsername, "The decoded username should reach the loader")
	})

	t.Run("SchemeIsCaseInsensitive", func(t *testing.T) {
		loader := &stubBasicAccountLoader{principal: security.NewExternalApp("http_basic:svc", "svc"), secret: "s3cret"}
		header := "basic " + base64.StdEncoding.EncodeToString([]byte("svc:s3cret"))

		_, err := runBasicRequest(t, newBasicStrategy(t, loader), header)
		require.NoError(t, err, "The Basic scheme comparison must be case-insensitive per RFC 7617")
	})

	t.Run("PasswordWithColonSurvivesDecoding", func(t *testing.T) {
		loader := &stubBasicAccountLoader{principal: security.NewExternalApp("http_basic:svc", "svc"), secret: "a:b:c"}

		_, err := runBasicRequest(t, newBasicStrategy(t, loader), basicHeader("svc", "a:b:c"))
		require.NoError(t, err, "Only the first colon separates username and password")
	})

	t.Run("WrongPasswordDenies", func(t *testing.T) {
		loader := &stubBasicAccountLoader{principal: security.NewExternalApp("http_basic:svc", "svc"), secret: "s3cret"}

		principal, err := runBasicRequest(t, newBasicStrategy(t, loader), basicHeader("svc", "wrong"))
		require.ErrorIs(t, err, security.ErrBasicCredentialsInvalid, "A secret mismatch must deny fail-closed")
		assert.Nil(t, principal, "No principal on denial")
	})

	t.Run("UnknownAccountDenies", func(t *testing.T) {
		principal, err := runBasicRequest(t, newBasicStrategy(t, &stubBasicAccountLoader{}), basicHeader("ghost", "x"))
		require.ErrorIs(t, err, security.ErrBasicCredentialsInvalid,
			"An unknown account must deny with the same sentinel as a wrong password")
		assert.Nil(t, principal, "No principal on denial")
	})

	t.Run("MalformedHeadersDeny", func(t *testing.T) {
		loader := &stubBasicAccountLoader{principal: security.NewExternalApp("http_basic:svc", "svc"), secret: "s3cret"}
		strategy := newBasicStrategy(t, loader)

		for name, header := range map[string]string{
			"NoHeader":      "",
			"BearerScheme":  "Bearer abc",
			"NotBase64":     "Basic %%%",
			"NoColon":       "Basic " + base64.StdEncoding.EncodeToString([]byte("svconly")),
			"EmptyUsername": "Basic " + base64.StdEncoding.EncodeToString([]byte(":pass")),
			"SchemeOnly":    "Basic ",
		} {
			t.Run(name, func(t *testing.T) {
				_, err := runBasicRequest(t, strategy, header)
				require.ErrorIs(t, err, security.ErrBasicCredentialsInvalid, "Malformed credentials must deny fail-closed")
			})
		}
	})

	t.Run("LoaderErrorPropagates", func(t *testing.T) {
		wantErr := errors.New("store unavailable")

		_, err := runBasicRequest(t, newBasicStrategy(t, &stubBasicAccountLoader{err: wantErr}), basicHeader("svc", "x"))
		require.ErrorIs(t, err, wantErr, "Infrastructure faults must propagate, not masquerade as 401")
	})

	t.Run("NilLoaderFallsBackToConfig", func(t *testing.T) {
		strategy, err := NewHTTPBasic(nil, &config.SecurityConfig{
			BasicAccounts: map[string]config.BasicAccountConfig{"svc": {Password: "s3cret"}},
		})
		require.NoError(t, err, "NewHTTPBasic should build the config-backed loader when none is registered")

		principal, err := runBasicRequest(t, strategy, basicHeader("svc", "s3cret"))
		require.NoError(t, err, "The config-backed account should authenticate")
		require.NotNil(t, principal, "The config-backed account should resolve a principal")
		assert.Equal(t, "http_basic:svc", principal.ID, "The synthesized principal should carry the username")
	})

	t.Run("InvalidConfigFailsConstruction", func(t *testing.T) {
		_, err := NewHTTPBasic(nil, &config.SecurityConfig{
			BasicAccounts: map[string]config.BasicAccountConfig{"svc": {}},
		})
		require.Error(t, err, "A blank configured password must fail strategy construction")
	})
}
