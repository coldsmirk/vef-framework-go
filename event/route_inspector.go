package event

// RouteInspector exposes read-only queries over the bus's configured
// routing rules so modules that depend on specific delivery guarantees
// can fail fast at start-up instead of at the first failing Publish.
//
// The framework's bus implementation satisfies this interface; tests
// can stub it with a simple in-memory map.
type RouteInspector interface {
	// HasTransactionalRoute reports whether the resolved route for
	// eventType contains at least one transport whose Capabilities
	// declare Transactional=true. Returns false when no transports
	// route to the type, or when the bus has not yet built its router
	// (i.e. before Start).
	//
	// Modules that publish with WithTx (transactional outbox pattern)
	// should call this once during fx lifecycle OnStart to assert that
	// their event types route to a Transactional transport — without
	// it, the first Publish under WithTx would fail with ErrTxRequired
	// at runtime.
	HasTransactionalRoute(eventType string) bool
}
