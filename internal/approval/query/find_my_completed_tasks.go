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

// completedStatuses lists the task statuses considered "completed" for user-facing queries.
var completedStatuses = []string{
	string(approval.TaskApproved),
	string(approval.TaskRejected),
	string(approval.TaskHandled),
	string(approval.TaskTransferred),
	string(approval.TaskRolledBack),
}

// FindMyCompletedTasksQuery queries tasks already processed by the current user.
type FindMyCompletedTasksQuery struct {
	cqrs.BaseQuery
	page.Pageable

	UserID   string
	TenantID *string
}

// FindMyCompletedTasksHandler handles the FindMyCompletedTasksQuery.
type FindMyCompletedTasksHandler struct {
	db orm.DB
}

// NewFindMyCompletedTasksHandler creates a new FindMyCompletedTasksHandler.
func NewFindMyCompletedTasksHandler(db orm.DB) *FindMyCompletedTasksHandler {
	return &FindMyCompletedTasksHandler{db: db}
}

func (h *FindMyCompletedTasksHandler) Handle(ctx context.Context, query FindMyCompletedTasksQuery) (*page.Page[my.CompletedTask], error) {
	db := contextx.DB(ctx, h.db)

	var tasks []approval.Task

	sq := db.NewSelect().Model(&tasks).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("assignee_id", query.UserID).
				In("status", completedStatuses).
				ApplyIf(query.TenantID != nil, func(cb orm.ConditionBuilder) {
					cb.Equals("tenant_id", *query.TenantID)
				})
		}).
		OrderByDesc("finished_at")

	query.Normalize(20)
	sq = sq.Limit(query.Size).Offset(query.Offset())

	count, err := sq.ScanAndCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("query completed tasks: %w", err)
	}

	if len(tasks) == 0 {
		result := page.New(query.Pageable, count, []my.CompletedTask{})

		return &result, nil
	}

	instanceIDs := make([]string, 0, len(tasks))

	nodeIDs := make([]string, 0, len(tasks))
	for _, t := range tasks {
		instanceIDs = append(instanceIDs, t.InstanceID)
		nodeIDs = append(nodeIDs, t.NodeID)
	}

	instanceMap, err := loadInstanceMap(ctx, db, instanceIDs)
	if err != nil {
		return nil, err
	}

	flowIDs := make([]string, 0, len(instanceMap))
	for _, inst := range instanceMap {
		flowIDs = append(flowIDs, inst.FlowID)
	}

	flowMap, err := loadFlowMap(ctx, db, flowIDs)
	if err != nil {
		return nil, err
	}

	nodeMap, err := loadNodeNameMap(ctx, db, nodeIDs)
	if err != nil {
		return nil, err
	}

	items := make([]my.CompletedTask, len(tasks))
	for i, t := range tasks {
		item := my.CompletedTask{
			TaskID:     t.ID,
			NodeName:   nodeMap[t.NodeID],
			Status:     string(t.Status),
			FinishedAt: t.FinishedAt,
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
