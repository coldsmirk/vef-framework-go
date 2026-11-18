package command_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/behavior"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/facade"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/timeout"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

type scopedHandovers map[string]approval.Handover

func (m scopedHandovers) ResolveHandovers(ctx context.Context, hc *approval.HandoverContext, ids []string) (map[string]approval.Handover, error) {
	if orm.IsQuietSQLLog(ctx) || hc.Instance == nil || hc.TenantID != hc.Instance.TenantID || hc.Node == nil || hc.Node.Name == "" {
		return nil, fmt.Errorf("invalid lookup scope for %v", ids)
	}

	return m, nil
}

type HandoverTestSuite struct {
	suite.Suite

	ctx      context.Context
	db       orm.DB
	fixture  *FlowFixture
	bus      cqrs.Bus
	events   *eventtest.FakeBus
	svc      approval.Service
	resolver scopedHandovers
}

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite { return &HandoverTestSuite{ctx: env.Ctx, db: env.DB} })
}

func (s *HandoverTestSuite) SetupSuite() {
	s.fixture = deployAndPublishFlow(s.T(), s.ctx, s.db, "handover", approvalFlowDef())
	s.events = eventtest.NewFakeBus()
	s.bus = cqrs.NewBus([]cqrs.Behavior{behavior.NewTransactionBehavior(s.db), behavior.NewActionLogBehavior(s.db), behavior.NewEventPublishBehavior(s.db, s.events)})
	s.svc = facade.NewService(s.bus)
	_, nodes, _ := buildTestServices(buildTestEngine(s.db))
	s.resolver = scopedHandovers{}
	cqrs.Register(s.bus, command.NewApplyHandoversHandler(s.db, s.resolver, nil, nodes, s.events))
}

func (s *HandoverTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
	clear(s.resolver)
	s.events.Reset()
}
func (s *HandoverTestSuite) TearDownSuite() { cleanAllApprovalData(s.ctx, s.db) }
func (s *HandoverTestSuite) apply(ids ...string) *approval.HandoverReport {
	report, err := s.svc.ApplyHandovers(s.ctx, s.db, approval.ApplyHandoversInput{UserIDs: ids, Operator: approval.UserInfo{ID: "operator"}, Caller: approval.CallerContext{TenantID: "default"}})
	s.Require().NoError(err, "Should apply handovers")

	return report
}

func (s *HandoverTestSuite) change(from, to string) {
	h := approval.Handover{Reason: "Changed role"}
	if to != "" {
		h.Successor = &approval.UserInfo{ID: to, Name: "Name " + to, DepartmentID: new("dept")}
	}

	s.resolver[from] = h
}

func (s *HandoverTestSuite) eventTypes() []string {
	var types []string
	for _, e := range s.events.Captured() {
		types = append(types, e.EventType())
	}

	return types
}

func (s *HandoverTestSuite) TestRewriteDelegatedSeat() {
	inst, task := setupRunningInstance(s.T(), s.ctx, s.db, s.fixture, "original-delegate")
	now := timex.Now()
	task.ReadAt = &now
	task.Deadline = new(now.AddHours(1))
	task.IsTimeout = true
	task.DelegatorID = new("old")
	task.DelegatorName = new("Original seat")
	_, err := s.db.NewUpdate().Model(task).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should seed original delegated task")

	delegation := &approval.Delegation{DelegatorID: "new", DelegateeID: "acting", StartsAt: now.AddHours(-1), EndsAt: now.AddHours(1), IsActive: true}
	_, err = s.db.NewInsert().Model(delegation).Exec(s.ctx)

	s.Require().NoError(err, "Should seed successor delegation")
	defer func() {
		_, err := s.db.NewDelete().Model(delegation).WherePK().Exec(s.ctx)
		s.Require().NoError(err, "Should clean delegation")
	}()

	s.change("old", "new")
	draftID := seedTaskDraft(s.T(), s.ctx, s.db, task)
	report := s.apply("old")
	requireNoteStatus(s.T(), s.ctx, s.db, draftID, approval.AnnotationDiscarded)
	s.Require().Len(report.Transferred, 1, "One seat should move")
	item := report.Transferred[0]
	s.Equal(task.ID, *item.TaskID, "Task identity should remain stable")
	s.Equal("new", item.Successor.ID, "Report names successor even when delegated")

	fresh := approval.Task{ID: task.ID}
	s.Require().NoError(s.db.NewSelect().Model(&fresh).WherePK().Scan(s.ctx), "Should read migrated task")
	s.Equal("acting", fresh.AssigneeID, "Should route to successor's delegate")
	s.Equal("new", fresh.Delegator().ID, "Should rewrite delegation attribution")
	s.Equal("Original seat", fresh.Predecessor().Name, "Should preserve predecessor snapshot")
	s.Equal(new("Changed role"), fresh.HandoverReason, "Should record handover reason")
	s.Nil(fresh.ReadAt, "New actor must see an unread task")
	s.True(fresh.IsTimeout, "Timeout marker should survive handover")
	s.WithinDuration(task.Deadline.Unwrap(), fresh.Deadline.Unwrap(), time.Second, "Should preserve deadline")
	s.Equal([]string{approval.EventTypeTaskHandedOver, approval.EventTypeTaskActivated}, s.eventTypes(), "Should announce handover before activation exactly once")

	var logs []approval.ActionLog
	s.Require().NoError(s.db.NewSelect().Model(&logs).Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).Scan(s.ctx), "Should load handover log")
	s.Require().Len(logs, 1, "One migration should produce one log")
	s.Equal(approval.ActionHandover, logs[0].Action, "Should identify handover")
	s.Equal(new("new"), logs[0].TransferToID, "Log names successor")
	s.Equal(new("Changed role"), logs[0].Opinion, "Log records resolver reason")

	tasks := service.NewTaskService()
	_, err = query.NewGetMyInstanceDetailHandler(s.db, nil, nil).Handle(s.ctx, query.GetMyInstanceDetailQuery{InstanceID: inst.ID, Viewer: approval.UserInfo{ID: "old"}})
	s.ErrorIs(err, approval.ErrAccessDenied, "Predecessor must not see instance details")
	_, err = command.NewUrgeTaskHandler(s.db, tasks, nil).Handle(s.ctx, command.UrgeTaskCmd{TaskID: task.ID, UrgerID: "old", Caller: approval.SystemCaller})
	s.ErrorIs(err, approval.ErrAccessDenied, "Predecessor must not urge")
}

func (s *HandoverTestSuite) TestDelegateeDeparture() {
	for _, tc := range []struct {
		name             string
		changes          map[string]string
		chain            []string
		wantActor        string
		wantDelegator    *string
		wantPredecessor  string
		from, to, reason string
		unassigned       bool
	}{
		{"DelegateeWithSuccessor", map[string]string{"B": "C"}, []string{"A", "B"}, "A", nil, "prior", "B", "A", "B left", false},
		{"DelegateeWithoutSuccessor", map[string]string{"B": ""}, []string{"A", "B"}, "A", nil, "prior", "B", "A", "B left", false},
		{"IntermediateDelegate", map[string]string{"B": "C"}, []string{"A", "D", "B"}, "D", new("A"), "prior", "B", "D", "B left", false},
		{"SeatCycle", map[string]string{"A": "X", "X": "A", "B": "C"}, []string{"A", "B"}, "A", nil, "prior", "B", "A", "B left", false},
		{"BothDeparted", map[string]string{"A": "S", "B": "C"}, []string{"A", "B"}, "S", nil, "A", "A", "S", "A left", false},
		{"SeatWithoutSuccessor", map[string]string{"A": "", "B": "C"}, []string{"A", "B"}, "B", new("A"), "prior", "A", "", "", true},
		{"OnlyUnselectedSeatDeparted", map[string]string{"A": "S"}, []string{"A", "B"}, "B", new("A"), "prior", "", "", "", false},
	} {
		s.Run(tc.name, func() {
			defer s.TearDownTest()

			_, task := setupRunningInstance(s.T(), s.ctx, s.db, s.fixture, "B")
			task.DelegatorID, task.DelegatorName = new("A"), new("Seat owner")
			task.PredecessorID, task.PredecessorName = new("prior"), new("Prior owner")
			task.PredecessorDepartmentID, task.HandoverReason = new("dept"), new("Prior transfer")
			_, err := s.db.NewUpdate().Model(task).WherePK().Exec(s.ctx)
			s.Require().NoError(err, "Should seed an already handed-over, delegated seat")

			now := timex.Now()
			for i := 1; i < len(tc.chain); i++ {
				d := &approval.Delegation{DelegatorID: tc.chain[i-1], DelegateeID: tc.chain[i], StartsAt: now.AddHours(-1), EndsAt: now.AddHours(1), IsActive: true}
				_, err := s.db.NewInsert().Model(d).Exec(s.ctx)
				s.Require().NoError(err, "Should seed the delegation chain")
				s.T().Cleanup(func() {
					_, err := s.db.NewDelete().Model(d).WherePK().Exec(s.ctx)
					s.Require().NoError(err, "Should clear this delegation")
				})
			}

			for from, to := range tc.changes {
				change := approval.Handover{Reason: from + " left"}
				if to != "" {
					change.Successor = &approval.UserInfo{ID: to}
				}

				s.resolver[from] = change
			}

			report := s.apply("B")
			task = &approval.Task{ID: task.ID}
			s.Require().NoError(s.db.NewSelect().Model(task).WherePK().Scan(s.ctx), "Should reload the seat")
			s.Equal(tc.wantActor, task.AssigneeID, "Delegation should stop before the departed delegate")
			s.Equal(tc.wantDelegator, task.DelegatorID, "Should keep only the active delegation")
			s.Equal(new(tc.wantPredecessor), task.PredecessorID, "Predecessor should describe the seat")

			if tc.wantPredecessor == "prior" {
				s.Equal(new("Prior owner"), task.PredecessorName, "Should retain the seat predecessor snapshot")
				s.Equal(new("dept"), task.PredecessorDepartmentID, "Should retain the predecessor department")
				s.Equal(new("Prior transfer"), task.HandoverReason, "The delegate's departure should not replace the seat's reason")
			}

			if tc.unassigned {
				s.Require().Len(report.Unassigned, 1, "The departed seat owner has no successor")
				s.Equal(tc.from, report.Unassigned[0].Predecessor.ID, "Should name the unassigned seat owner")
			} else {
				s.Empty(report.Unassigned, "A delegate's departure alone cannot leave the seat unassigned")
			}

			if tc.to == "" {
				s.Empty(report.Transferred, "An unchanged seat should not be reported as moved")
				s.Empty(s.events.Captured(), "An unchanged seat should produce no events")

				return
			}

			s.Require().Len(report.Transferred, 1, "Should report one actual transfer")
			s.Equal(tc.from, report.Transferred[0].Predecessor.ID, "The report should identify whose departure caused the move")
			s.Equal(tc.to, report.Transferred[0].Successor.ID, "The report should identify who took the work")
			s.Equal([]string{approval.EventTypeTaskHandedOver, approval.EventTypeTaskActivated}, s.eventTypes(), "Migration should precede activation")
			evt := s.events.Captured()[0].(*approval.TaskHandedOverEvent)
			s.Equal(tc.from, evt.Predecessor.ID, "The event should identify the departed user")
			s.Equal(tc.to, evt.Successor.ID, "The event should identify the recipient")
			s.Equal(tc.reason, evt.Reason, "The event should give the departure reason")

			var log approval.ActionLog
			s.Require().NoError(s.db.NewSelect().Model(&log).Where(func(cb orm.ConditionBuilder) { cb.Equals("task_id", task.ID) }).Scan(s.ctx), "Should load handover log")
			s.Equal(new(tc.to), log.TransferToID, "The log should name the actual recipient")
			s.Equal(new(tc.reason), log.Opinion, "The log should give the reason for this move")
		})
	}
}

func (s *HandoverTestSuite) TestDuplicateSeat() {
	for _, complete := range []bool{false, true} {
		s.Run(fmt.Sprintf("Complete%t", complete), func() {
			defer s.TearDownTest()

			inst, task := setupRunningInstance(s.T(), s.ctx, s.db, s.fixture, "old")
			next := &approval.Task{TenantID: inst.TenantID, InstanceID: inst.ID, NodeID: task.NodeID, VisitID: task.VisitID, AssigneeID: "next", Status: approval.TaskWaiting, SortOrder: 2}
			_, err := s.db.NewInsert().Model(next).Exec(s.ctx)
			s.Require().NoError(err, "Should seed successor's existing seat")

			if complete {
				// The applicant's queued seat auto-passes when the removal activates it.
				_, err = s.db.NewUpdate().Model(inst).Set("applicant_id", "next").WherePK().Exec(s.ctx)
				s.Require().NoError(err, "Should set applicant")
				_, err = s.db.NewUpdate().
					Model((*approval.FlowNode)(nil)).
					Set("same_applicant_action", approval.SameApplicantAutoPass).
					Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.NodeID) }).
					Exec(s.ctx)
				s.Require().NoError(err, "Should enable applicant auto-pass")
			}

			s.change("old", "next")
			s.Require().Len(s.apply("old").Transferred, 1, "Duplicate seat should be consolidated")
			s.Require().NoError(s.db.NewSelect().Model(task).WherePK().Scan(s.ctx), "Should load retired row")
			s.Equal(approval.TaskRemoved, task.Status, "Original seat must be removed")
			s.Require().NoError(s.db.NewSelect().Model(next).WherePK().Scan(s.ctx), "Should load surviving seat")
			types := s.eventTypes()
			s.Require().NotEmpty(types, "Removal should emit events")
			s.Equal(approval.EventTypeAssigneesRemoved, types[0], "Removal must precede queue effects")
			s.NotContains(types, approval.EventTypeTaskHandedOver, "Duplicate seat emits the standard removal event")

			if complete {
				s.Require().NoError(s.db.NewSelect().Model(inst).WherePK().Scan(s.ctx), "Should reload completed instance")
				s.Equal(approval.InstanceApproved, inst.Status, "Removal and automatic decision should advance to end")
				s.Equal(approval.EventTypeInstanceCompleted, types[len(types)-1], "Completion should follow its causes")
			} else {
				s.Equal(approval.TaskPending, next.Status, "Removal must promote the waiting successor")
			}
		})
	}
}

func (s *HandoverTestSuite) TestDepthLimitRollsBackEarlierInstance() {
	first, task := setupRunningInstance(s.T(), s.ctx, s.db, s.fixture, "old")
	second := &approval.Instance{
		TenantID:      first.TenantID,
		FlowID:        first.FlowID,
		FlowVersionID: first.FlowVersionID,
		Title:         "Second",
		InstanceNo:    "depth",
		ApplicantID:   "applicant",
		CurrentNodeID: first.CurrentNodeID,
		Status:        approval.InstanceRunning,
	}
	_, err := s.db.NewInsert().Model(second).Exec(s.ctx)
	s.Require().NoError(err, "Should seed second instance")
	visit := insertActiveVisit(s.T(), s.ctx, s.db, second.TenantID, second.ID, task.NodeID, 1)
	_, err = s.db.NewInsert().
		Model(&approval.Task{TenantID: second.TenantID, InstanceID: second.ID, VisitID: visit.ID, NodeID: task.NodeID, AssigneeID: "0", Status: approval.TaskPending}).
		Exec(s.ctx)
	s.Require().NoError(err, "Should seed overflowing seat")
	s.change("old", "new")

	for i := range 11 {
		s.change(fmt.Sprint(i), fmt.Sprint(i+1))
	}

	_, err = s.svc.ApplyHandovers(s.ctx, s.db, approval.ApplyHandoversInput{UserIDs: []string{"old", "0"}, Caller: approval.SystemCaller})
	s.Require().ErrorIs(err, engine.ErrHandoverDepth, "A chain exceeding ten hops must abort the transaction")
	s.Require().NoError(s.db.NewSelect().Model(task).WherePK().Scan(s.ctx), "Should reload first task")
	s.Equal("old", task.AssigneeID, "Earlier instance move must roll back")
	s.Empty(s.events.Captured(), "Failed batch must not emit events")
}

func (s *HandoverTestSuite) TestReportsAndTenantScope() {
	inst, task := setupRunningInstance(s.T(), s.ctx, s.db, s.fixture, "departed")
	s.change("departed", "")
	s.change("applicant-1", "next")

	for _, state := range []approval.InstanceStatus{approval.InstanceReturned, approval.InstanceWithdrawn} {
		archived := &approval.Instance{
			TenantID:      "default",
			FlowID:        inst.FlowID,
			FlowVersionID: inst.FlowVersionID,
			InstanceNo:    string(state),
			Title:         "Returned work",
			ApplicantID:   "applicant-1",
			CurrentNodeID: inst.CurrentNodeID,
			Status:        state,
		}
		_, err := s.db.NewInsert().Model(archived).Exec(s.ctx)
		s.Require().NoError(err, "Should create awaiting instance")
	}

	r := s.apply("departed", "applicant-1")
	s.Require().Len(r.Unassigned, 1, "Departure without successor requires administrator attention")
	s.Equal(task.ID, *r.Unassigned[0].TaskID, "Should identify unassigned work")
	s.Len(r.AwaitingApplicant, 2, "Returned and withdrawn instances remain with the applicant")
	r, err := s.svc.ApplyHandovers(s.ctx, s.db, approval.ApplyHandoversInput{UserIDs: []string{"departed", "applicant-1"}, Caller: approval.CallerContext{TenantID: "other"}})
	s.Require().NoError(err, "Tenant-scoped call should return a report")
	s.Empty(r.Unassigned, "Other tenant's work must remain invisible")
	s.Empty(r.AwaitingApplicant, "Other tenant's instances must remain invisible")
	s.Empty(s.eventTypes(), "Reporting must not publish events")
}

func (s *HandoverTestSuite) TestConcurrentScannersAndCycles() {
	_, task := setupRunningInstance(s.T(), s.ctx, s.db, s.fixture, "old")

	var dispatches atomic.Int32

	bus := cqrs.NewBus(nil)
	cqrs.Register(bus, cqrs.HandlerFunc[command.ApplyHandoversCmd, *approval.HandoverReport](func(ctx context.Context, cmd command.ApplyHandoversCmd) (*approval.HandoverReport, error) {
		dispatches.Add(1)

		return cqrs.Send[command.ApplyHandoversCmd, *approval.HandoverReport](ctx, s.bus, cmd)
	}))
	scanner := timeout.NewHandoverScanner(s.db, bus, s.resolver)
	scanner.Scan(s.ctx)
	s.Zero(dispatches.Load(), "Active seats without handovers must never dispatch a write transaction")
	s.change("old", "")

	for range 2 {
		scanner.Scan(s.ctx)
	}

	s.Zero(dispatches.Load(), "Repeated scans of unassigned seats must stay read-only")
	s.change("old", "next")

	var group sync.WaitGroup
	for range 2 {
		group.Go(func() { scanner.Scan(s.ctx) })
	}

	group.Wait()
	s.Require().NoError(s.db.NewSelect().Model(task).WherePK().Scan(s.ctx), "Should read scanned task")
	s.Equal("next", task.AssigneeID, "Scanner should apply the same migration as manual calls")
	s.Equal([]string{approval.EventTypeTaskHandedOver, approval.EventTypeTaskActivated}, s.eventTypes(), "Replicas must migrate a seat only once")

	afterMove := dispatches.Load()
	s.Positive(afterMove, "An actionable handover must dispatch a migration")
	s.change("next", "B")
	s.change("B", "C")
	s.change("C", "B")

	for range 2 {
		scanner.Scan(s.ctx)
	}

	s.Equal(afterMove, dispatches.Load(), "Stable cycles should not reopen write transactions")
	r := s.apply("next")
	s.Empty(r.Transferred, "Cycle that resolves to the current actor must be a no-op")
	s.Empty(r.Unassigned, "A cycle is not a departure without successor")
	s.Len(s.events.Captured(), 2, "Repeated scans must not emit additional events")

	var logs []approval.ActionLog
	s.Require().NoError(s.db.NewSelect().Model(&logs).Where(func(cb orm.ConditionBuilder) { cb.Equals("task_id", task.ID) }).Scan(s.ctx), "Should find scanner logs")
	s.Require().Len(logs, 1, "Repeated scans should not log a no-op")
	s.Equal(orm.OperatorSystem, logs[0].OperatorID, "Scanner acts as system")
}

func (s *HandoverTestSuite) TestScannerRechecksDelegateeDeparture() {
	_, task := setupRunningInstance(s.T(), s.ctx, s.db, s.fixture, "delegate")
	_, err := s.db.NewUpdate().Model(task).Set("delegator_id", "seat").WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should seed delegated task")

	// Discovery sees a departure without a successor. The first locked
	// recheck sees that HR has withdrawn it, the next one confirms it.
	discovery := scopedHandovers{"delegate": {}}
	scanner := timeout.NewHandoverScanner(s.db, s.bus, discovery)
	scanner.Scan(s.ctx)
	s.Empty(s.events.Captured(), "An obsolete read-only discovery must not move the task")
	s.change("delegate", "")
	scanner.Scan(s.ctx)
	s.Require().NoError(s.db.NewSelect().Model(task).WherePK().Scan(s.ctx), "Should read the restored seat")
	s.Equal("seat", task.AssigneeID, "A delegate with no successor should still trigger a return to the seat")
	s.Equal([]string{approval.EventTypeTaskHandedOver, approval.EventTypeTaskActivated}, s.eventTypes(), "Only the confirmed change should emit events")
}

func (s *HandoverTestSuite) TestReadConfirmationReceipts() {
	def := approvalFlowDef()
	def.Nodes[1] = approval.NodeDefinition{
		ID:   "approval-1",
		Kind: approval.NodeCC,
		Data: mustMarshal(approval.CCNodeData{Name: "Read confirmation", IsReadConfirmRequired: true, CCs: []approval.CCDefinition{{Kind: approval.CCUser, IDs: []string{"old"}}}}),
	}
	fixture := deployAndPublishFlow(s.T(), s.ctx, s.db, "handover-cc", def)
	nodeID := fixture.NodeIDs["approval-1"]
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        fixture.FlowID,
		FlowVersionID: fixture.VersionID,
		InstanceNo:    "CC",
		ApplicantID:   "applicant",
		Title:         "Confirm receipt",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &nodeID,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should create instance")
	previous := insertActiveVisit(s.T(), s.ctx, s.db, "default", inst.ID, nodeID, 1)
	_, err = s.db.NewUpdate().Model(previous).Set("status", approval.NodeVisitPassed).WherePK().Exec(s.ctx)
	s.Require().NoError(err, "Should finish historical visit")
	current := insertActiveVisit(s.T(), s.ctx, s.db, "default", inst.ID, nodeID, 2)
	receipt := func(visit *approval.NodeVisit, user string, read *timex.DateTime) *approval.CCRecord {
		r := &approval.CCRecord{InstanceID: inst.ID, NodeID: &nodeID, VisitID: &visit.ID, CCUserID: user, ReadAt: read}
		_, err := s.db.NewInsert().Model(r).Exec(s.ctx)
		s.Require().NoError(err, "Should insert CC receipt")

		return r
	}
	historical := receipt(previous, "old", nil)
	pending := receipt(current, "old", nil)

	s.change("old", "first")
	report := s.apply("old")
	s.Require().Len(report.Transferred, 1, "Only the current unread confirmation should move")
	s.Equal(pending.ID, *report.Transferred[0].CCRecordID, "Should identify receipt")
	s.Require().NoError(s.db.NewSelect().Model(pending).WherePK().Scan(s.ctx), "Should reload receipt")
	s.Equal("first", pending.CCUserID, "Recipient changes in place")
	s.Equal(new("dept"), pending.CCUserDepartmentID, "Should capture successor department")

	var log approval.ActionLog
	s.Require().NoError(s.db.NewSelect().Model(&log).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("instance_id", inst.ID).Equals("action", approval.ActionHandover)
	}).Scan(s.ctx), "Should load receipt migration log")
	s.Empty(log.CCUsers, "A handover log should not describe the departed person as an added CC recipient")
	s.Equal(new("first"), log.TransferToID, "Receipt migration should identify the new recipient")

	s.change("first", "middle")
	timeout.NewHandoverScanner(s.db, s.bus, s.resolver).Scan(s.ctx)
	s.Require().NoError(s.db.NewSelect().Model(pending).WherePK().Scan(s.ctx), "Should reload the receipt after discovery and migration")
	s.Equal("middle", pending.CCUserID, "An instance with only an actionable receipt must be scanned")
	receipt(current, "final", new(timex.Now()))
	s.change("middle", "final")
	report = s.apply("middle")
	s.Len(report.Transferred, 1, "Should merge with existing read receipt")
	exists, err := s.db.NewSelect().Model(pending).WherePK().Exists(s.ctx)
	s.Require().NoError(err, "Should check old receipt")
	s.False(exists, "Old unread receipt should be removed")
	s.Require().NoError(s.db.NewSelect().Model(inst).WherePK().Scan(s.ctx), "Should reload instance")
	s.Equal(approval.InstanceApproved, inst.Status, "Last confirmation clearing should advance to the end")
	s.Require().NoError(s.db.NewSelect().Model(historical).WherePK().Scan(s.ctx), "Should load historical receipt")
	s.Equal("old", historical.CCUserID, "Historical receipt must preserve original recipient")
}
