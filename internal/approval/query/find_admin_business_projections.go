package query

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/admin"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/page"
)

// FindAdminBusinessProjectionsQuery lists durable business projection state.
type FindAdminBusinessProjectionsQuery struct {
	cqrs.BaseQuery
	page.Pageable

	TenantID *string
	Status   *approval.BindingProjectionStatus
}

// FindAdminBusinessProjectionsHandler handles projection administration
// queries.
type FindAdminBusinessProjectionsHandler struct {
	db orm.DB
}

// NewFindAdminBusinessProjectionsHandler creates a projection query handler.
func NewFindAdminBusinessProjectionsHandler(db orm.DB) *FindAdminBusinessProjectionsHandler {
	return &FindAdminBusinessProjectionsHandler{db: db}
}

func (h *FindAdminBusinessProjectionsHandler) Handle(
	ctx context.Context,
	query FindAdminBusinessProjectionsQuery,
) (*page.Page[admin.BusinessProjection], error) {
	db := contextx.DB(ctx, h.db)

	var projections []approval.BusinessProjection

	sq := db.NewSelect().
		Model(&projections).
		Where(func(cb orm.ConditionBuilder) {
			cb.ApplyIf(query.TenantID != nil, func(cb orm.ConditionBuilder) {
				cb.Equals("tenant_id", *query.TenantID)
			}).ApplyIf(query.Status != nil, func(cb orm.ConditionBuilder) {
				cb.Equals("status", *query.Status)
			})
		}).
		OrderByDesc("updated_at", "id")

	sq = applyPageable(sq, &query.Pageable)

	count, err := sq.ScanAndCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("query admin business projections: %w", err)
	}

	items := make([]admin.BusinessProjection, len(projections))
	for i := range projections {
		projection := &projections[i]

		businessTable := ""
		if projection.Binding != nil {
			businessTable = projection.Binding.TableName
		}

		items[i] = admin.BusinessProjection{
			ProjectionID:           projection.ID,
			TenantID:               projection.TenantID,
			FlowID:                 projection.FlowID,
			FlowVersionID:          projection.FlowVersionID,
			OwnerInstanceID:        projection.OwnerInstanceID,
			AppliedOwnerInstanceID: projection.AppliedOwnerInstanceID,
			BusinessTable:          businessTable,
			RecordKey:              projection.RecordKey,
			Consistency:            projection.Consistency,
			DesiredStatus:          projection.DesiredStatus,
			DesiredStartedAt:       projection.DesiredStartedAt,
			DesiredFinishedAt:      projection.DesiredFinishedAt,
			DesiredRevision:        projection.DesiredRevision,
			AppliedRevision:        projection.AppliedRevision,
			Status:                 projection.Status,
			AttemptCount:           projection.AttemptCount,
			NextAttemptAt:          projection.NextAttemptAt,
			LeaseUntil:             projection.LeaseUntil,
			LastError:              projection.LastError,
			AppliedAt:              projection.AppliedAt,
			UpdatedAt:              projection.UpdatedAt,
		}
	}

	result := page.New(query.Pageable, count, items)

	return &result, nil
}
