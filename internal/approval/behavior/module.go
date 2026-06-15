package behavior

import (
	"errors"
	"fmt"

	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
)

// Module provides all CQRS behavior middlewares for the approval module.
// Behaviors are aggregated via FX group `group:"vef:cqrs:behaviors"`; the
// CQRS bus sorts them by their Order() method so wrapping order is
// independent of FX's group-resolution timing.
//
// Order assignments (see cqrs.Ordered). Lower Order wraps outer, and each
// collector flushes AFTER the wrapped handler returns, so the innermost
// behavior flushes first:
//
//   - Transaction  (Order 0)   wraps every inner behavior and the handler;
//     commits once they all succeed.
//   - ActionLog    (Order 100) wraps EventPublish; its audit rows therefore
//     flush AFTER EventPublish has published.
//   - EventPublish (Order 200) is the innermost behavior, so it flushes
//     (publishes events) FIRST — before the ActionLog rows are inserted.
//
// All three flushes run inside the single Transaction tx, so the relative
// order is invisible outside the commit: the events become visible iff the
// transaction (audit rows included) commits.
//
// The fx.Invoke hook runs a boot-time self-check that fails fast if either
// the ActionLog or EventPublish behavior is missing — the alternative is
// silent audit / event loss in production, which is far worse than refusing
// to start.
var Module = fx.Module(
	"vef:approval:behavior",

	fx.Provide(
		fx.Annotate(
			NewTransactionBehavior,
			fx.ResultTags(`group:"vef:cqrs:behaviors"`),
		),
		fx.Annotate(
			NewActionLogBehavior,
			fx.ResultTags(`group:"vef:cqrs:behaviors"`),
		),
		fx.Annotate(
			NewEventPublishBehavior,
			fx.ResultTags(`group:"vef:cqrs:behaviors"`),
		),
	),

	fx.Invoke(
		fx.Annotate(
			assertCollectorBehaviorsRegistered,
			fx.ParamTags(`group:"vef:cqrs:behaviors"`),
		),
	),
)

// ErrMissingCollectorBehavior is returned by the boot-time self-check when
// the ActionLog or EventPublish behavior failed to register. Surfaced via
// fx.Invoke so misconfigured hosts cannot start a process that would
// silently drop audit rows / domain events.
var ErrMissingCollectorBehavior = errors.New("approval: required collector behavior missing from cqrs:behaviors group")

// assertCollectorBehaviorsRegistered scans the behavior group for the two
// approval-critical collectors (ActionLog and EventPublish) and fails boot
// when either is absent. Host overrides via fx.Replace are fine — what we
// guard against is a host that strips approval.behavior.Module out of its
// FX graph while still pulling in the approval command handlers, which
// would silently drop audit rows and domain events.
//
// Matching is done by the concrete generic instantiation
// (*collectorBehavior[T]) rather than by Order so renumbering or adding
// host behaviors with shared Order values cannot accidentally satisfy the
// check.
func assertCollectorBehaviorsRegistered(behaviors []cqrs.Behavior) error {
	var hasActionLog, hasEventPublish bool

	for _, b := range behaviors {
		switch b.(type) {
		case *collectorBehavior[*approval.ActionLog]:
			hasActionLog = true
		case *collectorBehavior[approval.DomainEvent]:
			hasEventPublish = true
		}
	}

	switch {
	case !hasActionLog && !hasEventPublish:
		return fmt.Errorf("%w: ActionLogBehavior and EventPublishBehavior", ErrMissingCollectorBehavior)
	case !hasActionLog:
		return fmt.Errorf("%w: ActionLogBehavior", ErrMissingCollectorBehavior)
	case !hasEventPublish:
		return fmt.Errorf("%w: EventPublishBehavior", ErrMissingCollectorBehavior)
	}

	return nil
}
