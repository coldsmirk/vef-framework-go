package push

// Close codes the push endpoint sends, in the RFC 6455 private-use range
// (4000-4999). They are part of the client protocol contract: a client seeing
// one of these must treat the closure as terminal and not auto-reconnect
// (transport-level failures, by contrast, reconnect with backoff).
const (
	// CloseSessionInvalid reports that the connection's login session was
	// revoked (logout, concurrent-login eviction, administrative kick) or
	// expired. The client should enter its logged-out flow.
	CloseSessionInvalid = 4401
	// CloseTooManyConnections reports that the per-user connection cap was
	// reached; the client should not retry until another connection closes.
	CloseTooManyConnections = 4429
)
