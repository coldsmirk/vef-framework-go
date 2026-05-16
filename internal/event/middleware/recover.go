// Package middleware provides the built-in publish-side and
// consume-side middlewares wired by internal/event. Each middleware is
// a focused single-responsibility implementation of
// middleware.PublishMiddleware or middleware.ConsumeMiddleware.
package middleware

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/coldsmirk/vef-framework-go/event"
	pubmw "github.com/coldsmirk/vef-framework-go/event/middleware"
	"github.com/coldsmirk/vef-framework-go/event/transport"
	"github.com/coldsmirk/vef-framework-go/logx"
)

// Recover is a ConsumeMiddleware that converts handler panics into
// errors so the consume pipeline can apply standard retry / DLQ logic.
// The stack trace is logged at error level for postmortem.
type Recover struct {
	logger logx.Logger
}

// NewRecover constructs a Recover middleware.
func NewRecover(logger logx.Logger) *Recover {
	return &Recover{logger: logger}
}

// Name implements ConsumeMiddleware.
func (*Recover) Name() string { return "recover" }

// Applies implements ConsumeMiddleware. Recover is always useful so it
// attaches to every transport.
func (*Recover) Applies(transport.Capabilities) bool { return true }

// WrapConsume implements ConsumeMiddleware.
func (m *Recover) WrapConsume(next pubmw.ConsumeHandler) pubmw.ConsumeHandler {
	return func(ctx context.Context, d transport.Delivery, env event.Envelope) (err error) {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				m.logger.Errorf("event handler panic on %s (id=%s): %v\n%s", env.Type, env.ID, r, stack)
				err = fmt.Errorf("%w: %v", event.ErrHandlerPanic, r)
			}
		}()

		return next(ctx, d, env)
	}
}
