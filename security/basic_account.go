package security

import "context"

// BasicAccountLoader resolves service accounts for the "http_basic" auth
// strategy. The framework ships a configuration-backed implementation
// (vef.security.basic_accounts); applications may register their own to load
// accounts from a database, a config center, or any other source.
//
// The loader returns the stored secret and the framework performs the
// constant-time comparison, so every implementation shares the same
// fail-closed semantics (mirroring UserLoader / ExternalAppLoader). Basic
// accounts are machine-to-machine credentials: store high-entropy random
// secrets, not user passwords.
type BasicAccountLoader interface {
	// LoadByUsername retrieves a service account by username, returning the
	// Principal and its stored secret. A nil Principal or empty secret means
	// the account is unknown; an error signals an infrastructure fault.
	LoadByUsername(ctx context.Context, username string) (*Principal, string, error)
}
