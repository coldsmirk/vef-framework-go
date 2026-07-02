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

// stubIPWhitelistLoader is a test double for security.IPWhitelistLoader.
type stubIPWhitelistLoader struct {
	whitelist *security.IPWhitelist
	err       error
}

func (s *stubIPWhitelistLoader) LoadByName(context.Context, string) (*security.IPWhitelist, error) {
	return s.whitelist, s.err
}

// runIPRequest exercises the strategy through a real Fiber request; under
// app.Test the resolved client IP is "0.0.0.0".
func runIPRequest(t *testing.T, strategy api.AuthStrategy, options map[string]any) (*security.Principal, error) {
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
	_, err := app.Test(req)
	require.NoError(t, err, "Fiber test request should not fail")

	return gotPrincipal, gotErr
}

func newIPStrategy(t *testing.T, loader security.IPWhitelistLoader) api.AuthStrategy {
	t.Helper()

	strategy, err := NewIP(loader, new(config.SecurityConfig))
	require.NoError(t, err, "NewIP with an explicit loader should not fail")

	return strategy
}

func whitelistOptions(name string) map[string]any {
	return map[string]any{api.AuthOptionWhitelist: name}
}

// TestIPStrategyName verifies the strategy identifier.
func TestIPStrategyName(t *testing.T) {
	s := newIPStrategy(t, &stubIPWhitelistLoader{})
	assert.Equal(t, "ip", s.Name(), "Strategy name should be 'ip'")
}

// TestNewIP covers loader selection and eager config validation.
func TestNewIP(t *testing.T) {
	t.Run("NilLoaderFallsBackToConfig", func(t *testing.T) {
		cfg := &config.SecurityConfig{IPWhitelists: map[string][]string{
			"internal": {"0.0.0.0"},
		}}

		strategy, err := NewIP(nil, cfg)
		require.NoError(t, err, "A valid ip_whitelists config should construct the default loader")

		principal, authErr := runIPRequest(t, strategy, whitelistOptions("internal"))
		require.NoError(t, authErr, "The config-backed loader should authenticate a whitelisted IP")
		assert.Equal(t, "ip:internal", principal.ID, "Principal must come from the config-backed whitelist")
	})

	t.Run("NilLoaderInvalidConfigFailsConstruction", func(t *testing.T) {
		cfg := &config.SecurityConfig{IPWhitelists: map[string][]string{
			"internal": {"not-an-ip"},
		}}

		_, err := NewIP(nil, cfg)
		require.Error(t, err, "An invalid ip_whitelists config must fail construction (start-up)")
	})

	t.Run("CustomLoaderSkipsConfigValidation", func(t *testing.T) {
		cfg := &config.SecurityConfig{IPWhitelists: map[string][]string{
			"internal": {"not-an-ip"},
		}}

		_, err := NewIP(&stubIPWhitelistLoader{}, cfg)
		require.NoError(t, err, "A custom loader replaces the config loader entirely, so config faults are irrelevant")
	})
}

// TestIPStrategyAuthenticate covers the fail-closed matching behavior.
func TestIPStrategyAuthenticate(t *testing.T) {
	t.Run("AllowedIPReturnsSynthesizedPrincipal", func(t *testing.T) {
		loader := &stubIPWhitelistLoader{whitelist: &security.IPWhitelist{Entries: []string{"0.0.0.0"}}}
		s := newIPStrategy(t, loader)

		principal, err := runIPRequest(t, s, whitelistOptions("internal"))
		require.NoError(t, err, "A whitelisted IP should authenticate")
		require.NotNil(t, principal, "A whitelisted IP should yield a principal")
		assert.Equal(t, security.PrincipalTypeExternalApp, principal.Type, "The synthesized principal is an external app, never system")
		assert.Equal(t, "ip:internal", principal.ID, "Principal ID should embed the whitelist name")
		assert.Equal(t, "internal", principal.Name, "Principal name should be the whitelist name")
	})

	t.Run("AllowedCIDRReturnsPrincipal", func(t *testing.T) {
		loader := &stubIPWhitelistLoader{whitelist: &security.IPWhitelist{Entries: []string{"0.0.0.0/8"}}}
		s := newIPStrategy(t, loader)

		principal, err := runIPRequest(t, s, whitelistOptions("internal"))
		require.NoError(t, err, "An IP inside a whitelisted CIDR should authenticate")
		require.NotNil(t, principal, "An IP inside a whitelisted CIDR should yield a principal")
	})

	t.Run("UnlistedIPDenied", func(t *testing.T) {
		loader := &stubIPWhitelistLoader{whitelist: &security.IPWhitelist{Entries: []string{"10.0.0.1"}}}
		s := newIPStrategy(t, loader)

		_, err := runIPRequest(t, s, whitelistOptions("internal"))
		assert.ErrorIs(t, err, security.ErrIPNotAllowed, "An unlisted client IP must be denied")
	})

	t.Run("UnknownWhitelistDenied", func(t *testing.T) {
		s := newIPStrategy(t, &stubIPWhitelistLoader{whitelist: nil})

		_, err := runIPRequest(t, s, whitelistOptions("missing"))
		assert.ErrorIs(t, err, security.ErrIPNotAllowed, "A whitelist the loader cannot resolve must deny, not allow")
	})

	t.Run("EmptyEntriesDenied", func(t *testing.T) {
		// An empty whitelist would make the validator allow-all; the strategy
		// must fail closed instead of turning the endpoint public.
		loader := &stubIPWhitelistLoader{whitelist: &security.IPWhitelist{}}
		s := newIPStrategy(t, loader)

		_, err := runIPRequest(t, s, whitelistOptions("internal"))
		assert.ErrorIs(t, err, security.ErrIPNotAllowed, "A whitelist with no entries must deny every IP")
	})

	t.Run("BlankEntriesDenied", func(t *testing.T) {
		loader := &stubIPWhitelistLoader{whitelist: &security.IPWhitelist{Entries: []string{"", "   "}}}
		s := newIPStrategy(t, loader)

		_, err := runIPRequest(t, s, whitelistOptions("internal"))
		assert.ErrorIs(t, err, security.ErrIPNotAllowed, "A whitelist of blank entries must deny every IP")
	})

	t.Run("InvalidEntriesDenied", func(t *testing.T) {
		// A dynamic loader may return malformed entries; the validator treats
		// any parse failure as deny-all and the strategy must surface that.
		loader := &stubIPWhitelistLoader{whitelist: &security.IPWhitelist{Entries: []string{"0.0.0.0", "not-an-ip"}}}
		s := newIPStrategy(t, loader)

		_, err := runIPRequest(t, s, whitelistOptions("internal"))
		assert.ErrorIs(t, err, security.ErrIPNotAllowed, "Malformed loader entries must deny even a listed IP")
	})

	t.Run("MissingWhitelistOptionDenied", func(t *testing.T) {
		s := newIPStrategy(t, &stubIPWhitelistLoader{whitelist: &security.IPWhitelist{Entries: []string{"0.0.0.0"}}})

		_, err := runIPRequest(t, s, nil)
		assert.ErrorIs(t, err, security.ErrIPNotAllowed, "An operation that names no whitelist must deny")
	})

	t.Run("NonStringWhitelistOptionDenied", func(t *testing.T) {
		s := newIPStrategy(t, &stubIPWhitelistLoader{whitelist: &security.IPWhitelist{Entries: []string{"0.0.0.0"}}})

		_, err := runIPRequest(t, s, map[string]any{api.AuthOptionWhitelist: 42})
		assert.ErrorIs(t, err, security.ErrIPNotAllowed, "A non-string whitelist option must deny")
	})

	t.Run("LoaderErrorPropagates", func(t *testing.T) {
		loadErr := errors.New("backing store unavailable")
		s := newIPStrategy(t, &stubIPWhitelistLoader{err: loadErr})

		_, err := runIPRequest(t, s, whitelistOptions("internal"))
		require.Error(t, err, "A loader failure should surface")
		assert.ErrorIs(t, err, loadErr, "Loader errors are infrastructure faults and must propagate as-is")
	})

	t.Run("FreshPrincipalEachCall", func(t *testing.T) {
		loader := &stubIPWhitelistLoader{whitelist: &security.IPWhitelist{Entries: []string{"0.0.0.0"}}}
		s := newIPStrategy(t, loader)

		p1, err := runIPRequest(t, s, whitelistOptions("internal"))
		require.NoError(t, err, "First Authenticate should succeed")
		p2, err := runIPRequest(t, s, whitelistOptions("internal"))
		require.NoError(t, err, "Second Authenticate should succeed")

		assert.NotSame(t, p1, p2, "Authenticate must return a distinct principal per request to avoid shared-state corruption")
	})
}
