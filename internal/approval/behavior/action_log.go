package behavior

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// ActionLogCollector is the request-scoped buffer for approval ActionLog
// entries. Alias of Collector[*approval.ActionLog] for call-site clarity.
type ActionLogCollector = Collector[*approval.ActionLog]

// NewActionLogBehavior buffers ActionLog entries produced by a command
// handler and inserts them in a single batch after the handler succeeds.
// Failed handlers short-circuit so audit rows never describe a non-event.
//
// Order positions the behavior between Transaction (outermost) and
// EventPublish (innermost), so logs persist inside the tx but before
// events emit.
func NewActionLogBehavior(db orm.DB) cqrs.Behavior {
	return &collectorBehavior[*approval.ActionLog]{
		order: 100,
		name:  "action log",
		flush: func(ctx context.Context, entries []*approval.ActionLog) error {
			tx := contextx.DB(ctx, db)
			_, err := tx.NewInsert().Model(&entries).Exec(ctx)

			return err
		},
	}
}

// ActionLogCollectorFromContext returns the request-scoped collector or a
// detached no-op collector (with a warning) when called outside the CQRS
// pipeline.
func ActionLogCollectorFromContext(ctx context.Context) *ActionLogCollector {
	return collectorFromContextOrWarn[*approval.ActionLog](ctx, "ActionLogCollector", "ActionLogBehavior")
}

// TryActionLogCollectorFromContext returns the collector silently when
// missing, for engine paths that may legitimately run outside the CQRS
// pipeline (e.g. the timeout scanner) and fall back to event-only auditing.
func TryActionLogCollectorFromContext(ctx context.Context) (*ActionLogCollector, bool) {
	return TryCollectorFromContext[*approval.ActionLog](ctx)
}
