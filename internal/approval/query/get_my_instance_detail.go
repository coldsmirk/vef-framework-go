package query

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/approval/my"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
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

	bundle, err := loadInstanceDetailBundle(ctx, db, query.InstanceID)
	if err != nil {
		return nil, err
	}

	// Check participant membership.
	isParticipant, err := h.taskSvc.IsInstanceParticipant(ctx, db, query.InstanceID, query.UserID)
	if err != nil {
		return nil, err
	}

	if !isParticipant {
		return nil, shared.ErrAccessDenied
	}

	// Build DTO.
	instance := bundle.Instance
	flow := bundle.Flow
	tasks := bundle.Tasks
	actionLogs := bundle.ActionLogs
	flowNodes := bundle.FlowNodes
	nodeNameMap := bundle.NodeNameMap

	detail := &my.InstanceDetail{
		Instance: my.InstanceInfo{
			InstanceID:              instance.ID,
			InstanceNo:              instance.InstanceNo,
			Title:                   instance.Title,
			FlowName:                flow.Name,
			FlowIcon:                flow.Icon,
			ApplicantID:             instance.ApplicantID,
			ApplicantName:           instance.ApplicantName,
			ApplicantDepartmentName: instance.ApplicantDepartmentName,
			Status:                  string(instance.Status),
			CurrentNodeID:           instance.CurrentNodeID,
			BusinessRecordID:        instance.BusinessRecordID,
			FormData:                instance.FormData,
			FormSchema:              bundle.FormSchema,
			CreatedAt:               instance.CreatedAt,
			FinishedAt:              instance.FinishedAt,
		},
		Tasks:            make([]my.TaskInfo, len(tasks)),
		ActionLogs:       make([]my.ActionLogInfo, len(actionLogs)),
		FlowGraph:        buildInstanceFlowGraph(bundle),
		AvailableActions: h.computeActions(instance, tasks, flowNodes, query.UserID),
	}

	if instance.CurrentNodeID != nil {
		if name, ok := nodeNameMap[*instance.CurrentNodeID]; ok {
			detail.Instance.CurrentNodeName = &name
		}
	}

	for i, t := range tasks {
		detail.Tasks[i] = my.TaskInfo{
			TaskID:        t.ID,
			NodeID:        t.NodeID,
			NodeName:      nodeNameMap[t.NodeID],
			AssigneeID:    t.AssigneeID,
			AssigneeName:  t.AssigneeName,
			DelegatorID:   t.DelegatorID,
			DelegatorName: t.DelegatorName,
			Status:        string(t.Status),
			SortOrder:     t.SortOrder,
			Deadline:      t.Deadline,
			IsTimeout:     t.IsTimeout,
			CreatedAt:     t.CreatedAt,
			FinishedAt:    t.FinishedAt,
		}
	}

	for i, log := range actionLogs {
		detail.ActionLogs[i] = my.ActionLogInfo{
			LogID:                  log.ID,
			Action:                 string(log.Action),
			NodeID:                 log.NodeID,
			OperatorID:             log.OperatorID,
			OperatorName:           log.OperatorName,
			OperatorDepartmentName: log.OperatorDepartmentName,
			TransferToID:           log.TransferToID,
			TransferToName:         log.TransferToName,
			RollbackToNodeID:       log.RollbackToNodeID,
			AddedAssignees:         zipUserBriefs(log.AddedAssigneeIDs, log.AddedAssigneeNames),
			RemovedAssignees:       zipUserBriefs(log.RemovedAssigneeIDs, log.RemovedAssigneeNames),
			CCUsers:                zipUserBriefs(log.CCUserIDs, log.CCUserNames),
			Opinion:                log.Opinion,
			Attachments:            log.Attachments,
			CreatedAt:              log.CreatedAt,
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

	// Applicant lifecycle actions are derived from the instance state machine
	// so the offered set can never drift from what the command handlers accept
	// (withdraw covers both "cancel a running instance" and "abandon a
	// returned one"; resubmit reactivates returned / withdrawn instances).
	if isApplicant && engine.InstanceStateMachine.CanTransition(instance.Status, approval.InstanceWithdrawn) {
		actions.Add("withdraw")
	}

	if isApplicant && engine.InstanceStateMachine.CanTransition(instance.Status, approval.InstanceRunning) {
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
