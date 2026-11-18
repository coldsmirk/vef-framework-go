package command

import (
	"context"
	"database/sql"
	"fmt"
	"slices"

	"github.com/samber/lo"

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

// handoverCandidates selects instances with eligible work. An empty user set is
// reserved for the scanner and selects active work only.
func handoverCandidates(db orm.DB, ids []string) orm.SelectQuery {
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

	q := handoverCandidates(db, ids)
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
		candidates, err := loadHandoverWork(ctx, db, instances[i:i+1], ids, true)
		if err != nil {
			return nil, err
		}

		if err := h.applyInstance(ctx, &batch, &candidates[0], ids); err != nil {
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

// HandoverCandidate groups one instance's eligible work by resolver scope.
type HandoverCandidate struct {
	Instance *approval.Instance
	groups   []*handoverWork
}

type handoverWork struct {
	node      *approval.FlowNode
	tasks     []*approval.Task
	receipts  []*approval.CCRecord
	applicant bool
}

// FindHandoverScanPage reads a keyset page of candidates without opening a
// transaction. Work and node snapshots are loaded in batches for the whole page.
func FindHandoverScanPage(ctx context.Context, db orm.DB, cursor string, limit int) ([]HandoverCandidate, error) {
	var instances []approval.Instance
	if err := handoverCandidates(db, nil).Where(func(cb orm.ConditionBuilder) { cb.GreaterThan("id", cursor) }).Limit(limit).Scan(ctx, &instances); err != nil {
		return nil, err
	}

	return loadHandoverWork(ctx, db, instances, nil, false)
}

func loadHandoverWork(ctx context.Context, db orm.DB, instances []approval.Instance, ids []string, lock bool) ([]HandoverCandidate, error) {
	if len(instances) == 0 {
		return nil, nil
	}

	candidates := make([]HandoverCandidate, len(instances))
	byInstance := make(map[string]*HandoverCandidate, len(instances))

	instanceIDs := make([]string, len(instances))
	for i := range instances {
		candidates[i].Instance = &instances[i]
		instanceIDs[i] = instances[i].ID
		byInstance[instanceIDs[i]] = &candidates[i]
	}

	tq := handoverTasks(db.NewSelect(), ids).Where(func(cb orm.ConditionBuilder) { cb.In("instance_id", instanceIDs) }).OrderBy("sort_order", "id")

	rq := handoverReceipts(db.NewSelect(), ids).Where(func(cb orm.ConditionBuilder) { cb.In("acr.instance_id", instanceIDs) }).OrderBy("acr.id")
	if lock {
		tq.ForUpdate()
		rq.SelectModelColumns().ForUpdate((*approval.CCRecord)(nil))
	} else {
		// Discovery needs only the resolver scope and the users holding work.
		tq.Select("instance_id", "node_id", "assignee_id", "delegator_id")
		rq.Select("acr.instance_id", "acr.node_id", "acr.cc_user_id")
	}

	var tasks []approval.Task
	if err := tq.Scan(ctx, &tasks); err != nil {
		return nil, err
	}

	var receipts []approval.CCRecord
	if err := rq.Scan(ctx, &receipts); err != nil {
		return nil, err
	}

	type scope struct{ instanceID, nodeID string }

	groups := make(map[scope]*handoverWork)
	nodeIDs := shared.NewOrderedUnique[string](len(tasks) + len(receipts))

	group := func(instanceID, nodeID string) *handoverWork {
		key := scope{instanceID, nodeID}
		if g := groups[key]; g != nil {
			return g
		}

		g := &handoverWork{}
		groups[key] = g
		c := byInstance[instanceID]
		c.groups = append(c.groups, g)

		if nodeID != "" {
			nodeIDs.Add(nodeID)
		}

		return g
	}
	for i := range tasks {
		g := group(tasks[i].InstanceID, tasks[i].NodeID)
		g.tasks = append(g.tasks, &tasks[i])
	}

	for i := range receipts {
		g := group(receipts[i].InstanceID, *receipts[i].NodeID)
		g.receipts = append(g.receipts, &receipts[i])
	}

	for _, inst := range instances {
		if (inst.Status == approval.InstanceReturned || inst.Status == approval.InstanceWithdrawn) && slices.Contains(ids, inst.ApplicantID) {
			group(inst.ID, lo.FromPtr(inst.CurrentNodeID)).applicant = true
		}
	}

	if nodes := nodeIDs.ToSlice(); len(nodes) > 0 {
		var rows []approval.FlowNode
		if err := db.NewSelect().Model(&rows).Where(func(cb orm.ConditionBuilder) { cb.In("id", nodes) }).Scan(ctx); err != nil {
			return nil, err
		}

		if len(rows) != len(nodes) {
			return nil, fmt.Errorf("load handover nodes: %w", sql.ErrNoRows)
		}

		byNode := make(map[string]*approval.FlowNode, len(rows))
		for i := range rows {
			byNode[rows[i].ID] = &rows[i]
		}

		for key, g := range groups {
			g.node = byNode[key.nodeID]
		}
	}

	return candidates, nil
}

// NeedsMigration checks the read-only snapshot. The command resolves fresh
// handovers after taking locks before it makes any change.
func (c *HandoverCandidate) NeedsMigration(ctx context.Context, handovers approval.HandoverResolver) (bool, error) {
	for _, g := range c.groups {
		r := engine.NewActorResolution(c.Instance, g.node, handovers)

		changes, err := r.Handovers(ctx, g.userIDs(c.Instance))
		if err != nil {
			return false, err
		}

		for _, task := range g.tasks {
			if move := planTaskHandover(task, changes, nil); move != nil && move.actionable() {
				return true, nil
			}
		}

		for _, receipt := range g.receipts {
			if successor := changes[receipt.CCUserID].Successor; successor != nil && successor.ID != receipt.CCUserID {
				return true, nil
			}
		}
	}

	return false, nil
}

func (h *ApplyHandoversHandler) applyInstance(ctx context.Context, batch *handoverBatch, candidate *HandoverCandidate, ids []string) error {
	inst := candidate.Instance
	for _, g := range candidate.groups {
		r := engine.NewActorResolution(inst, g.node, h.handovers)

		changes, err := r.Handovers(ctx, g.userIDs(inst))
		if err != nil {
			return err
		}

		if g.applicant {
			if _, ok := changes[inst.ApplicantID]; ok {
				batch.report.AwaitingApplicant = append(batch.report.AwaitingApplicant, inst.ID)
			}
		}

		for _, task := range g.tasks {
			if move := planTaskHandover(task, changes, ids); move != nil {
				if err := h.moveTask(ctx, batch, inst, g.node, task, move, r); err != nil {
					return err
				}
			}
		}

		for _, receipt := range g.receipts {
			if change, ok := changes[receipt.CCUserID]; ok {
				if err := h.moveReceipt(ctx, batch, inst, g.node, receipt, change); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (g *handoverWork) userIDs(inst *approval.Instance) []string {
	users := shared.NewOrderedUnique[string](len(g.tasks)*2 + len(g.receipts) + 1)
	for _, t := range g.tasks {
		users.Add(t.AssigneeID)

		if t.DelegatorID != nil {
			users.Add(*t.DelegatorID)
		}
	}

	for _, r := range g.receipts {
		users.Add(r.CCUserID)
	}

	if g.applicant {
		users.Add(inst.ApplicantID)
	}

	return users.ToSlice()
}

type taskHandover struct {
	seat, from approval.UserInfo
	change     approval.Handover
}

// A delegate's departure unwinds their personal delegation even when they
// have no successor. A seat's own departure requires an HR successor.
func (m *taskHandover) actionable() bool {
	return m.from.ID != m.seat.ID || m.change.Successor != nil
}

func planTaskHandover(task *approval.Task, changes map[string]approval.Handover, ids []string) *taskHandover {
	assignee := task.Assignee()

	seat := assignee
	if d := task.Delegator(); d != nil {
		seat = *d
	}

	seatChange, seatDeparted := changes[seat.ID]
	actorChange, actorDeparted := changes[assignee.ID]

	selected := len(ids) == 0 || seatDeparted && slices.Contains(ids, seat.ID) || actorDeparted && slices.Contains(ids, assignee.ID)
	if !selected {
		return nil
	}

	if seatDeparted && (seatChange.Successor == nil || seatChange.Successor.ID != seat.ID) {
		return &taskHandover{seat: seat, from: seat, change: seatChange}
	}

	if assignee.ID != seat.ID && actorDeparted {
		return &taskHandover{seat: seat, from: assignee, change: actorChange}
	}

	return nil
}

func (h *ApplyHandoversHandler) moveTask(
	ctx context.Context,
	batch *handoverBatch,
	inst *approval.Instance,
	node *approval.FlowNode,
	snapshot *approval.Task,
	move *taskHandover,
	r *engine.ActorResolution,
) error {
	db, report := batch.db, batch.report
	// Queue advancement or a re-entrant command can change a later task.
	// Only apply the plan if the active seat still has the owners we resolved.
	task := &approval.Task{ID: snapshot.ID}
	if err := db.NewSelect().Model(task).WherePK().ForUpdate().Scan(ctx); err != nil {
		return err
	}

	if !slices.Contains(activeHandoverTasks, task.Status) || task.AssigneeID != snapshot.AssigneeID || lo.FromPtr(task.DelegatorID) != lo.FromPtr(snapshot.DelegatorID) {
		return nil
	}

	item := approval.HandoverItem{InstanceID: inst.ID, TaskID: &task.ID, Predecessor: move.from, Successor: move.change.Successor}
	if !move.actionable() {
		report.Unassigned = append(report.Unassigned, item)

		return nil
	}

	ctx = orm.WithoutQuietSQLLog(ctx)
	seat := approval.ResolvedAssignee{User: move.seat, Predecessor: task.Predecessor(), HandoverReason: lo.FromPtr(task.HandoverReason)}

	actors, err := r.Actors(ctx, db, []approval.ResolvedAssignee{seat}, h.users)
	if err != nil {
		return err
	}

	actor := actors[0]
	if actor.User.ID == task.AssigneeID && lo.FromPtr(actor.Delegator).ID == lo.FromPtr(task.DelegatorID) && lo.FromPtr(actor.Predecessor).ID == lo.FromPtr(task.PredecessorID) {
		return nil
	}

	if move.from.ID != move.seat.ID {
		// Work returns to the last active delegate or to the seat holder.
		item.Successor = &actor.User
	}

	duplicate, err := db.NewSelect().Model((*approval.Task)(nil)).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("visit_id", task.VisitID).Equals("assignee_id", actor.User.ID).NotEquals("id", task.ID).In("status", activeHandoverTasks)
	}).ForUpdate().Exists(ctx)
	if err != nil {
		return err
	}

	logHandover(ctx, item, node.ID, move.change.Reason, batch.operator)

	if duplicate {
		if err := h.nodes.RemoveTask(ctx, db, inst, node, task, &actor); err != nil {
			return err
		}
	} else {
		if err := service.ReplaceTaskActor(ctx, db, task, actor); err != nil {
			return err
		}

		events := []approval.DomainEvent{
			approval.NewTaskHandedOverEvent(inst, task, node, item.Predecessor, *item.Successor, move.change.Reason),
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
	behavior.ActionLogCollectorFromContext(ctx).Add(entry)
}
