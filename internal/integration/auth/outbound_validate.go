package auth

import (
	"github.com/coldsmirk/vef-framework-go/integration"
)

// ValidateOutboundAuth rejects a signing body on an outbound scheme that will
// never run one, mirroring the inbound structural check. Scheme existence and
// parameter validation stay in definition.ValidateSystem, which exercises the
// scheme's Apply against the decrypted config.
func ValidateOutboundAuth(cfg *integration.OutboundAuthConfig) error {
	if cfg == nil {
		return nil
	}

	if cfg.Scheme != OutboundSchemeScript && cfg.Script != "" {
		return integration.ErrInvalidAuthParams("script is only supported by the script scheme")
	}

	return nil
}
