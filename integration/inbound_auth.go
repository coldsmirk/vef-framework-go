package integration

import "context"

// InboundAuthScheme verifies that an inbound request truly originates
// from the system it targets, using the system's stored inbound auth
// configuration. Built-in schemes cover the common cases; applications
// register their own via vef.ProvideIntegrationInboundAuthScheme, and a
// scheme whose name matches a built-in replaces it.
type InboundAuthScheme interface {
	// Name identifies the scheme referenced by a system's inboundAuth.scheme.
	Name() string
	// Verify authenticates the request against the system's inbound auth
	// configuration. The config arrives with sensitive parameters decrypted;
	// their values must never be logged. Any returned error denies the
	// delivery — its message stays server-side and is never echoed to the
	// caller.
	Verify(ctx context.Context, req *InboundRequest, auth *InboundAuthConfig) error
	// SensitiveParams names the parameters whose values are stored encrypted
	// and masked in management API responses; the SensitiveAll wildcard marks
	// every parameter sensitive.
	SensitiveParams() []string
}
