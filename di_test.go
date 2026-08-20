package vef

import (
	"context"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/app"
	iapp "github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/security"
)

// HostMiddleware is shaped like an application's own middleware: it names the
// public contract, which is all a host in another module can reach.
type HostMiddleware struct{}

func (*HostMiddleware) Name() string { return "host-middleware" }

func (*HostMiddleware) Order() int { return 460 }

func (*HostMiddleware) Apply(fiber.Router) {}

// HostAuthenticator is shaped like an application's own login mechanism.
type HostAuthenticator struct{}

func (*HostAuthenticator) Supports(authType string) bool { return authType == "host" }

func (*HostAuthenticator) Authenticate(context.Context, security.Authentication) (*security.Principal, error) {
	return nil, nil
}

// TestProvideMiddlewareReachesTheRouter pins the property a host depends on:
// the type ProvideMiddleware puts into the group is the very type internal/app
// collects. fx matches a group by exact type and drops a mismatch without an
// error, so a break here surfaces as a route that silently never registers —
// declaring the collector exactly as the framework does is what catches it.
func TestProvideMiddlewareReachesTheRouter(t *testing.T) {
	type collector struct {
		fx.In

		Middlewares []iapp.Middleware `group:"vef:app:middlewares"`
	}

	var collected []iapp.Middleware

	fxApp := fx.New(
		ProvideMiddleware(func() app.Middleware { return new(HostMiddleware) }),
		fx.Invoke(func(c collector) { collected = c.Middlewares }),
		fx.NopLogger,
	)

	require.NoError(t, fxApp.Err(), "The middleware graph should resolve")
	require.Len(t, collected, 1,
		"A middleware registered through the public contract must reach the group the router assembles from")
	require.Equal(t, "host-middleware", collected[0].Name(),
		"The collected middleware should be the one that was registered")
}

// TestProvideAuthenticatorReachesTheAuthManager pins the same property for the
// login mechanism group: a custom authenticator must arrive where AuthManager
// aggregates, or security/auth.login rejects its type as unsupported.
func TestProvideAuthenticatorReachesTheAuthManager(t *testing.T) {
	type collector struct {
		fx.In

		Authenticators []security.Authenticator `group:"vef:security:authenticators"`
	}

	var collected []security.Authenticator

	fxApp := fx.New(
		ProvideAuthenticator(func() security.Authenticator { return new(HostAuthenticator) }),
		fx.Invoke(func(c collector) { collected = c.Authenticators }),
		fx.NopLogger,
	)

	require.NoError(t, fxApp.Err(), "The authenticator graph should resolve")
	require.Len(t, collected, 1,
		"An authenticator registered through the public contract must reach the group AuthManager aggregates")
	require.True(t, collected[0].Supports("host"),
		"The collected authenticator should be the one that was registered")
}
