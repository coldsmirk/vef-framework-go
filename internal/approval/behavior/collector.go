package behavior

import (
	"context"
	"fmt"
	"reflect"

	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
)

var logger = logx.Named("approval:behavior")

// Collector buffers items of type T produced by a command handler while it
// runs inside the CQRS pipeline. Handlers append items to the collector
// instead of touching the outside world directly; a collectorBehavior[T]
// flushes the buffer in one batch after the handler returns successfully.
//
// Concrete uses live in event_publish.go (Collector[approval.DomainEvent])
// and action_log.go (Collector[*approval.ActionLog]).
type Collector[T any] struct {
	items []T
}

// Add appends items to the collector. Nil entries — including typed-nil
// pointers / nil interface values — are silently dropped so call sites can
// pass best-effort outputs without guarding each one.
func (c *Collector[T]) Add(items ...T) {
	for _, item := range items {
		if isNil(item) {
			continue
		}

		c.items = append(c.items, item)
	}
}

// Items returns the buffered slice. Callers should not mutate it; the
// returned slice is shared with the behavior that will eventually flush it.
func (c *Collector[T]) Items() []T {
	return c.items
}

// truncate drops everything appended after mark, undoing the additions of a
// failed re-entrant dispatch the way a rolled-back savepoint unwinds the
// commit hooks registered inside it.
func (c *Collector[T]) truncate(mark int) {
	clear(c.items[mark:])
	c.items = c.items[:mark]
}

func isNil(v any) bool {
	if v == nil {
		return true
	}

	rv := reflect.ValueOf(v)

	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
		return rv.IsNil()
	default:
		return false
	}
}

type collectorKey[T any] struct{}

// installCollector binds a Collector[T] to ctx and returns the derived
// context, the collector handle, and whether this call owns the flush.
//
// A collector already on the context is reused and ownership declined, which
// is what keeps a re-entrant dispatch ordered. approval.Service lets host code
// re-enter the pipeline from inside a running command — an
// InstanceLifecycleHook fires in-transaction and may itself act on another
// task — and a fresh inner collector would flush the inner command's events
// and action logs the moment the inner handler returned, i.e. ahead of the
// outer command's, publishing an effect before its cause. Appending to the
// outer buffer instead keeps one ordered sequence flushed once by the
// outermost invocation, matching how TransactionBehavior joins an already-open
// transaction rather than nesting a second one. It also gives the two the same
// failure semantics: the inner items are discarded if the outer command fails,
// exactly like the writes they describe.
func installCollector[T any](ctx context.Context) (context.Context, *Collector[T], bool) {
	if existing, ok := TryCollectorFromContext[T](ctx); ok {
		return ctx, existing, false
	}

	collector := new(Collector[T])

	return context.WithValue(ctx, collectorKey[T]{}, collector), collector, true
}

// TryCollectorFromContext returns the request-scoped Collector[T] if one is
// installed, else (nil, false). Use this when the absence of the collector
// should not produce a warning — for example, engine helpers that must
// fall back to a direct transactional publish when invoked outside the CQRS
// pipeline (the timeout scanner driving the engine).
func TryCollectorFromContext[T any](ctx context.Context) (*Collector[T], bool) {
	c, ok := ctx.Value(collectorKey[T]{}).(*Collector[T])

	return c, ok
}

// collectorFromContextOrWarn returns the request-scoped Collector[T] or a
// detached collector with a Warn-level log line so misconfigured handlers
// surface in operator logs rather than silently dropping events / audit
// rows.
//
// label names the collector for the warning message (e.g. "EventCollector",
// "ActionLogCollector"); behaviorName names the FX-registered behavior the
// operator should check (e.g. "EventPublishBehavior").
func collectorFromContextOrWarn[T any](ctx context.Context, label, behaviorName string) *Collector[T] {
	if c, ok := TryCollectorFromContext[T](ctx); ok {
		return c
	}

	logger.Warnf("approval: %s missing from context — items will be discarded; ensure %s is registered", label, behaviorName)

	return new(Collector[T])
}

// collectorBehavior buffers values produced by a command handler in a ctx-
// bound Collector[T] and runs flush once the handler returns successfully.
// Query actions and handler errors short-circuit the flush so callers never
// see partial side effects.
//
// All of the per-T plumbing — ctx key, slice management, error wrapping,
// query-bypass — lives here; concrete behaviors only have to supply flush
// (and an Order for the deterministic wrapping).
type collectorBehavior[T any] struct {
	order int
	name  string
	flush func(ctx context.Context, items []T) error
}

// Order returns the deterministic wrapping position (see cqrs.Ordered).
func (b *collectorBehavior[T]) Order() int { return b.order }

// Handle installs the collector, runs the handler, and flushes on success.
// A re-entrant dispatch joins the enclosing collector and leaves the flush to
// it, so the buffered items stay in occurrence order.
func (b *collectorBehavior[T]) Handle(ctx context.Context, action cqrs.Action, next func(context.Context) (any, error)) (any, error) {
	if action.Kind() == cqrs.Query {
		return next(ctx)
	}

	ctx, collector, owned := installCollector[T](ctx)
	mark := len(collector.items)

	result, err := next(ctx)
	if err != nil {
		// Whatever this dispatch buffered describes writes that are not
		// happening. Owning the collector, that is the whole buffer; joined to
		// an enclosing one, it is only what was appended here — the enclosing
		// command may still succeed, and its own items must survive.
		collector.truncate(mark)

		return nil, err
	}

	if !owned || len(collector.items) == 0 {
		return result, nil
	}

	if err := b.flush(ctx, collector.items); err != nil {
		return nil, fmt.Errorf("flush %s: %w", b.name, err)
	}

	return result, nil
}
