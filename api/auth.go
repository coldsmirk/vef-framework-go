package api

import (
	"fmt"
	"maps"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/security"
)

// AuthStrategy handles authentication for a specific auth type.
type AuthStrategy interface {
	// Name returns the strategy name (used in AuthConfig.Strategy).
	Name() string
	// Authenticate validates credentials and returns principal.
	Authenticate(ctx fiber.Ctx, options map[string]any) (*security.Principal, error)
}

// AuthStrategyRegistry manages authentication strategies.
type AuthStrategyRegistry interface {
	// Register adds a strategy to the registry.
	Register(strategy AuthStrategy)
	// Get retrieves a strategy by name.
	Get(name string) (AuthStrategy, bool)
	// Names returns all registered strategy names.
	Names() []string
}

// Auth strategy constants.
const (
	AuthStrategyNone      = "none"
	AuthStrategyBearer    = "bearer"
	AuthStrategySignature = "signature"
	AuthStrategyIP        = "ip"
)

// AuthOptionWhitelist is the AuthConfig.Options key holding the name of the
// IP whitelist the "ip" strategy authenticates against.
const AuthOptionWhitelist = "whitelist"

// DefaultIPWhitelist is the whitelist name IPAuth falls back to when called
// without an explicit name; configure it as the "default" key under
// vef.security.ip_whitelists (or serve it from a custom loader).
const DefaultIPWhitelist = "default"

// AuthConfig defines authentication configuration for an operation.
type AuthConfig struct {
	// Strategy specifies the auth strategy name. default is "bearer".
	// Built-in: "none", "bearer", "signature", "ip"
	// Custom strategies can be registered via AuthStrategyRegistry.
	Strategy string
	// Options holds strategy-specific configuration.
	Options map[string]any
}

// Clone creates a deep copy of the AuthConfig.
func (c *AuthConfig) Clone() *AuthConfig {
	if c == nil {
		return nil
	}

	clone := &AuthConfig{
		Strategy: c.Strategy,
	}

	if c.Options != nil {
		clone.Options = maps.Clone(c.Options)
	}

	return clone
}

// Public creates an AuthConfig for public endpoints (no authentication).
func Public() *AuthConfig {
	return &AuthConfig{
		Strategy: AuthStrategyNone,
	}
}

// BearerAuth creates an AuthConfig for BearerAuth token authentication.
func BearerAuth() *AuthConfig {
	return &AuthConfig{
		Strategy: AuthStrategyBearer,
	}
}

// SignatureAuth creates an AuthConfig for signature-based authentication.
func SignatureAuth() *AuthConfig {
	return &AuthConfig{
		Strategy: AuthStrategySignature,
	}
}

// IPAuth creates an AuthConfig for source-IP whitelist authentication. The
// request is authenticated when the client IP matches the named whitelist,
// resolved through the registered security.IPWhitelistLoader (by default the
// vef.security.ip_whitelists configuration). Call it with no argument to
// target the DefaultIPWhitelist name, or with exactly one name to target a
// specific whitelist; passing more than one name panics. The client IP is the
// one resolved by Fiber, so behind a reverse proxy vef.app.trusted_proxies
// must be configured for the whitelist to see the real client address.
func IPAuth(whitelistName ...string) *AuthConfig {
	if len(whitelistName) > 1 {
		panic(fmt.Sprintf("api.IPAuth accepts at most one whitelist name, got %d", len(whitelistName)))
	}

	name := DefaultIPWhitelist
	if len(whitelistName) == 1 {
		name = whitelistName[0]
	}

	return &AuthConfig{
		Strategy: AuthStrategyIP,
		Options:  map[string]any{AuthOptionWhitelist: name},
	}
}
