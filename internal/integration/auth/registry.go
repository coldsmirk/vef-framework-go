package auth

import (
	"github.com/coldsmirk/vef-framework-go/integration"
)

// Registry resolves auth schemes by name: the built-in schemes overlaid by
// application-provided ones (group "vef:integration:auth_schemes"). An
// application scheme whose name matches a built-in replaces it.
type Registry struct {
	schemes map[string]integration.AuthScheme
}

// NewRegistry builds the registry from the built-in schemes and the
// application-provided overlays.
func NewRegistry(appSchemes []integration.AuthScheme) *Registry {
	builtins := builtinSchemes()
	schemes := make(map[string]integration.AuthScheme, len(builtins)+len(appSchemes))

	for _, scheme := range builtins {
		schemes[scheme.Name()] = scheme
	}

	for _, scheme := range appSchemes {
		if scheme != nil {
			schemes[scheme.Name()] = scheme
		}
	}

	return &Registry{schemes: schemes}
}

// Get returns the scheme registered under name.
func (r *Registry) Get(name string) (integration.AuthScheme, bool) {
	scheme, ok := r.schemes[name]

	return scheme, ok
}

// Resolve returns the scheme for cfg: the none scheme for a nil config, else
// the scheme cfg names. An unknown name reports ok=false.
func (r *Registry) Resolve(cfg *integration.AuthConfig) (integration.AuthScheme, bool) {
	if cfg == nil || cfg.Scheme == "" {
		return r.schemes[SchemeNone], true
	}

	return r.Get(cfg.Scheme)
}
