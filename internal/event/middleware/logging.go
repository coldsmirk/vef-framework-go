package middleware

import (
	"context"
	"time"

	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/event/middleware"
	"github.com/coldsmirk/vef-framework-go/event/transport"
	"github.com/coldsmirk/vef-framework-go/logx"
)

// Logging logs publish and consume events at debug/info level. Errors
// are escalated to warn so they show up in default operational logs.
type Logging struct {
	logger logx.Logger
}

// NewLogging constructs a Logging middleware.
func NewLogging(logger logx.Logger) *Logging {
	return &Logging{logger: logger}
}

// Name implements both PublishMiddleware and ConsumeMiddleware.
func (*Logging) Name() string { return "logging" }

// Order implements both middleware interfaces. Logging runs after tracing
// so log lines can carry the trace ID.
func (*Logging) Order() int { return middleware.OrderLogging }

// Applies always returns true; logging is cross-cutting.
func (*Logging) Applies(transport.Capabilities) bool { return true }

// WrapPublish implements PublishMiddleware.
func (m *Logging) WrapPublish(next middleware.PublishHandler) middleware.PublishHandler {
	return func(ctx context.Context, env *event.Envelope) error {
		err := next(ctx, env)
		if err != nil {
			m.logger.Warnf("event publish failed: type=%s id=%s: %v", env.Type, env.ID, err)
		} else {
			m.logger.Debugf("event published: type=%s id=%s", env.Type, env.ID)
		}

		return err
	}
}

// WrapConsume implements ConsumeMiddleware.
func (m *Logging) WrapConsume(next middleware.ConsumeHandler) middleware.ConsumeHandler {
	return func(ctx context.Context, d transport.Delivery, env event.Envelope) error {
		start := time.Now()
		err := next(ctx, d, env)

		elapsed := time.Since(start)
		if err != nil {
			m.logger.Warnf("event consume failed: type=%s id=%s attempt=%d elapsed=%s: %v",
				env.Type, env.ID, d.Attempt(), elapsed, err)
		} else {
			m.logger.Debugf("event consumed: type=%s id=%s attempt=%d elapsed=%s",
				env.Type, env.ID, d.Attempt(), elapsed)
		}

		return err
	}
}
