package crud

import (
	"time"

	"github.com/coldsmirk/vef-framework-go/api"
)

// Builder defines the interface for building API endpoints.
type Builder[T any] interface {
	// ResourceKind sets the resource kind (RPC or REST) for this endpoint.
	ResourceKind(kind api.Kind) T
	// Action sets the action name for this endpoint.
	Action(action string) T
	// EnableAudit enables audit logging for this endpoint.
	EnableAudit() T
	// Timeout sets the request timeout for this endpoint.
	Timeout(timeout time.Duration) T
	// Public marks this endpoint as publicly accessible without authentication.
	Public() T
	// PermToken sets the permission token required to access this endpoint.
	PermToken(token string) T
	// RateLimit configures rate limiting for this endpoint.
	RateLimit(maxRequests int, period time.Duration) T
	// Build creates an OperationSpec with the configured settings and the given handler.
	Build(handler any) api.OperationSpec
}

type baseBuilder[T any] struct {
	kind        api.Kind
	action      string
	enableAudit bool
	timeout     time.Duration
	public      bool
	permToken   string
	rateLimit   *api.RateLimitConfig

	self T
}

func (b *baseBuilder[T]) ResourceKind(kind api.Kind) T {
	b.kind = kind

	return b.self
}

func (b *baseBuilder[T]) Action(action string) T {
	if err := api.ValidateActionName(action, b.kind); err != nil {
		panic(err)
	}

	b.action = action

	return b.self
}

func (b *baseBuilder[T]) EnableAudit() T {
	b.enableAudit = true

	return b.self
}

func (b *baseBuilder[T]) Timeout(timeout time.Duration) T {
	b.timeout = timeout

	return b.self
}

func (b *baseBuilder[T]) Public() T {
	b.public = true

	return b.self
}

func (b *baseBuilder[T]) PermToken(token string) T {
	b.permToken = token

	return b.self
}

func (b *baseBuilder[T]) RateLimit(maxRequests int, period time.Duration) T {
	b.rateLimit = &api.RateLimitConfig{
		Max:    maxRequests,
		Period: period,
	}

	return b.self
}

func (b *baseBuilder[T]) Build(handler any) api.OperationSpec {
	return api.OperationSpec{
		Action:      b.action,
		EnableAudit: b.enableAudit,
		Timeout:     b.timeout,
		Public:      b.public,
		PermToken:   b.permToken,
		RateLimit:   b.rateLimit,
		Handler:     handler,
	}
}
