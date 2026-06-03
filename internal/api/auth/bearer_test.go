package auth

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/security"
)

// stubAuthenticator is a test double for TokenAuthenticator.
type stubAuthenticator struct {
	principal *security.Principal
	err       error
}

func (s *stubAuthenticator) Authenticate(context.Context, string) (*security.Principal, error) {
	return s.principal, s.err
}

// runBearerRequest exercises the BearerStrategy by sending an HTTP request
// with the given token in the Authorization header and returning the strategy
// result.
func runBearerRequest(t *testing.T, strategy *BearerStrategy, token string) (*security.Principal, error) {
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

	header := ""
	if token != "" {
		header = "Bearer " + token
	}

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}

	_, err := app.Test(req)
	require.NoError(t, err, "Fiber test request should not fail")

	return gotPrincipal, gotErr
}

// TestBearerStrategyName verifies the strategy identifier.
func TestBearerStrategyName(t *testing.T) {
	s := NewBearer(nil)
	assert.Equal(t, "bearer", s.Name(), "Strategy name should be 'bearer'")
}

// TestBearerStrategyAuthenticate covers the authentication scenarios.
func TestBearerStrategyAuthenticate(t *testing.T) {
	user := security.NewUser("u1", "Alice")

	t.Run("AuthenticatorSucceeds", func(t *testing.T) {
		stub := &stubAuthenticator{principal: user}
		strategy := NewBearer([]TokenAuthenticator{stub}).(*BearerStrategy)

		principal, err := runBearerRequest(t, strategy, "valid-token")
		require.NoError(t, err, "Authenticate should not return an error when authenticator succeeds")
		assert.Equal(t, user, principal, "Authenticate should return the principal from the authenticator")
	})

	t.Run("AuthenticatorReturnsError", func(t *testing.T) {
		authErr := errors.New("auth backend unavailable")
		stub := &stubAuthenticator{err: authErr}
		strategy := NewBearer([]TokenAuthenticator{stub}).(*BearerStrategy)

		_, err := runBearerRequest(t, strategy, "some-token")
		require.Error(t, err, "Authenticate should propagate authenticator errors")
		assert.True(t, errors.Is(err, authErr), "Propagated error should wrap the authenticator error")
	})

	t.Run("AllAuthenticatorsReturnNilPrincipal401", func(t *testing.T) {
		// All authenticators signal 'not my token' via (nil, nil). The strategy
		// must return a 401-mapped error (security.ErrTokenInvalid), not a generic 500.
		stub := &stubAuthenticator{principal: nil, err: nil}
		strategy := NewBearer([]TokenAuthenticator{stub}).(*BearerStrategy)

		_, err := runBearerRequest(t, strategy, "unrecognized-token")
		require.Error(t, err, "Authenticate should return an error when all authenticators return nil principal")
		assert.ErrorIs(t, err, security.ErrTokenInvalid, "Error should be security.ErrTokenInvalid (401)")
	})

	t.Run("NoAuthenticatorsReturns401", func(t *testing.T) {
		// With an empty authenticator list the fallback must still be 401.
		strategy := NewBearer(nil).(*BearerStrategy)

		_, err := runBearerRequest(t, strategy, "any-token")
		require.Error(t, err, "Authenticate should return an error with no authenticators")
		assert.ErrorIs(t, err, security.ErrTokenInvalid, "Error should be security.ErrTokenInvalid (401)")
	})

	t.Run("MissingTokenReturnsUnauthorized", func(t *testing.T) {
		strategy := NewBearer(nil).(*BearerStrategy)

		// No Authorization header → extractor returns ErrNotFound → wrapExtractError → fiber.ErrUnauthorized
		_, err := runBearerRequest(t, strategy, "")
		require.Error(t, err, "Authenticate should return an error when token is missing")
	})

	t.Run("FirstAuthenticatorSucceedsSecondSkipped", func(t *testing.T) {
		first := &stubAuthenticator{principal: user}
		second := &stubAuthenticator{err: errors.New("should not be called")}
		strategy := NewBearer([]TokenAuthenticator{first, second}).(*BearerStrategy)

		principal, err := runBearerRequest(t, strategy, "token")
		require.NoError(t, err, "Authenticate should succeed when first authenticator matches")
		assert.Equal(t, user, principal, "Should return the principal from the first authenticator")
	})
}
