package query

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/admin"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// GetMetricsQuery aggregates instance / task / timeout counts for the admin
// dashboard. TenantID scopes the result; an empty TenantID returns a cross-
// tenant snapshot and should be reserved for super-admin callers (the
// resource layer enforces this).
type GetMetricsQuery struct {
	cqrs.BaseQuery

	TenantID string
}

// GetMetricsHandler handles the GetMetricsQuery.
type GetMetricsHandler struct {
	db orm.DB
}

// NewGetMetricsHandler creates a new GetMetricsHandler.
func NewGetMetricsHandler(db orm.DB) *GetMetricsHandler {
	return &GetMetricsHandler{db: db}
}

type metricStatusRow struct {
	Status string `bun:"status"`
	Count  int64  `bun:"count"`
}

func (h *GetMetricsHandler) Handle(ctx context.Context, query GetMetricsQuery) (*admin.Metrics, error) {
	db := contextx.DB(ctx, h.db)

	metrics := &admin.Metrics{
		TenantID:             query.TenantID,
		CapturedAt:           timex.Now(),
		InstanceCounts:       map[string]int{},
		TaskCounts:           map[string]int{},
		AvgCompletionSeconds: -1,
	}

	var instanceRows []metricStatusRow
	if err := db.NewSelect().
		Model((*approval.Instance)(nil)).
		Select("status").
		SelectExpr(func(eb orm.ExprBuilder) any { return eb.CountAll() }, "count").
		Where(func(cb orm.ConditionBuilder) {
			cb.ApplyIf(query.TenantID != "", func(cb orm.ConditionBuilder) {
				cb.Equals("tenant_id", query.TenantID)
			})
		}).
		GroupBy("status").
		Scan(ctx, &instanceRows); err != nil {
		return nil, fmt.Errorf("query instance counts: %w", err)
	}

	for _, row := range instanceRows {
		metrics.InstanceCounts[row.Status] = int(row.Count)
	}

	var taskRows []metricStatusRow
	if err := db.NewSelect().
		Model((*approval.Task)(nil)).
		Select("status").
		SelectExpr(func(eb orm.ExprBuilder) any { return eb.CountAll() }, "count").
		Where(func(cb orm.ConditionBuilder) {
			cb.ApplyIf(query.TenantID != "", func(cb orm.ConditionBuilder) {
				cb.Equals("tenant_id", query.TenantID)
			})
		}).
		GroupBy("status").
		Scan(ctx, &taskRows); err != nil {
		return nil, fmt.Errorf("query task counts: %w", err)
	}

	for _, row := range taskRows {
		metrics.TaskCounts[row.Status] = int(row.Count)
	}

	timeoutCount, err := db.NewSelect().
		Model((*approval.Task)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("status", approval.TaskPending).
				IsTrue("is_timeout").
				ApplyIf(query.TenantID != "", func(cb orm.ConditionBuilder) {
					cb.Equals("tenant_id", query.TenantID)
				})
		}).
		Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("query timeout task count: %w", err)
	}

	metrics.TimeoutTaskCount = int(timeoutCount)

	return metrics, nil
}
