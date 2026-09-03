package engine

import (
	"context"
	"fmt"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/timex"
)

// Audit reasons stamped on engine-initiated decisions. User-facing text
// follows the framework's default language, matching the timeout scanner's
// system-action opinions.
const (
	autoPassReasonExecutionType       = "节点执行类型为自动通过"
	autoRejectReasonExecutionType     = "节点执行类型为自动拒绝"
	autoPassReasonEmptyAssignee       = "无审批人，按节点配置自动通过"
	autoPassReasonConsecutiveApprover = "审批人在上一节点已通过，自动通过"
	excludeReasonSameApplicant        = "审批人与发起人相同，按节点配置回避"
	cancelReasonEntryNodePassed       = "节点已通过，剩余任务无需处理"
)

// AutoPassReasonSameApplicant is exported (unlike its siblings above) because
// the sequential queue-advance path in the service layer stamps the same
// reason when it auto-passes the applicant's seat as the queue reaches it.
const AutoPassReasonSameApplicant = "审批人与发起人相同，按节点配置自动通过"

// resolveAutoExecution short-circuits task nodes whose ExecutionType decides
// the outcome without human input. AutoPass advances past the node and emits
// NodeAutoPassedEvent so timelines render the skipped step; AutoReject
// completes the instance as rejected (handleProcessResult publishes the
// completion event). Both record a system action log when a request-scoped
// collector is present.
func resolveAutoExecution(ctx context.Context, pc *ProcessContext) (*ProcessResult, bool) {
	switch pc.Node.ExecutionType {
	case approval.ExecutionAutoPass:
		recordSystemActionLog(ctx, pc, nil, autoPassReasonExecutionType)

		return &ProcessResult{
			Action: NodeActionContinue,
			Events: []approval.DomainEvent{
				approval.NewNodeAutoPassedEvent(pc.Instance, pc.Node, autoPassReasonExecutionType),
			},
		}, true

	case approval.ExecutionAutoReject:
		recordSystemActionLog(ctx, pc, nil, autoRejectReasonExecutionType)

		return &ProcessResult{
			Action:      NodeActionComplete,
			FinalStatus: new(approval.InstanceRejected),
		}, true

	default:
		return nil, false
	}
}

// nodeAutoPassResult builds the standard "advance without tasks" result for
// rule-driven automatic passes (empty assignee, same applicant), pairing the
// advance with its audit event and system action log.
func nodeAutoPassResult(ctx context.Context, pc *ProcessContext, reason string) *ProcessResult {
	recordSystemActionLog(ctx, pc, nil, reason)

	return &ProcessResult{
		Action: NodeActionContinue,
		Events: []approval.DomainEvent{
			approval.NewNodeAutoPassedEvent(pc.Instance, pc.Node, reason),
		},
	}
}

// recordSystemActionLog appends a system-operated ActionLog entry when the
// request-scoped collector is available. task pins the entry to a specific
// task for task-scoped decisions (e.g. a consecutive-approver auto-pass);
// node-scoped decisions pass nil. Outside the CQRS pipeline (timeout scanner
// driving the engine) the collector is absent and the corresponding domain
// event remains the audit record.
func recordSystemActionLog(ctx context.Context, pc *ProcessContext, task *approval.Task, reason string) {
	collector, ok := behavior.TryActionLogCollectorFromContext(ctx)
	if !ok {
		return
	}

	entry := shared.SystemOperator.NewActionLog(pc.Instance.ID, approval.ActionExecute)
	entry.NodeID = new(pc.Node.ID)
	entry.Opinion = new(reason)

	if task != nil {
		entry.TaskID = new(task.ID)
	}

	collector.Add(entry)
}

// saveFormSnapshot persists a snapshot of the form data at the current node.
func saveFormSnapshot(ctx context.Context, pc *ProcessContext) error {
	if _, err := pc.DB.NewInsert().
		Model(&approval.FormSnapshot{
			InstanceID: pc.Instance.ID,
			NodeID:     pc.Node.ID,
			FormData:   pc.FormData.ToMap(),
		}).
		Exec(ctx); err != nil {
		return fmt.Errorf("save form snapshot: %w", err)
	}

	return nil
}

// resolveNodeAssignees is the single assignee-resolution pipeline for task
// nodes: load configs, resolve to concrete users, deduplicate, apply
// delegation, then deduplicate again. The second pass matters because two
// distinct assignees may delegate to the same person; without it the
// delegatee would receive duplicate tasks and double-count in pass-rule
// totals. When delegations collide, the first delegation chain (by assignee
// order) wins and keeps its delegator attribution.
func resolveNodeAssignees(ctx context.Context, pc *ProcessContext) ([]approval.ResolvedAssignee, error) {
	assignees, err := resolveAssignees(ctx, pc)
	if err != nil {
		return nil, err
	}

	assignees = deduplicateAssignees(assignees)

	assignees, err = applyDelegation(ctx, pc.DB, pc.Instance.FlowID, assignees, pc.UserResolver)
	if err != nil {
		return nil, err
	}

	return deduplicateAssignees(assignees), nil
}

// resolveAssignees loads the node's assignee configs and resolves them to concrete users.
func resolveAssignees(ctx context.Context, pc *ProcessContext) ([]approval.ResolvedAssignee, error) {
	var assignees []approval.FlowNodeAssignee

	if err := pc.DB.NewSelect().
		Model(&assignees).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("node_id", pc.Node.ID)
		}).
		OrderBy("sort_order").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("load node assignees: %w", err)
	}

	return pc.Registry.Assignees().ResolveAll(ctx, assignees, pc.NodeResolveContext())
}

// deduplicateAssignees removes duplicate assignees based on the receiving
// user's ID.
func deduplicateAssignees(assignees []approval.ResolvedAssignee) []approval.ResolvedAssignee {
	seen := collections.NewHashSet[string]()
	result := make([]approval.ResolvedAssignee, 0, len(assignees))

	for _, a := range assignees {
		if a.User.ID == "" {
			continue
		}

		if !seen.Add(a.User.ID) {
			continue
		}

		result = append(result, a)
	}

	return result
}

// findPreviousApprovalApprovers returns the set of assignee IDs who approved in the
// most recent approval node before the current one within the same instance.
//
// Assumption: the instance advances through a single active path in an acyclic flow.
// Under that model, the latest created task among prior approval nodes belongs to the
// immediately preceding approval node. Returns an empty set if no previous approval
// node exists.
func findPreviousApprovalApprovers(ctx context.Context, db orm.DB, instance *approval.Instance, currentNodeID string) (collections.Set[string], error) {
	// Step 1: Find all approval node IDs in this flow version (excluding current node)
	var approvalNodeIDs []string

	if err := db.NewSelect().
		Model((*approval.FlowNode)(nil)).
		Select("id").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_version_id", instance.FlowVersionID).
				Equals("kind", string(approval.NodeApproval)).
				NotEquals("id", currentNodeID)
		}).
		Scan(ctx, &approvalNodeIDs); err != nil {
		return nil, fmt.Errorf("find approval nodes: %w", err)
	}

	if len(approvalNodeIDs) == 0 {
		return collections.NewHashSet[string](), nil
	}

	// Step 2: Under the single-active-path assumption above, the latest task
	// created for any prior approval node identifies the immediately preceding
	// approval node in this instance.
	var prevNodeID string

	if err := db.NewSelect().
		Model((*approval.Task)(nil)).
		Select("node_id").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instance.ID).
				In("node_id", approvalNodeIDs)
		}).
		OrderByDesc("created_at").
		Limit(1).
		Scan(ctx, &prevNodeID); err != nil {
		if result.IsRecordNotFound(err) {
			// No previous approval node has been processed yet
			return collections.NewHashSet[string](), nil
		}

		return nil, fmt.Errorf("find previous approval node: %w", err)
	}

	// Step 3: Get assignee IDs of approved tasks in the previous approval node
	var approvedAssigneeIDs []string

	if err := db.NewSelect().
		Model((*approval.Task)(nil)).
		Select("assignee_id").
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instance.ID).
				Equals("node_id", prevNodeID).
				Equals("status", string(approval.TaskApproved))
		}).
		Scan(ctx, &approvedAssigneeIDs); err != nil {
		return nil, fmt.Errorf("find approved assignees in previous node: %w", err)
	}

	return collections.NewHashSetFrom(approvedAssigneeIDs...), nil
}

// applyDelegation resolves delegation chains for each assignee, replacing
// delegators with delegatees. A delegated entry keeps the original assignee as
// Delegator so the task can attribute both parties.
func applyDelegation(ctx context.Context, db orm.DB, flowID string, assignees []approval.ResolvedAssignee, userResolver approval.UserInfoResolver) ([]approval.ResolvedAssignee, error) {
	categoryID, err := loadFlowCategoryID(ctx, db, flowID)
	if err != nil {
		return nil, err
	}

	delegateeByIndex := make([]string, len(assignees))

	var delegateeIDs []string

	for i, assignee := range assignees {
		delegateeID, _, err := resolveDelegationChain(ctx, db, assignee.User.ID, flowID, categoryID)
		if err != nil {
			return nil, err
		}

		delegateeByIndex[i] = delegateeID

		if delegateeID != assignee.User.ID {
			delegateeIDs = append(delegateeIDs, delegateeID)
		}
	}

	delegateeInfos, err := shared.ResolveUserInfoMap(ctx, userResolver, delegateeIDs)
	if err != nil {
		return nil, fmt.Errorf("resolve delegatee info: %w", err)
	}

	result := make([]approval.ResolvedAssignee, len(assignees))

	for i, assignee := range assignees {
		delegateeID := delegateeByIndex[i]
		if delegateeID != assignee.User.ID {
			delegatee := delegateeInfos[delegateeID]
			delegatee.ID = delegateeID
			delegator := assignee.User
			result[i] = approval.ResolvedAssignee{User: delegatee, Delegator: &delegator}
		} else {
			result[i] = assignee
		}
	}

	return result, nil
}

// buildTask creates a base task from the process context with the given
// assignee, snapshotting both parties' identity and department, bound to the
// node visit that created it — every task belongs to exactly one visit.
func buildTask(pc *ProcessContext, assignee approval.ResolvedAssignee, deadline *timex.DateTime) *approval.Task {
	task := &approval.Task{
		TenantID:               pc.Instance.TenantID,
		InstanceID:             pc.Instance.ID,
		NodeID:                 pc.Node.ID,
		VisitID:                pc.Visit.ID,
		AssigneeID:             assignee.User.ID,
		AssigneeName:           assignee.User.Name,
		AssigneeDepartmentID:   assignee.User.DepartmentID,
		AssigneeDepartmentName: assignee.User.DepartmentName,
		SortOrder:              0,
		Status:                 approval.TaskPending,
		Deadline:               deadline,
	}

	if assignee.Delegator != nil {
		task.DelegatorID = new(assignee.Delegator.ID)
		task.DelegatorName = new(assignee.Delegator.Name)
		task.DelegatorDepartmentID = assignee.Delegator.DepartmentID
		task.DelegatorDepartmentName = assignee.Delegator.DepartmentName
	}

	return task
}

// taskInsertedEvents returns the events describing a just-inserted task row:
// always the TaskCreatedEvent reporting the row's physical creation, followed
// by a TaskActivatedEvent when the task is actionable straight away. A
// sequential node's queued tasks are inserted as TaskWaiting and get their
// activation event later, when the queue reaches them.
func taskInsertedEvents(pc *ProcessContext, task *approval.Task) []approval.DomainEvent {
	events := []approval.DomainEvent{approval.NewTaskCreatedEvent(pc.Instance, task, pc.Node)}

	if task.Status == approval.TaskPending {
		events = append(events, approval.NewTaskActivatedEvent(
			pc.Instance, task, pc.Node, approval.TaskActivationAssigned,
		))
	}

	return events
}

// suppressActivationsForClearedTasks drops activation events for tasks that no
// longer await their assignee, judged by the tasks' final in-memory state. It
// exists for the consecutive-approver cascade, where promoting the next queued
// task and clearing it can both happen while the loop runs: the promotion is
// real, but nobody was ever asked to act on it.
func suppressActivationsForClearedTasks(events []approval.DomainEvent, tasks []approval.Task) []approval.DomainEvent {
	if len(events) == 0 {
		return events
	}

	cleared := collections.NewHashSet[string]()

	for i := range tasks {
		if tasks[i].Status != approval.TaskPending {
			cleared.Add(tasks[i].ID)
		}
	}

	if cleared.IsEmpty() {
		return events
	}

	kept := make([]approval.DomainEvent, 0, len(events))

	for _, evt := range events {
		if a, ok := evt.(*approval.TaskActivatedEvent); ok && cleared.Contains(a.TaskID) {
			continue
		}

		kept = append(kept, evt)
	}

	return kept
}

// taskCreatedEventsFor returns the events for a batch of just-inserted tasks,
// preserving input order.
func taskCreatedEventsFor(pc *ProcessContext, tasks []*approval.Task) []approval.DomainEvent {
	events := make([]approval.DomainEvent, 0, len(tasks)*2)
	for _, t := range tasks {
		events = append(events, taskInsertedEvents(pc, t)...)
	}

	return events
}

// createTasksForUsers creates pending tasks for a list of user IDs with sortOrder=0.
// User names are resolved via pc.UserResolver. Returns a Wait result whose
// Events field carries one TaskCreatedEvent per inserted task, or
// approval.ErrNoAssignee if the list is empty.
func createTasksForUsers(ctx context.Context, pc *ProcessContext, userIDs []string) (*ProcessResult, error) {
	normalizedIDs := shared.NormalizeUniqueIDs(userIDs)
	if len(normalizedIDs) == 0 {
		return nil, approval.ErrNoAssignee
	}

	infos, err := shared.ResolveUserInfoMap(ctx, pc.UserResolver, normalizedIDs)
	if err != nil {
		return nil, fmt.Errorf("resolve user info: %w", err)
	}

	deadline := computeDeadline(pc.Node)

	tasks := make([]*approval.Task, len(normalizedIDs))

	for i, uid := range normalizedIDs {
		info := infos[uid]
		info.ID = uid
		tasks[i] = buildTask(pc, approval.ResolvedAssignee{User: info}, deadline)
	}

	if _, err := pc.DB.NewInsert().Model(&tasks).Exec(ctx); err != nil {
		return nil, fmt.Errorf("create tasks: %w", err)
	}

	return &ProcessResult{Action: NodeActionWait, Events: taskCreatedEventsFor(pc, tasks)}, nil
}

// handleEmptyAssignee handles the case when no assignees are resolved.
// The behavior depends on the node's EmptyAssigneeAction configuration.
func handleEmptyAssignee(ctx context.Context, pc *ProcessContext, assigneeService approval.AssigneeService) (*ProcessResult, error) {
	switch pc.Node.EmptyAssigneeAction {
	case approval.EmptyAssigneeAutoPass:
		return nodeAutoPassResult(ctx, pc, autoPassReasonEmptyAssignee), nil

	case approval.EmptyAssigneeTransferAdmin:
		return createTasksForUsers(ctx, pc, pc.Node.AdminUserIDs)

	case approval.EmptyAssigneeTransferApplicant:
		return createTasksForUsers(ctx, pc, []string{pc.Instance.ApplicantID})

	case approval.EmptyAssigneeTransferSpecified:
		return createTasksForUsers(ctx, pc, pc.Node.FallbackUserIDs)

	case approval.EmptyAssigneeTransferSuperior:
		superiorInfo, err := getSuperior(ctx, assigneeService, pc.Instance.ApplicantID)
		if err != nil {
			return nil, err
		}

		if superiorInfo == nil || superiorInfo.ID == "" {
			return nil, approval.ErrNoAssignee
		}

		return createTasksForUsers(ctx, pc, []string{superiorInfo.ID})

	default:
		return nil, approval.ErrNoAssignee
	}
}

// createTasksWithDelegation creates tasks for resolved assignees, snapshotting
// the delegator (id, name, department) when applicable. Returns one
// TaskCreatedEvent per inserted task in input order so the caller can attach
// them to ProcessResult.Events alongside any node-level events.
func createTasksWithDelegation(ctx context.Context, pc *ProcessContext, assignees []approval.ResolvedAssignee) ([]approval.DomainEvent, error) {
	deadline := computeDeadline(pc.Node)

	tasks := make([]*approval.Task, len(assignees))
	for i, assignee := range assignees {
		tasks[i] = buildTask(pc, assignee, deadline)
	}

	if _, err := pc.DB.NewInsert().Model(&tasks).Exec(ctx); err != nil {
		return nil, fmt.Errorf("create tasks: %w", err)
	}

	return taskCreatedEventsFor(pc, tasks), nil
}

// getSuperior retrieves the superior user info. Returns ErrAssigneeServiceNotConfigured if assigneeService is nil.
func getSuperior(ctx context.Context, assigneeService approval.AssigneeService, userID string) (*approval.UserInfo, error) {
	if assigneeService == nil {
		return nil, ErrAssigneeServiceNotConfigured
	}

	return assigneeService.GetSuperior(ctx, userID)
}

// computeDeadline returns a deadline based on the node's TimeoutHours configuration.
// Returns nil if TimeoutHours is not set.
func computeDeadline(node *approval.FlowNode) *timex.DateTime {
	if node == nil {
		return nil
	}

	return shared.ComputeTaskDeadline(node.TimeoutHours)
}

// loadFlowCategoryID loads the category ID for a flow.
func loadFlowCategoryID(ctx context.Context, db orm.DB, flowID string) (string, error) {
	var flow approval.Flow

	flow.ID = flowID

	if err := db.NewSelect().
		Model(&flow).
		Select("category_id").
		WherePK().
		Scan(ctx); err != nil {
		return "", fmt.Errorf("load flow category: %w", err)
	}

	return flow.CategoryID, nil
}

// resolveDelegationChain resolves a delegation chain A->B->C with cycle detection.
// Matching priority: flow-specific > category-specific > global (by created_at DESC).
func resolveDelegationChain(ctx context.Context, db orm.DB, userID, flowID, flowCategoryID string) (delegateeID, originalUserID string, err error) {
	const maxDepth = 10

	var (
		currentID  = userID
		originalID = userID
		visited    = collections.NewHashSetFrom(userID)
		now        = timex.Now()
	)

	for range maxDepth {
		var delegations []approval.Delegation

		if err := db.NewSelect().
			Model(&delegations).
			Select("delegatee_id", "starts_at", "ends_at", "flow_category_id", "flow_id").
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("delegator_id", currentID).
					IsTrue("is_active")
			}).
			OrderByDesc("created_at").
			Limit(100).
			Scan(ctx); err != nil {
			return "", "", fmt.Errorf("load delegations for user %s: %w", currentID, err)
		}

		if len(delegations) == 0 {
			break
		}

		matched := matchDelegation(delegations, now, flowID, flowCategoryID)
		if matched == nil {
			break
		}

		nextID := matched.DelegateeID
		if visited.Contains(nextID) {
			break
		}

		visited.Add(nextID)
		currentID = nextID
	}

	if currentID == originalID {
		return currentID, "", nil
	}

	return currentID, originalID, nil
}

// matchDelegation finds the best matching delegation with priority:
// flow-specific > category-specific > global.
func matchDelegation(delegations []approval.Delegation, now timex.DateTime, flowID, flowCategoryID string) *approval.Delegation {
	var categoryMatch, globalMatch *approval.Delegation

	for i := range delegations {
		d := &delegations[i]

		if !d.StartsAt.IsZero() && now.Before(d.StartsAt) {
			continue
		}

		if !d.EndsAt.IsZero() && now.After(d.EndsAt) {
			continue
		}

		if d.FlowCategoryID != nil && *d.FlowCategoryID != flowCategoryID {
			continue
		}

		if d.FlowID != nil && *d.FlowID != flowID {
			continue
		}

		if d.FlowID != nil {
			return d
		}

		if d.FlowCategoryID != nil && categoryMatch == nil {
			categoryMatch = d
		}

		if d.FlowCategoryID == nil && globalMatch == nil {
			globalMatch = d
		}
	}

	if categoryMatch != nil {
		return categoryMatch
	}

	return globalMatch
}
