package query

import (
	"context"

	"github.com/coldsmirk/go-collections"

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
		return nil, approval.ErrAccessDenied
	}

	// Build DTO.
	instance := bundle.Instance
	flow := bundle.Flow

	// Resolve the viewer-scoped field permissions once — they both ship in the
	// DTO and gate which form-data fields this viewer is allowed to see.
	fieldPermissions := resolveViewerFieldPermissions(bundle, query.UserID)

	detail := &my.InstanceDetail{
		Instance: my.InstanceInfo{
			InstanceID:    instance.ID,
			InstanceNo:    instance.InstanceNo,
			Title:         instance.Title,
			FlowID:        instance.FlowID,
			FlowCode:      instance.FlowCode,
			FlowName:      flow.Name,
			FlowIcon:      flow.Icon,
			Labels:        flow.Labels,
			Applicant:     instance.Applicant(),
			Status:        string(instance.Status),
			CurrentNodeID: instance.CurrentNodeID,
			BusinessRef:   instance.BusinessRef,
			FormData:      stripHiddenFormData(instance.FormData, fieldPermissions),
			CreatedAt:     instance.CreatedAt,
			FinishedAt:    instance.FinishedAt,
		},
		FormSchema:       bundle.FormSchema,
		Timeline:         buildInstanceTimeline(bundle),
		FlowGraph:        buildInstanceFlowGraph(bundle),
		AvailableActions: h.computeActions(instance, bundle.Tasks, bundle.FlowNodes, query.UserID),
		FieldPermissions: fieldPermissions,
		MyTask:           buildViewerTask(bundle, query.UserID),
	}

	if instance.CurrentNodeID != nil {
		if name, ok := bundle.NodeNameMap[*instance.CurrentNodeID]; ok {
			detail.Instance.CurrentNodeName = &name
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
	hasOwnOrDelegatedTask := false

	for _, t := range tasks {
		if t.AssigneeID == userID || (t.DelegatorID != nil && *t.DelegatorID == userID) {
			hasOwnOrDelegatedTask = true
		}

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

		if node.IsRemoveAssigneeAllowed {
			actions.Add("remove_assignee")
		}

		if node.IsManualCCAllowed {
			actions.Add("add_cc")
		}
	}

	// Urge mirrors TaskService.IsUrgeAuthorized: the applicant, or anyone a
	// task was ever opened on — held directly or delegated away, the slot stays
	// theirs. CC-only viewers are excluded, so the offered set cannot drift
	// from what UrgeTaskHandler accepts.
	if hasPendingTask && (isApplicant || hasOwnOrDelegatedTask) {
		actions.Add("urge")
	}

	return actions.ToSlice()
}

// buildViewerTask locates the viewer's pending task and packages the node's
// action configuration so the client never re-derives engine semantics. Tasks
// arrive in sort order; when a viewer somehow holds several pending tasks the
// first is the actionable one (mirroring the queue position semantics).
func buildViewerTask(bundle *instanceDetailBundle, userID string) *my.ViewerTask {
	var task *approval.Task

	for i := range bundle.Tasks {
		t := &bundle.Tasks[i]
		if t.AssigneeID == userID && t.Status == approval.TaskPending {
			task = t

			break
		}
	}

	if task == nil {
		return nil
	}

	viewer := &my.ViewerTask{
		TaskID: task.ID,
		NodeID: task.NodeID,
	}

	var node *approval.FlowNode

	for i := range bundle.FlowNodes {
		if bundle.FlowNodes[i].ID == task.NodeID {
			node = &bundle.FlowNodes[i]

			break
		}
	}

	if node == nil {
		return viewer
	}

	viewer.IsOpinionRequired = node.IsOpinionRequired

	if node.IsAddAssigneeAllowed {
		viewer.AddAssigneeTypes = node.AddAssigneeTypes
	}

	if node.IsRollbackAllowed {
		viewer.RollbackTargets = resolveRollbackTargets(bundle, node)
	}

	if node.IsRemoveAssigneeAllowed {
		viewer.RemovableAssignees = resolveRemovableAssignees(bundle.Tasks, task)
	}

	return viewer
}

// resolveRemovableAssignees mirrors the remove-assignee command's target
// eligibility over the already-loaded detail bundle: still-actionable peers
// (pending / waiting) of the viewer's own visit. The viewer's task is excluded
// — with it staying actionable, every listed peer also passes the command's
// last-assignee simulation.
func resolveRemovableAssignees(tasks []approval.Task, own *approval.Task) []my.RemovableAssignee {
	var removable []my.RemovableAssignee

	for i := range tasks {
		task := &tasks[i]
		if task.ID == own.ID || task.VisitID != own.VisitID {
			continue
		}

		if task.Status != approval.TaskPending && task.Status != approval.TaskWaiting {
			continue
		}

		removable = append(removable, my.RemovableAssignee{
			TaskID:   task.ID,
			Assignee: task.Assignee(),
			Status:   string(task.Status),
		})
	}

	return removable
}

// resolveRollbackTargets mirrors ValidationService.ValidateRollbackTarget over
// the already-loaded detail bundle: the returned set is exactly what the
// rollback command would accept, so the client's target picker cannot offer a
// destination the engine rejects.
func resolveRollbackTargets(bundle *instanceDetailBundle, current *approval.FlowNode) []my.RollbackTarget {
	nodeByID := make(map[string]*approval.FlowNode, len(bundle.FlowNodes))
	nodeByKey := make(map[string]*approval.FlowNode, len(bundle.FlowNodes))

	for i := range bundle.FlowNodes {
		node := &bundle.FlowNodes[i]
		nodeByID[node.ID] = node
		nodeByKey[node.Key] = node
	}

	concluded := collections.NewHashSet[string]()

	for _, visit := range bundle.Visits {
		switch visit.Status {
		case approval.NodeVisitPassed, approval.NodeVisitRejected, approval.NodeVisitReturned:
			concluded.Add(visit.NodeID)
		}
	}

	var candidates []*approval.FlowNode

	switch current.RollbackType {
	case approval.RollbackPrevious:
		// Direct graph predecessors, matching the command's edge lookup.
		if bundle.FlowSchema != nil {
			for _, edge := range bundle.FlowSchema.Edges {
				if target := nodeByKey[edge.Target]; target != nil && target.ID == current.ID {
					if source := nodeByKey[edge.Source]; source != nil {
						candidates = append(candidates, source)
					}
				}
			}
		}

	case approval.RollbackStart:
		for _, node := range nodeByID {
			if node.Kind == approval.NodeStart {
				candidates = append(candidates, node)

				break
			}
		}

	case approval.RollbackAny:
		// Bounded by the visit trail: decision points (approval / handle) or
		// the start node the instance actually traversed.
		for i := range bundle.FlowNodes {
			node := &bundle.FlowNodes[i]

			switch node.Kind {
			case approval.NodeApproval, approval.NodeHandle, approval.NodeStart:
				if concluded.Contains(node.ID) {
					candidates = append(candidates, node)
				}
			}
		}

	case approval.RollbackSpecified:
		for _, key := range current.RollbackTargetKeys {
			if node := nodeByKey[key]; node != nil && concluded.Contains(node.ID) {
				candidates = append(candidates, node)
			}
		}

	case approval.RollbackNone:
	}

	targets := make([]my.RollbackTarget, 0, len(candidates))
	seen := collections.NewHashSet[string]()

	for _, node := range candidates {
		if node.ID == current.ID || seen.Contains(node.ID) {
			continue
		}

		seen.Add(node.ID)
		targets = append(targets, my.RollbackTarget{NodeID: node.ID, Name: node.Name})
	}

	return targets
}
