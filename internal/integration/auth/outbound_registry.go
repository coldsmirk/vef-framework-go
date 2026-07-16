package auth

import (
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/js"
)

// OutboundRegistry resolves auth schemes by name: the built-in schemes overlaid by
// application-provided ones (group "vef:integration:outbound_auth_schemes"). An
// application scheme whose name matches a built-in replaces it.
type OutboundRegistry struct {
	schemes map[string]integration.OutboundAuthScheme
}

// NewOutboundRegistry builds the registry from the built-in schemes and the
// application-provided overlays. The engine powers the script scheme's
// signing runtimes.
func NewOutboundRegistry(engine *js.Engine, cfg *config.IntegrationConfig, appSchemes []integration.OutboundAuthScheme) *OutboundRegistry {
	builtins := builtinOutboundSchemes(engine, cfg.EffectiveRunTimeout())
	schemes := make(map[string]integration.OutboundAuthScheme, len(builtins)+len(appSchemes))

	for _, scheme := range builtins {
		schemes[scheme.Name()] = scheme
	}

	for _, scheme := range appSchemes {
		if scheme != nil {
			schemes[scheme.Name()] = scheme
		}
	}

	return &OutboundRegistry{schemes: schemes}
}

// Get returns the scheme registered under name.
func (r *OutboundRegistry) Get(name string) (integration.OutboundAuthScheme, bool) {
	scheme, ok := r.schemes[name]

	return scheme, ok
}

// Resolve returns the scheme for cfg: the none scheme for a nil config, else
// the scheme cfg names. An unknown name reports ok=false.
func (r *OutboundRegistry) Resolve(cfg *integration.OutboundAuthConfig) (integration.OutboundAuthScheme, bool) {
	if cfg == nil || cfg.Scheme == "" {
		return r.schemes[OutboundSchemeNone], true
	}

	return r.Get(cfg.Scheme)
}
