package query

import (
	"context"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/my"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

// GetMyInstanceDetailQuery retrieves instance detail with access control for the current user.
type GetMyInstanceDetailQuery struct {
	cqrs.BaseQuery

	InstanceID string
	UserID     string
}

// GetMyInstanceDetailHandler handles the GetMyInstanceDetailQuery.
type GetMyInstanceDetailHandler struct {
	db      orm.DB
	taskSvc *service.TaskService
}

// NewGetMyInstanceDetailHandler creates a new GetMyInstanceDetailHandler.
func NewGetMyInstanceDetailHandler(db orm.DB, taskSvc *service.TaskService) *GetMyInstanceDetailHandler {
	return &GetMyInstanceDetailHandler{db: db, taskSvc: taskSvc}
}

func (h *GetMyInstanceDetailHandler) Handle(ctx context.Context, query GetMyInstanceDetailQuery) (*my.InstanceDetail, error) {
	db := contextx.DB(ctx, h.db)

	// Load instance.
	var instance approval.Instance

	instance.ID = query.InstanceID

	if err := db.NewSelect().Model(&instance).WherePK().Scan(ctx); err != nil {
		if result.IsRecordNotFound(err) {
			return nil, shared.ErrInstanceNotFound
		}

		return nil, fmt.Errorf("query instance: %w", err)
	}

	// Check participant membership.
	isParticipant, err := h.taskSvc.IsInstanceParticipant(ctx, db, query.InstanceID, query.UserID)
	if err != nil {
		return nil, err
	}

	if !isParticipant {
		return nil, shared.ErrAccessDenied
	}

	// Load flow.
	var flow approval.Flow

	flow.ID = instance.FlowID
	if err := db.NewSelect().Model(&flow).WherePK().Scan(ctx); err != nil && !result.IsRecordNotFound(err) {
		return nil, fmt.Errorf("query flow: %w", err)
	}

	// Load tasks.
	var tasks []approval.Task
	if err := db.NewSelect().Model(&tasks).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", query.InstanceID) }).
		OrderBy("sort_order").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}

	// Load action logs.
	var actionLogs []approval.ActionLog
	if err := db.NewSelect().Model(&actionLogs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", query.InstanceID) }).
		OrderBy("created_at").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query action logs: %w", err)
	}

	// Load flow nodes.
	var flowNodes []approval.FlowNode
	if err := db.NewSelect().Model(&flowNodes).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("flow_version_id", instance.FlowVersionID) }).
		OrderBy("created_at").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("query flow nodes: %w", err)
	}

	nodeNameMap := make(map[string]string, len(flowNodes))
	for _, n := range flowNodes {
		nodeNameMap[n.ID] = n.Name
	}

	// Build DTO.
	detail := &my.InstanceDetail{
		Instance: my.InstanceInfo{
			InstanceID:       instance.ID,
			InstanceNo:       instance.InstanceNo,
			Title:            instance.Title,
			FlowName:         flow.Name,
			FlowIcon:         flow.Icon,
			ApplicantID:      instance.ApplicantID,
			ApplicantName:    instance.ApplicantName,
			Status:           string(instance.Status),
			BusinessRecordID: instance.BusinessRecordID,
			FormData:         instance.FormData,
			CreatedAt:        instance.CreatedAt,
			FinishedAt:       instance.FinishedAt,
		},
		Tasks:            make([]my.TaskInfo, len(tasks)),
		ActionLogs:       make([]my.ActionLogInfo, len(actionLogs)),
		FlowNodes:        make([]my.FlowNodeInfo, len(flowNodes)),
		AvailableActions: h.computeActions(instance, tasks, flowNodes, query.UserID),
	}

	if instance.CurrentNodeID != nil {
		if name, ok := nodeNameMap[*instance.CurrentNodeID]; ok {
			detail.Instance.CurrentNodeName = &name
		}
	}

	for i, t := range tasks {
		detail.Tasks[i] = my.TaskInfo{
			TaskID:       t.ID,
			NodeName:     nodeNameMap[t.NodeID],
			AssigneeID:   t.AssigneeID,
			AssigneeName: t.AssigneeName,
			Status:       string(t.Status),
			SortOrder:    t.SortOrder,
			CreatedAt:    t.CreatedAt,
			FinishedAt:   t.FinishedAt,
		}
	}

	for i, log := range actionLogs {
		detail.ActionLogs[i] = my.ActionLogInfo{
			Action:       string(log.Action),
			OperatorName: log.OperatorName,
			Opinion:      log.Opinion,
			CreatedAt:    log.CreatedAt,
		}
	}

	for i, n := range flowNodes {
		detail.FlowNodes[i] = my.FlowNodeInfo{
			NodeID: n.ID,
			Key:    n.Key,
			Kind:   string(n.Kind),
			Name:   n.Name,
		}
	}

	return detail, nil
}

// computeActions determines the available actions for the user on this instance.
func (*GetMyInstanceDetailHandler) computeActions(
	instance approval.Instance,
	tasks []approval.Task,
	flowNodes []approval.FlowNode,
	userID string,
) []string {
	actions := shared.NewOrderedUnique[string](8)

	nodeByID := make(map[string]*approval.FlowNode, len(flowNodes))
	for i := range flowNodes {
		nodeByID[flowNodes[i].ID] = &flowNodes[i]
	}

	isApplicant := instance.ApplicantID == userID

	if isApplicant && instance.Status == approval.InstanceRunning {
		actions.Add("withdraw")
	}

	if isApplicant && (instance.Status == approval.InstanceRejected || instance.Status == approval.InstanceReturned) {
		actions.Add("resubmit")
	}

	hasPendingTask := false

	for _, t := range tasks {
		if t.Status != approval.TaskPending {
			continue
		}

		hasPendingTask = true

		if t.AssigneeID != userID {
			continue
		}

		node := nodeByID[t.NodeID]
		if node != nil && node.Kind == approval.NodeHandle {
			actions.Add("handle")
		} else {
			actions.Add("approve")
		}

		actions.Add("reject")

		if node == nil {
			continue
		}

		if node.IsTransferAllowed {
			actions.Add("transfer")
		}

		if node.IsRollbackAllowed {
			actions.Add("rollback")
		}

		if node.IsAddAssigneeAllowed {
			actions.Add("add_assignee")
		}

		if node.IsManualCCAllowed {
			actions.Add("add_cc")
		}
	}

	if hasPendingTask {
		actions.Add("urge")
	}

	return actions.ToSlice()
}
