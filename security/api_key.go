package security

import "context"

// APIKeyLoader resolves presented API keys for the "api_key" auth strategy.
// The framework ships a configuration-backed implementation
// (vef.security.api_keys); applications may register their own to load keys
// from a database, a config center, or any other source.
//
// The presented key is the lookup credential itself, so — unlike
// IPWhitelistLoader — the comparison lives inside the implementation.
// Implementations that scan candidate keys MUST compare in constant time
// (the config-backed loader does); implementations that index by key should
// only serve high-entropy random keys, where lookup timing reveals nothing.
type APIKeyLoader interface {
	// LoadByKey resolves the presented key to its Principal, or nil when no
	// key matches. An error signals an infrastructure fault, not a rejection.
	LoadByKey(ctx context.Context, key string) (*Principal, error)
}
