package middleware

import (
	"context"
	"slices"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/event/transport"
)

// Built-in middleware ordering. Lower Order wraps further out (runs
// earlier on the way in, later on the way out); equal Order preserves
// registration order. Custom middleware can position itself relative to
// these reference points.
const (
	// OrderRecover wraps outermost so it captures panics from every other
	// middleware and from the handler.
	OrderRecover = -100
	// OrderTracing runs early so the trace context it installs is visible
	// to logging, metrics, and the handler.
	OrderTracing = -50
	// OrderLogging runs after tracing so log lines can carry the trace ID.
	OrderLogging = -25
	// OrderMetrics measures the handler plus the inner middlewares.
	OrderMetrics = 0
	// OrderInbox runs innermost so the dedupe decision sits closest to the
	// handler's side effects.
	OrderInbox = 100
)

// PublishHandler is the inner function a PublishMiddleware wraps.
type PublishHandler func(ctx context.Context, env *event.Envelope) error

// PublishMiddleware augments the publish pipeline. Implementations can
// mutate the Envelope (e.g. inject tracing headers) before invoking
// next, or short-circuit by returning without calling next.
type PublishMiddleware interface {
	// Name identifies the middleware for diagnostics.
	Name() string
	// Order sets the middleware's chain position. Lower values wrap
	// further out; equal values keep registration order. See the Order*
	// constants.
	Order() int
	// WrapPublish returns a new handler that wraps next with the
	// middleware's behavior.
	WrapPublish(next PublishHandler) PublishHandler
}

// ConsumeHandler is the inner function a ConsumeMiddleware wraps.
// The Delivery is exposed so middleware can inspect Attempt counts and
// Ack/Nack the message directly (e.g. inbox dedupe).
type ConsumeHandler func(ctx context.Context, d transport.Delivery, env event.Envelope) error

// ConsumeMiddleware augments the consume pipeline. Implementations run
// before the user-registered handler and may transform context, log,
// dedupe, capture metrics, recover panics, etc.
type ConsumeMiddleware interface {
	// Name identifies the middleware for diagnostics.
	Name() string
	// Order sets the middleware's chain position. Lower values wrap
	// further out; equal values keep registration order. See the Order*
	// constants.
	Order() int
	// Applies reports whether the middleware should attach to a
	// transport with the given capabilities. For example, Inbox
	// returns false on non-AtLeastOnce transports to avoid pointless
	// database writes.
	Applies(caps transport.Capabilities) bool
	// WrapConsume returns a new handler that wraps next.
	WrapConsume(next ConsumeHandler) ConsumeHandler
}

// ChainPublish composes middlewares by ascending Order so the lowest
// Order becomes the outermost wrapper; equal Order keeps registration
// order. Nil entries are skipped so opt-out constructors (returning nil
// under disabled feature flags) compose safely with fx groups.
func ChainPublish(mws []PublishMiddleware, base PublishHandler) PublishHandler {
	ordered := make([]PublishMiddleware, 0, len(mws))
	for _, mw := range mws {
		if mw != nil {
			ordered = append(ordered, mw)
		}
	}

	slices.SortStableFunc(ordered, func(a, b PublishMiddleware) int {
		return a.Order() - b.Order()
	})

	h := base
	for _, mw := range slices.Backward(ordered) {
		h = mw.WrapPublish(h)
	}

	return h
}

// ChainConsume composes consume middlewares filtered by transport
// capabilities, by ascending Order so the lowest Order becomes the
// outermost wrapper; equal Order keeps registration order. Nil and
// non-applicable entries are skipped (see ChainPublish).
func ChainConsume(mws []ConsumeMiddleware, caps transport.Capabilities, base ConsumeHandler) ConsumeHandler {
	ordered := make([]ConsumeMiddleware, 0, len(mws))
	for _, mw := range mws {
		if mw != nil && mw.Applies(caps) {
			ordered = append(ordered, mw)
		}
	}

	slices.SortStableFunc(ordered, func(a, b ConsumeMiddleware) int {
		return a.Order() - b.Order()
	})

	h := base
	for _, mw := range slices.Backward(ordered) {
		h = mw.WrapConsume(h)
	}

	return h
}
