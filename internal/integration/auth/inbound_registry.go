package auth

import (
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/js"
	"github.com/coldsmirk/vef-framework-go/security"
)

// InboundRegistry resolves inbound auth schemes by name: the built-in schemes
// overlaid by application-provided ones (group
// "vef:integration:inbound_auth_schemes"). An application scheme whose name
// matches a built-in replaces it.
type InboundRegistry struct {
	schemes map[string]integration.InboundAuthScheme
}

// NewInboundRegistry builds the registry from the built-in schemes and the
// application-provided overlays. The engine powers the built-in script
// scheme's verification runtimes.
func NewInboundRegistry(engine *js.Engine, cfg *config.IntegrationConfig, nonceStore security.NonceStore, appSchemes []integration.InboundAuthScheme) *InboundRegistry {
	return &InboundRegistry{schemes: overlayByName(builtinInboundSchemes(engine, cfg.EffectiveRunTimeout(), nonceStore), appSchemes)}
}

// Resolve returns the scheme for cfg. Unlike the outbound registry there is
// no implicit default: inbound delivery is fail-closed, so a nil config or a
// blank scheme name resolves to nothing and the caller denies.
func (r *InboundRegistry) Resolve(cfg *integration.InboundAuthConfig) (integration.InboundAuthScheme, bool) {
	if cfg == nil || cfg.Scheme == "" {
		return nil, false
	}

	scheme, ok := r.schemes[cfg.Scheme]

	return scheme, ok
}
