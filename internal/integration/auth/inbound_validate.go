package auth

import (
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/definition"
)

// ValidateInboundAuth rejects an inbound auth config referencing an unknown
// scheme, a script scheme without a compilable verification body, or a
// verification body on a scheme that will never run one. Params must already
// be in their submitted form — the check runs before encryption. It returns
// the resolved scheme (nil for a nil config) so the save path resolves once.
func ValidateInboundAuth(registry *InboundRegistry, cfg *integration.InboundAuthConfig) (integration.InboundAuthScheme, error) {
	if cfg == nil {
		return nil, nil
	}

	scheme, ok := registry.Resolve(cfg)
	if !ok {
		return nil, integration.ErrUnknownAuthScheme(cfg.Scheme)
	}

	if cfg.Scheme != InboundSchemeScript {
		if cfg.Script != "" {
			return nil, integration.ErrInvalidAuthParams("script is only supported by the script scheme")
		}

		// A blank credential value would fail open: the multi-pair schemes
		// compare it against an absent request header, and two empty strings
		// match. Reject it at save so a fail-closed scheme cannot be
		// misconfigured into an open door.
		if cfg.Scheme == InboundSchemeHeader || cfg.Scheme == InboundSchemeQuery {
			for name, value := range cfg.Params {
				if value == "" {
					return nil, integration.ErrInvalidAuthParams("credential value for " + name + " must not be empty")
				}
			}
		}

		return scheme, nil
	}

	if cfg.Script == "" {
		return nil, integration.ErrInvalidAuthParams("the script scheme requires a verification script")
	}

	if _, err := definition.CompileScript(cfg.Script); err != nil {
		return nil, integration.ErrInvalidScript(err.Error())
	}

	return scheme, nil
}
