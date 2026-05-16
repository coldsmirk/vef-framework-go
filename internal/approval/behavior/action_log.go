package behavior

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// ActionLogCollector buffers ActionLog entries produced by a command handler.
// Like EventCollector, it lets handlers describe "what was done" without
// touching the database; ActionLogBehavior flushes the buffer in a single
// INSERT after the handler returns successfully.
type ActionLogCollector struct {
	entries []*approval.ActionLog
}

// Add records additional ActionLog entries. Nil entries are dropped.
// Handlers usually call Add exactly once but multi-step actions (e.g.
// admin force-reassign with a side note) may push more than one.
func (c *ActionLogCollector) Add(entries ...*approval.ActionLog) {
	for _, entry := range entries {
		if entry == nil {
			continue
		}

		c.entries = append(c.entries, entry)
	}
}

// Entries returns the buffered logs. Callers should not mutate the slice.
func (c *ActionLogCollector) Entries() []*approval.ActionLog {
	return c.entries
}

type actionLogCollectorKey struct{}

// ActionLogCollectorFromContext returns the request-scoped ActionLogCollector
// or a detached no-op collector when called outside the CQRS pipeline.
func ActionLogCollectorFromContext(ctx context.Context) *ActionLogCollector {
	if c, ok := ctx.Value(actionLogCollectorKey{}).(*ActionLogCollector); ok {
		return c
	}

	return new(ActionLogCollector)
}

// ActionLogBehavior installs a per-command ActionLogCollector into the
// context and flushes whatever the handler accumulated when the handler
// returns successfully. Queries pass through untouched. Errors short-
// circuit the persist so failed commands leave no audit residue.
type ActionLogBehavior struct {
	db orm.DB
}

// NewActionLogBehavior constructs the behavior.
func NewActionLogBehavior(db orm.DB) cqrs.Behavior {
	return &ActionLogBehavior{db: db}
}

// Handle wraps the handler with collector lifecycle management.
func (b *ActionLogBehavior) Handle(ctx context.Context, action cqrs.Action, next func(context.Context) (any, error)) (any, error) {
	if action.Kind() == cqrs.Query {
		return next(ctx)
	}

	collector := &ActionLogCollector{}
	ctx = context.WithValue(ctx, actionLogCollectorKey{}, collector)

	result, err := next(ctx)
	if err != nil {
		return nil, err
	}

	if len(collector.entries) == 0 {
		return result, nil
	}

	db := contextx.DB(ctx, b.db)
	if _, err := db.NewInsert().Model(&collector.entries).Exec(ctx); err != nil {
		return nil, fmt.Errorf("insert action logs: %w", err)
	}

	return result, nil
}
