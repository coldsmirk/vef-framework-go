package integration

import "github.com/coldsmirk/vef-framework-go/httpx"

// MaskedSecret is the placeholder management APIs return in place of a
// sensitive auth parameter value. An update submitting the placeholder keeps
// the stored value unchanged.
const MaskedSecret = "******"

// AuthScheme turns a system's declarative auth parameters into httpx options
// (credentials, signing hooks) applied to every outbound request of that
// system. Built-in schemes cover the common cases; applications register
// their own via vef.ProvideIntegrationAuthScheme, and a scheme whose name
// matches a built-in replaces it.
type AuthScheme interface {
	// Name identifies the scheme referenced by a system's auth.scheme.
	Name() string
	// Apply validates params and returns the client options implementing the
	// scheme. Params arrive decrypted; sensitive values must never be logged.
	Apply(params map[string]string) ([]httpx.Option, error)
	// SensitiveParams names the parameters whose values are stored encrypted
	// and masked in management API responses.
	SensitiveParams() []string
}
