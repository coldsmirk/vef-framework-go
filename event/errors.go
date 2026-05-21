package event

import "errors"

var (
	// ErrBusNotStarted indicates Publish was called before the bus
	// completed its Start lifecycle hook. Publishing during fx Provide
	// is not supported; move the call to a fx.Invoke after app start.
	ErrBusNotStarted = errors.New("event: bus not started")
	// ErrBusAlreadyStarted indicates a second Start call on the bus.
	ErrBusAlreadyStarted = errors.New("event: bus already started")
	// ErrTxRequired indicates WithTx was used but the resolved route
	// has no TxTransport. Typically a routing misconfiguration.
	ErrTxRequired = errors.New("event: transactional publish requires a TxTransport on route")
	// ErrTransportNotFound indicates routing referenced a transport
	// name that was not registered or whose enabled flag is false.
	ErrTransportNotFound = errors.New("event: transport not found")
	// ErrAsyncQueueFull indicates the async fan-in queue is full and
	// the publish was dropped. Reported via ErrorSink, not returned.
	ErrAsyncQueueFull = errors.New("event: async queue full")
	// ErrQueueFull indicates a transport operating with a non-blocking
	// full policy rejected the publish.
	ErrQueueFull = errors.New("event: queue full")
	// ErrHandlerPanic wraps a recovered panic from a subscriber.
	ErrHandlerPanic = errors.New("event: handler panic")
	// ErrShutdownTimeout indicates the bus did not drain within the
	// graceful shutdown deadline.
	ErrShutdownTimeout = errors.New("event: shutdown timeout exceeded")
	// ErrNoRouteMatched indicates no routing rule (including the
	// fallback) matched the published event type.
	ErrNoRouteMatched = errors.New("event: no route matched")
	// ErrUnknownPayload indicates SubscribeTyped received an Envelope
	// whose Payload could neither be type-asserted nor JSON-decoded
	// into the declared T.
	ErrUnknownPayload = errors.New("event: unknown payload type")
	// ErrPayloadTooLarge indicates a publish was rejected because the
	// encoded payload or headers exceed the framework size limits.
	// Cross-process transports inherit the limit so a single hostile
	// publisher cannot push arbitrarily large frames through the bus.
	ErrPayloadTooLarge = errors.New("event: payload too large")
	// ErrInvalidEventType indicates an event type contains characters
	// outside the allowed alphabet (^[a-zA-Z0-9._-]+$). The framework
	// rejects such types at publish to keep transport keys safe.
	ErrInvalidEventType = errors.New("event: invalid event type")
	// ErrNilTypeParameter indicates SubscribeTyped was instantiated
	// with a type parameter that has no runtime representation (a
	// nil interface). Callers should pass a concrete *MyEvent or
	// MyEvent type, not the bare Event interface.
	ErrNilTypeParameter = errors.New("event: SubscribeTyped requires a concrete event type")
	// ErrGroupRequired indicates Subscribe was called on a route that
	// resolves to one or more at-least-once transports (outbox, Redis
	// Streams, …) without an explicit WithGroup option. A stable group
	// name is the dedupe scope for the Inbox middleware and the
	// consumer group name for Redis Streams; an auto-generated value
	// would change across restarts and either reset Redis Streams
	// position or invalidate dedupe records.
	ErrGroupRequired = errors.New("event: at-least-once subscription requires event.WithGroup(\"name\")")
	// ErrTxAsyncMutex indicates the caller combined WithTx and
	// WithAsync, which is contradictory: a transactional publish must
	// complete inside the caller's transaction window.
	ErrTxAsyncMutex = errors.New("event: WithTx and WithAsync are mutually exclusive")
)
