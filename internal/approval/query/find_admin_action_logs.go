package query

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/admin"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/page"
)

// FindAdminActionLogsQuery queries action logs for an instance with pagination.
type FindAdminActionLogsQuery struct {
	cqrs.BaseQuery
	page.Pageable

	InstanceID string
	TenantID   *string
}

// FindAdminActionLogsHandler handles the FindAdminActionLogsQuery.
type FindAdminActionLogsHandler struct {
	db orm.DB
}

// NewFindAdminActionLogsHandler creates a new FindAdminActionLogsHandler.
func NewFindAdminActionLogsHandler(db orm.DB) *FindAdminActionLogsHandler {
	return &FindAdminActionLogsHandler{db: db}
}

func (h *FindAdminActionLogsHandler) Handle(ctx context.Context, query FindAdminActionLogsQuery) (*page.Page[admin.ActionLog], error) {
	db := contextx.DB(ctx, h.db)

	if query.TenantID != nil {
		exists, err := db.NewSelect().
			Model((*approval.Instance)(nil)).
			Where(func(cb orm.ConditionBuilder) {
				cb.PKEquals(query.InstanceID).
					Equals("tenant_id", *query.TenantID)
			}).
			Exists(ctx)
		if err != nil {
			return nil, fmt.Errorf("check instance tenant: %w", err)
		}

		if !exists {
			return nil, shared.ErrInstanceNotFound
		}
	}

	var logs []approval.ActionLog

	sq := db.NewSelect().Model(&logs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", query.InstanceID) }).
		OrderBy("created_at")

	query.Normalize(20)
	sq = sq.Limit(query.Size).Offset(query.Offset())

	count, err := sq.ScanAndCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("query admin action logs: %w", err)
	}

	if len(logs) == 0 {
		result := page.New(query.Pageable, count, []admin.ActionLog{})

		return &result, nil
	}

	items := make([]admin.ActionLog, len(logs))
	for i, log := range logs {
		items[i] = admin.ActionLog{
			LogID:            log.ID,
			Action:           string(log.Action),
			OperatorID:       log.OperatorID,
			OperatorName:     log.OperatorName,
			OperatorDeptName: log.OperatorDeptName,
			TransferToID:     log.TransferToID,
			TransferToName:   log.TransferToName,
			Opinion:          log.Opinion,
			CreatedAt:        log.CreatedAt,
		}
	}

	result := page.New(query.Pageable, count, items)

	return &result, nil
}
