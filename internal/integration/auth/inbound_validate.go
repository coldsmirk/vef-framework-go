package auth

import (
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/service"
)

// ValidateInboundAuth rejects an inbound auth config referencing an unknown
// scheme, a script scheme without a compilable verification body, or a
// verification body on a scheme that will never run one. Params must already
// be in their submitted form — the check runs before encryption.
func ValidateInboundAuth(registry *InboundRegistry, cfg *integration.InboundAuthConfig) error {
	if cfg == nil {
		return nil
	}

	if _, ok := registry.Resolve(cfg); !ok {
		return integration.ErrUnknownAuthScheme(cfg.Scheme)
	}

	if cfg.Scheme != InboundSchemeScript {
		if cfg.Script != "" {
			return integration.ErrInvalidAuthParams("script is only supported by the script scheme")
		}

		return nil
	}

	if cfg.Script == "" {
		return integration.ErrInvalidAuthParams("the script scheme requires a verification script")
	}

	if _, err := service.CompileScript(cfg.Script); err != nil {
		return integration.ErrInvalidScript(err.Error())
	}

	return nil
}
