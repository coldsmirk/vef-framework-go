package query

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/page"
)

// FindFlowsQuery queries flows for admin management.
type FindFlowsQuery struct {
	cqrs.BaseQuery
	page.Pageable

	TenantID   string
	CategoryID string
	Keyword    string
	IsActive   *bool
}

// FindFlowsHandler handles the FindFlowsQuery.
type FindFlowsHandler struct {
	db orm.DB
}

// NewFindFlowsHandler creates a new FindFlowsHandler.
func NewFindFlowsHandler(db orm.DB) *FindFlowsHandler {
	return &FindFlowsHandler{db: db}
}

func (h *FindFlowsHandler) Handle(ctx context.Context, query FindFlowsQuery) (*page.Page[approval.Flow], error) {
	db := contextx.DB(ctx, h.db)

	var flows []approval.Flow

	sq := db.NewSelect().Model(&flows).
		Where(func(cb orm.ConditionBuilder) {
			cb.ApplyIf(query.TenantID != "", func(cb orm.ConditionBuilder) {
				cb.Equals("tenant_id", query.TenantID)
			}).
				ApplyIf(query.CategoryID != "", func(cb orm.ConditionBuilder) {
					cb.Equals("category_id", query.CategoryID)
				}).
				ApplyIf(query.IsActive != nil, func(cb orm.ConditionBuilder) {
					cb.Equals("is_active", *query.IsActive)
				}).
				ApplyIf(query.Keyword != "", func(cb orm.ConditionBuilder) {
					cb.Contains("name", query.Keyword)
				})
		}).
		OrderBy("name")

	query.Normalize(20)
	sq = sq.Limit(query.Size).Offset(query.Offset())

	count, err := sq.ScanAndCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("query flows: %w", err)
	}

	result := page.New(query.Pageable, count, flows)

	return &result, nil
}
