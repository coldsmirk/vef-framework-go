package api

import (
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
)

// AuthConfig defines authentication configuration for an operation.
type AuthConfig struct {
	// Strategy specifies the auth strategy name. default is "bearer".
	// Built-in: "none", "bearer", "signature"
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
