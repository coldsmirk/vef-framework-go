package auth

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/contextx"
	isecurity "github.com/coldsmirk/vef-framework-go/internal/security"
	"github.com/coldsmirk/vef-framework-go/security"
)

// APIKeyStrategy implements api.AuthStrategy for static API key
// authentication: the request presents a key in a header (HeaderXAPIKey by
// default, overridable per operation via api.APIKeyAuth(header)) and the
// strategy resolves it through the security.APIKeyLoader.
//
// Every authentication failure denies with security.ErrAPIKeyInvalid (fail
// closed) and configuration faults are additionally logged server-side, so
// the 401 stays opaque to the caller. Loader errors propagate as-is: an
// unavailable backing store is an infrastructure fault, not a credential
// rejection.
type APIKeyStrategy struct {
	loader security.APIKeyLoader
}

// NewAPIKey creates a new API key authentication strategy. The loader is
// optional: when the application registers no security.APIKeyLoader, the
// strategy falls back to the configuration-backed loader serving
// vef.security.api_keys (whose construction validates the configured keys
// eagerly and fails start-up on a fault).
func NewAPIKey(loader security.APIKeyLoader, cfg *config.SecurityConfig) (api.AuthStrategy, error) {
	if loader == nil {
		var err error
		if loader, err = isecurity.NewConfigAPIKeyLoader(cfg); err != nil {
			return nil, err
		}
	}

	return &APIKeyStrategy{loader: loader}, nil
}

// Name returns the strategy name.
func (*APIKeyStrategy) Name() string {
	return api.AuthStrategyAPIKey
}

// Authenticate resolves the key presented in the configured header through
// the loader and returns the principal it maps to.
func (s *APIKeyStrategy) Authenticate(ctx fiber.Ctx, options map[string]any) (*security.Principal, error) {
	header, _ := options[api.AuthOptionAPIKeyHeader].(string)
	if header == "" {
		header = api.HeaderXAPIKey
	}

	key := ctx.Get(header)
	if key == "" {
		return nil, security.ErrAPIKeyInvalid
	}

	principal, err := s.loader.LoadByKey(ctx.Context(), key)
	if err != nil {
		return nil, err
	}

	if principal == nil {
		contextx.Logger(ctx, logger).Warnf("API key authentication failed: no key matched (header %q)", header)

		return nil, security.ErrAPIKeyInvalid
	}

	return principal, nil
}
