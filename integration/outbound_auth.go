package integration

import "github.com/coldsmirk/vef-framework-go/httpx"

// OutboundAuthScheme turns a system's declarative outbound auth configuration
// into httpx options (credentials, signing hooks) applied to every outbound
// request of that system. Built-in schemes cover the common cases;
// applications register their own via vef.ProvideIntegrationOutboundAuthScheme,
// and a scheme whose name matches a built-in replaces it.
type OutboundAuthScheme interface {
	// Name identifies the scheme referenced by a system's outboundAuth.scheme.
	Name() string
	// Apply validates cfg and returns the client options implementing the
	// scheme. Cfg is never nil and its params arrive decrypted; sensitive
	// values must never be logged.
	Apply(cfg *OutboundAuthConfig) ([]httpx.Option, error)
	// SensitiveParams names the parameters whose values are stored encrypted
	// and masked in management API responses; the SensitiveAll wildcard marks
	// every parameter sensitive.
	SensitiveParams() []string
}
