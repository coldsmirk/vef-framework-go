// Package middleware defines the publish-side and consume-side
// middleware contracts used by the event bus. Middleware composes
// orthogonal concerns (tracing, logging, metrics, idempotency, panic
// recovery) so that transports and handlers stay focused.
package middleware

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/event/transport"
)

// PublishHandler is the inner function a PublishMiddleware wraps.
type PublishHandler func(ctx context.Context, env *event.Envelope) error

// PublishMiddleware augments the publish pipeline. Implementations can
// mutate the Envelope (e.g. inject tracing headers) before invoking
// next, or short-circuit by returning without calling next.
type PublishMiddleware interface {
	// Name identifies the middleware for ordering and diagnostics.
	Name() string
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
	// Name identifies the middleware for ordering and diagnostics.
	Name() string
	// Applies reports whether the middleware should attach to a
	// transport with the given capabilities. For example, Inbox
	// returns false on non-AtLeastOnce transports to avoid pointless
	// database writes.
	Applies(caps transport.Capabilities) bool
	// WrapConsume returns a new handler that wraps next.
	WrapConsume(next ConsumeHandler) ConsumeHandler
}

// ChainPublish composes middlewares in order: the first element in mws
// becomes the outermost wrapper. Nil entries are skipped so opt-out
// constructors (returning nil under disabled feature flags) compose
// safely with fx groups.
func ChainPublish(mws []PublishMiddleware, base PublishHandler) PublishHandler {
	h := base
	for i := len(mws) - 1; i >= 0; i-- {
		if mws[i] == nil {
			continue
		}

		h = mws[i].WrapPublish(h)
	}

	return h
}

// ChainConsume composes consume middlewares filtered by transport
// capabilities. The first applicable middleware becomes the outermost
// wrapper. Nil entries are skipped (see ChainPublish).
func ChainConsume(mws []ConsumeMiddleware, caps transport.Capabilities, base ConsumeHandler) ConsumeHandler {
	h := base
	for i := len(mws) - 1; i >= 0; i-- {
		if mws[i] == nil || !mws[i].Applies(caps) {
			continue
		}

		h = mws[i].WrapConsume(h)
	}

	return h
}
