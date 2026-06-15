package query

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/my"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/page"
)

// FindMyPendingTasksQuery queries pending tasks assigned to the current user.
type FindMyPendingTasksQuery struct {
	cqrs.BaseQuery
	page.Pageable

	UserID string
	// TenantID is a self-scoped narrowing filter, NOT an authorization
	// boundary: rows are already pinned to UserID, so it only narrows the
	// caller's own tasks and does not gate access.
	TenantID *string
}

// FindMyPendingTasksHandler handles the FindMyPendingTasksQuery.
type FindMyPendingTasksHandler struct {
	db orm.DB
}

// NewFindMyPendingTasksHandler creates a new FindMyPendingTasksHandler.
func NewFindMyPendingTasksHandler(db orm.DB) *FindMyPendingTasksHandler {
	return &FindMyPendingTasksHandler{db: db}
}

func (h *FindMyPendingTasksHandler) Handle(ctx context.Context, query FindMyPendingTasksQuery) (*page.Page[my.PendingTask], error) {
	db := contextx.DB(ctx, h.db)

	var tasks []approval.Task

	sq := db.NewSelect().Model(&tasks).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("assignee_id", query.UserID).
				Equals("status", string(approval.TaskPending)).
				ApplyIf(query.TenantID != nil, func(cb orm.ConditionBuilder) {
					cb.Equals("tenant_id", *query.TenantID)
				})
		}).
		OrderByDesc("created_at")

	sq = applyPageable(sq, &query.Pageable)

	count, err := sq.ScanAndCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("query pending tasks: %w", err)
	}

	if len(tasks) == 0 {
		result := page.New(query.Pageable, count, []my.PendingTask{})

		return &result, nil
	}

	// Collect instance IDs and node IDs for batch lookup.
	instanceIDs := make([]string, 0, len(tasks))

	nodeIDs := make([]string, 0, len(tasks))
	for _, t := range tasks {
		instanceIDs = append(instanceIDs, t.InstanceID)
		nodeIDs = append(nodeIDs, t.NodeID)
	}

	instanceMap, flowMap, nodeMap, err := loadEnrichmentMaps(ctx, db, instanceIDs, nodeIDs)
	if err != nil {
		return nil, err
	}

	items := make([]my.PendingTask, len(tasks))
	for i, t := range tasks {
		item := my.PendingTask{
			TaskID:    t.ID,
			NodeName:  nodeMap[t.NodeID],
			CreatedAt: t.CreatedAt,
			Deadline:  t.Deadline,
			IsTimeout: t.IsTimeout,
		}
		if inst := instanceMap[t.InstanceID]; inst != nil {
			item.InstanceID = inst.ID
			item.InstanceTitle = inst.Title
			item.InstanceNo = inst.InstanceNo

			item.ApplicantName = inst.ApplicantName
			if flow := flowMap[inst.FlowID]; flow != nil {
				item.FlowName = flow.Name
				item.FlowIcon = flow.Icon
			}
		}

		items[i] = item
	}

	result := page.New(query.Pageable, count, items)

	return &result, nil
}
