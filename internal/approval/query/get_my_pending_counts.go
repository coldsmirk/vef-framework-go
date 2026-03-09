package query

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/my"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// GetMyPendingCountsQuery retrieves pending task and unread CC counts for the current user.
type GetMyPendingCountsQuery struct {
	cqrs.BaseQuery

	UserID string
}

// GetMyPendingCountsHandler handles the GetMyPendingCountsQuery.
type GetMyPendingCountsHandler struct {
	db orm.DB
}

// NewGetMyPendingCountsHandler creates a new GetMyPendingCountsHandler.
func NewGetMyPendingCountsHandler(db orm.DB) *GetMyPendingCountsHandler {
	return &GetMyPendingCountsHandler{db: db}
}

func (h *GetMyPendingCountsHandler) Handle(ctx context.Context, query GetMyPendingCountsQuery) (*my.PendingCounts, error) {
	db := contextx.DB(ctx, h.db)

	pendingCount, err := db.NewSelect().
		Model((*approval.Task)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("assignee_id", query.UserID).
				Equals("status", string(approval.TaskPending))
		}).
		Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("count pending tasks: %w", err)
	}

	unreadCCCount, err := db.NewSelect().
		Model((*approval.CCRecord)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("cc_user_id", query.UserID).
				IsNull("read_at")
		}).
		Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("count unread cc records: %w", err)
	}

	return &my.PendingCounts{
		PendingTaskCount: int(pendingCount),
		UnreadCCCount:    int(unreadCCCount),
	}, nil
}
