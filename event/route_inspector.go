package event

// RouteInspector exposes read-only queries over the bus's configured
// routing rules so modules that depend on specific delivery guarantees
// can fail fast at start-up instead of at the first failing Publish or
// Subscribe.
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

	// HasSubscribableTransport reports whether the resolved route for
	// eventType contains at least one transport whose Capabilities do
	// not declare PublishOnly. Returns false when no transports route
	// to the type, or when the bus has not yet built its router.
	//
	// Modules whose framework-side code subscribes to an event
	// (binding listeners, projections, integration handlers) should
	// call this during fx lifecycle OnStart to assert that subscribers
	// can actually receive deliveries. Without it, a route that
	// resolves only to publish-only transports (e.g. the transactional
	// outbox alone) lets the application start, but every Subscribe
	// call against the route fails at runtime with ErrNoRouteMatched
	// — typically after the bus has already accepted publishes that
	// no consumer will ever see.
	HasSubscribableTransport(eventType string) bool
}
