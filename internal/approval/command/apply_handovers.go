package command

import (
	"context"
	"fmt"
	"slices"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// ApplyHandoversCmd runs inside the normal transaction and event/log collectors.
// ScanInstanceID restricts one internal polling dispatch to one instance.
type ApplyHandoversCmd struct {
	cqrs.BaseCommand
	approval.ApplyHandoversInput

	ScanInstanceID string
}

type ApplyHandoversHandler struct {
	db        orm.DB
	handovers approval.HandoverResolver
	users     approval.UserInfoResolver
	nodes     *service.NodeService
	bus       event.Bus
}

func NewApplyHandoversHandler(db orm.DB, handovers approval.HandoverResolver, users approval.UserInfoResolver, nodes *service.NodeService, bus event.Bus) *ApplyHandoversHandler {
	return &ApplyHandoversHandler{db: db, handovers: handovers, users: users, nodes: nodes, bus: bus}
}

var activeHandoverTasks = []approval.TaskStatus{approval.TaskPending, approval.TaskWaiting}

func handoverTasks(q orm.SelectQuery, ids []string) orm.SelectQuery {
	return q.Model((*approval.Task)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.In("status", activeHandoverTasks)

		if len(ids) > 0 {
			cb.Group(func(cb orm.ConditionBuilder) { cb.In("assignee_id", ids).OrIn("delegator_id", ids) })
		}
	})
}

func handoverReceipts(q orm.SelectQuery, ids []string) orm.SelectQuery {
	return q.Model((*approval.CCRecord)(nil)).
		Join((*approval.NodeVisit)(nil), func(cb orm.ConditionBuilder) { cb.EqualsColumn("anv.id", "acr.visit_id") }).
		Join((*approval.FlowNode)(nil), func(cb orm.ConditionBuilder) { cb.EqualsColumn("afn.id", "anv.node_id") }).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("anv.status", approval.NodeVisitActive).Equals("afn.kind", approval.NodeCC).IsTrue("afn.is_read_confirm_required").IsNull("acr.read_at")

			if len(ids) > 0 {
				cb.In("acr.cc_user_id", ids)
			}
		})
}

// HandoverCandidates selects instances with eligible work. An empty user set is
// reserved for the scanner and selects active work only.
func HandoverCandidates(db orm.DB, ids []string) orm.SelectQuery {
	return db.NewSelect().Model((*approval.Instance)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.Group(func(cb orm.ConditionBuilder) {
			cb.InSubQuery("id", func(q orm.SelectQuery) { handoverTasks(q, ids).Select("instance_id") }).
				OrInSubQuery("id", func(q orm.SelectQuery) { handoverReceipts(q, ids).Select("acr.instance_id") })

			if len(ids) > 0 {
				cb.OrGroup(func(cb orm.ConditionBuilder) {
					cb.In("applicant_id", ids).In("status", []approval.InstanceStatus{approval.InstanceReturned, approval.InstanceWithdrawn})
				})
			}
		})
	}).OrderBy("id")
}

func (h *ApplyHandoversHandler) Handle(ctx context.Context, cmd ApplyHandoversCmd) (*approval.HandoverReport, error) {
	tenant, err := cmd.Caller.TenantScopeFilter("")
	if err != nil {
		return nil, err
	}

	ids := shared.NormalizeUniqueIDs(cmd.UserIDs)
	if len(ids) == 0 && cmd.ScanInstanceID == "" {
		return nil, approval.ErrNoUsersSpecified
	}

	db := contextx.DB(ctx, h.db)
	report := &approval.HandoverReport{Transferred: []approval.HandoverItem{}, Unassigned: []approval.HandoverItem{}, AwaitingApplicant: []string{}}

	if cmd.ScanInstanceID != "" {
		ctx = orm.WithQuietSQLLog(ctx)
	}

	q := HandoverCandidates(db, ids)
	if tenant != nil {
		q.Where(func(cb orm.ConditionBuilder) { cb.Equals("tenant_id", *tenant) })
	}

	if cmd.ScanInstanceID != "" {
		q.Where(func(cb orm.ConditionBuilder) { cb.PKEquals(cmd.ScanInstanceID) })
	}

	var instances []approval.Instance
	if err := q.ForUpdate().Scan(ctx, &instances); err != nil {
		return nil, fmt.Errorf("lock handover instances: %w", err)
	}

	batch := handoverBatch{db: db, operator: cmd.Operator, report: report}
	for i := range instances {
		if err := h.applyInstance(ctx, &batch, &instances[i], ids); err != nil {
			return nil, fmt.Errorf("apply handovers to instance %s: %w", instances[i].ID, err)
		}
	}

	return report, nil
}

type handoverBatch struct {
	db       orm.DB
	operator approval.UserInfo
	report   *approval.HandoverReport
}

// applyInstance runs after locking the instance. A running instance has one
// active node. Tasks are grouped by their own node to retain their resolver scope.
func (h *ApplyHandoversHandler) applyInstance(ctx context.Context, batch *handoverBatch, inst *approval.Instance, ids []string) error {
	db := batch.db

	var tasks []approval.Task
	if err := handoverTasks(db.NewSelect(), ids).Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).OrderBy("sort_order", "id").ForUpdate().Scan(ctx, &tasks); err != nil {
		return err
	}

	var receipts []approval.CCRecord
	if err := handoverReceipts(
		db.NewSelect(),
		ids,
	).SelectModelColumns().
		Where(func(cb orm.ConditionBuilder) { cb.Equals("acr.instance_id", inst.ID) }).
		OrderBy("acr.id").ForUpdate((*approval.CCRecord)(nil)).
		Scan(ctx, &receipts); err != nil {
		return err
	}

	byNode := make(map[string]*handoverWork)
	nodeIDs := shared.NewOrderedUnique[string](len(tasks) + len(receipts))

	group := func(id string) *handoverWork {
		if nodeIDs.Add(id) {
			byNode[id] = &handoverWork{}
		}

		return byNode[id]
	}
	for i := range tasks {
		g := group(tasks[i].NodeID)
		g.tasks = append(g.tasks, &tasks[i])
	}

	for i := range receipts {
		g := group(*receipts[i].NodeID)
		g.receipts = append(g.receipts, &receipts[i])
	}

	if (inst.Status == approval.InstanceReturned || inst.Status == approval.InstanceWithdrawn) && slices.Contains(ids, inst.ApplicantID) {
		var nodeID string
		if inst.CurrentNodeID != nil {
			nodeID = *inst.CurrentNodeID
		}

		group(nodeID).applicant = true
	}

	for _, nodeID := range nodeIDs.ToSlice() {
		var node *approval.FlowNode
		if nodeID != "" {
			node = &approval.FlowNode{ID: nodeID}
			if err := db.NewSelect().Model(node).WherePK().Scan(ctx); err != nil {
				return err
			}
		}

		g := byNode[nodeID]
		r := engine.NewActorResolution(inst, node, h.handovers)

		changes, err := r.Handovers(ctx, g.userIDs(inst, ids))
		if err != nil {
			return err
		}

		if g.applicant {
			if _, ok := changes[inst.ApplicantID]; ok {
				batch.report.AwaitingApplicant = append(batch.report.AwaitingApplicant, inst.ID)
			}
		}

		for _, task := range g.tasks {
			predecessor := task.Assignee()
			if delegator := task.Delegator(); delegator != nil {
				if _, ok := changes[delegator.ID]; ok {
					predecessor = *delegator
				}
			}

			if change, ok := changes[predecessor.ID]; ok {
				if err := h.moveTask(ctx, batch, inst, node, task, predecessor, change, r); err != nil {
					return err
				}
			}
		}

		for _, receipt := range g.receipts {
			if change, ok := changes[receipt.CCUserID]; ok {
				if err := h.moveReceipt(ctx, batch, inst, node, receipt, change); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

type handoverWork struct {
	tasks     []*approval.Task
	receipts  []*approval.CCRecord
	applicant bool
}

func (g *handoverWork) userIDs(inst *approval.Instance, filter []string) []string {
	users := shared.NewOrderedUnique[string](len(g.tasks)*2 + len(g.receipts) + 1)

	add := func(id string) {
		if len(filter) == 0 || slices.Contains(filter, id) {
			users.Add(id)
		}
	}
	for _, t := range g.tasks {
		add(t.AssigneeID)

		if t.DelegatorID != nil {
			add(*t.DelegatorID)
		}
	}

	for _, r := range g.receipts {
		add(r.CCUserID)
	}

	if g.applicant {
		add(inst.ApplicantID)
	}

	return users.ToSlice()
}

func (h *ApplyHandoversHandler) moveTask(
	ctx context.Context,
	batch *handoverBatch,
	inst *approval.Instance,
	node *approval.FlowNode,
	task *approval.Task,
	predecessor approval.UserInfo,
	change approval.Handover,
	r *engine.ActorResolution,
) error {
	db, report := batch.db, batch.report
	// A previous removal in this batch may have promoted or canceled this seat.
	if err := db.NewSelect().Model(task).WherePK().ForUpdate().Scan(ctx); err != nil {
		return err
	}

	if !slices.Contains(activeHandoverTasks, task.Status) || (task.AssigneeID != predecessor.ID && (task.DelegatorID == nil || *task.DelegatorID != predecessor.ID)) {
		return nil
	}

	item := approval.HandoverItem{InstanceID: inst.ID, TaskID: &task.ID, Predecessor: predecessor, Successor: change.Successor}
	if change.Successor == nil {
		report.Unassigned = append(report.Unassigned, item)

		return nil
	}

	if change.Successor.ID == predecessor.ID {
		return nil
	}

	ctx = orm.WithoutQuietSQLLog(ctx)

	actors, err := r.Actors(ctx, db, []approval.ResolvedAssignee{{User: predecessor}}, h.users)
	if err != nil {
		return err
	}

	actor := actors[0]
	// A successor's delegate may already be the actor on this very task.
	// The seat still changed hands, so update the delegator and predecessor.

	duplicate, err := db.NewSelect().Model((*approval.Task)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("visit_id", task.VisitID).Equals("assignee_id", actor.User.ID).NotEquals("id", task.ID).In("status", activeHandoverTasks)
	}).ForUpdate().Exists(ctx)
	if err != nil {
		return err
	}

	logHandover(ctx, item, node.ID, change.Reason, batch.operator)

	if duplicate {
		if err := h.nodes.RemoveTask(ctx, db, inst, node, task, &actor); err != nil {
			return err
		}
	} else {
		if err := service.ReplaceTaskActor(ctx, db, task, actor); err != nil {
			return err
		}

		events := []approval.DomainEvent{
			approval.NewTaskHandedOverEvent(inst, task, node, predecessor, *change.Successor, change.Reason),
		}
		if task.Status == approval.TaskPending {
			events = append(events, approval.NewTaskActivatedEvent(inst, task, node, approval.TaskActivationHandedOver))
		}

		if err := behavior.EmitEvents(ctx, h.bus, db, events...); err != nil {
			return err
		}
	}

	report.Transferred = append(report.Transferred, item)

	return nil
}

func (h *ApplyHandoversHandler) moveReceipt(
	ctx context.Context,
	batch *handoverBatch,
	inst *approval.Instance,
	node *approval.FlowNode,
	receipt *approval.CCRecord,
	change approval.Handover,
) error {
	db, report := batch.db, batch.report
	item := approval.HandoverItem{InstanceID: inst.ID, CCRecordID: &receipt.ID, Predecessor: receipt.Recipient().User, Successor: change.Successor}

	to := change.Successor
	if to == nil {
		report.Unassigned = append(report.Unassigned, item)

		return nil
	}

	if to.ID == receipt.CCUserID {
		return nil
	}

	ctx = orm.WithoutQuietSQLLog(ctx)

	duplicate, err := db.NewSelect().
		Model((*approval.CCRecord)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("visit_id", *receipt.VisitID).Equals("cc_user_id", to.ID) }).
		ForUpdate().
		Exists(ctx)
	if err != nil {
		return err
	}

	logHandover(ctx, item, node.ID, change.Reason, batch.operator)

	if duplicate {
		if _, err := db.NewDelete().Model(receipt).WherePK().Exec(ctx); err != nil {
			return err
		}

		if err := h.nodes.AdvanceCCNodeIfAllRead(ctx, db, inst.ID, []approval.CCRecord{*receipt}); err != nil {
			return err
		}
	} else {
		if _, err := db.NewUpdate().
			Model(receipt).
			Set("cc_user_id", to.ID).
			Set("cc_user_name", to.Name).
			Set("cc_user_department_id", to.DepartmentID).
			Set("cc_user_department_name", to.DepartmentName).
			WherePK().
			Exec(ctx); err != nil {
			return err
		}
	}

	report.Transferred = append(report.Transferred, item)

	return nil
}

func logHandover(ctx context.Context, item approval.HandoverItem, nodeID, reason string, op approval.UserInfo) {
	entry := op.NewActionLog(item.InstanceID, approval.ActionHandover)
	entry.TaskID = item.TaskID
	entry.NodeID = &nodeID
	entry.TransferToID = new(item.Successor.ID)
	entry.TransferToName = new(item.Successor.Name)
	entry.TransferToDepartmentID = item.Successor.DepartmentID
	entry.TransferToDepartmentName = item.Successor.DepartmentName

	entry.Opinion = new(reason)
	if item.CCRecordID != nil {
		entry.CCUsers = []approval.UserInfo{item.Predecessor}
	}

	behavior.ActionLogCollectorFromContext(ctx).Add(entry)
}
