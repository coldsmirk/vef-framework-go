package command_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/decimal"
	"github.com/coldsmirk/vef-framework-go/internal/approval/binding"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &PassRuleVisitScopeTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// PassRuleVisitScopeTestSuite pins the visit scoping of pass-rule evaluation:
// tasks left behind by an earlier traversal of a node (an approval that
// survived a peer-initiated rollback) must not count toward the redo round's
// decision.
type PassRuleVisitScopeTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	fixture *FlowFixture

	start    cqrs.Handler[command.StartInstanceCmd, *approval.Instance]
	approve  cqrs.Handler[command.ApproveTaskCmd, cqrs.Unit]
	rollback cqrs.Handler[command.RollbackTaskCmd, cqrs.Unit]
	resubmit cqrs.Handler[command.ResubmitInstanceCmd, cqrs.Unit]
}

func (s *PassRuleVisitScopeTestSuite) SetupSuite() {
	s.fixture = deployAndPublishFlow(s.T(), s.ctx, s.db, "ratio-scope", ratioFlowDef())

	eng := buildTestEngine(s.db)
	taskSvc, nodeSvc, validSvc := buildTestServices(eng)
	instanceSvc := service.NewInstanceService(nil)
	bus := eventtest.NewFakeBus()

	s.start = wrapWithBusAndDB(
		s.db,
		bus,
		command.NewStartInstanceHandler(s.db, eng, &MockInstanceNoGenerator{}, validSvc, binding.NewNoopRefProvider(), binding.NewWriter(binding.NewIdentityResolver()), nil),
	)
	s.approve = wrapWithBusAndDB(s.db, bus, command.NewApproveTaskHandler(s.db, taskSvc, nodeSvc, validSvc, nil))
	s.rollback = wrapWithBusAndDB(s.db, bus, command.NewRollbackTaskHandler(s.db, taskSvc, instanceSvc, validSvc, eng, nil))
	s.resubmit = wrapWithBusAndDB(s.db, bus, command.NewResubmitInstanceHandler(s.db, eng, validSvc, instanceSvc, nil))
}

func (s *PassRuleVisitScopeTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *PassRuleVisitScopeTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

// ratioFlowDef builds start → approval(parallel, ratio 67%, three assignees) → end.
// Two of three approvals is 66.7% — just below the threshold — so the third
// vote is always required.
func ratioFlowDef() approval.FlowDefinition {
	return approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{BaseNodeData: approval.BaseNodeData{Name: "开始"}})},
			{ID: "approval-1", Kind: approval.NodeApproval, Data: mustMarshal(approval.ApprovalNodeData{
				BaseNodeData: approval.BaseNodeData{Name: "会签"},
				TaskNodeData: approval.TaskNodeData{
					Assignees: []approval.AssigneeDefinition{
						{Kind: approval.AssigneeUser, IDs: []string{"u-1", "u-2", "u-3"}, SortOrder: 1},
					},
					ExecutionType:       approval.ExecutionManual,
					EmptyAssigneeAction: approval.EmptyAssigneeAutoPass,
				},
				ApprovalMethod: approval.ApprovalParallel,
				PassRule:       approval.PassRatio,
				PassRatio:      decimal.NewFromInt(67),
			})},
			{ID: "end-1", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{BaseNodeData: approval.BaseNodeData{Name: "结束"}})},
		},
		Edges: []approval.EdgeDefinition{
			{ID: "edge-1", Source: "start-1", Target: "approval-1"},
			{ID: "edge-2", Source: "approval-1", Target: "end-1"},
		},
	}
}

// loadInstance reloads the instance row.
func (s *PassRuleVisitScopeTestSuite) loadInstance(instanceID string) *approval.Instance {
	var instance approval.Instance

	instance.ID = instanceID
	s.Require().NoError(s.db.NewSelect().Model(&instance).WherePK().Scan(s.ctx), "Should reload instance")

	return &instance
}

// pendingTaskOf returns the pending task of the given assignee.
func (s *PassRuleVisitScopeTestSuite) pendingTaskOf(instanceID, assigneeID string) *approval.Task {
	var task approval.Task

	s.Require().NoError(s.db.NewSelect().Model(&task).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instanceID).
				Equals("assignee_id", assigneeID).
				Equals("status", approval.TaskPending)
		}).
		Scan(s.ctx), "Should find the pending task of %s", assigneeID)

	return &task
}

// approveAs approves the assignee's pending task.
func (s *PassRuleVisitScopeTestSuite) approveAs(instanceID, assigneeID string) {
	task := s.pendingTaskOf(instanceID, assigneeID)
	_, err := s.approve.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   task.ID,
		Operator: approval.UserInfo{ID: assigneeID, Name: assigneeID},
		Opinion:  "ok",
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "%s should approve", assigneeID)
}

func (s *PassRuleVisitScopeTestSuite) TestRedoCountsOnlyItsOwnRound() {
	instance, err := s.start.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  "ratio-scope-flow",
		Applicant: approval.UserInfo{ID: "applicant-1", Name: "Applicant"},
		FormData:  map[string]any{"reason": "scope"},
		Caller:    approval.SystemCaller,
	})
	s.Require().NoError(err, "Should start instance through the engine")

	// Round 1: u-1 approves (1/3 = 33%, node stays open), then u-2 sends the
	// flow back to the applicant. u-3's pending task is canceled; u-1's
	// approved task survives untouched — the contamination source.
	s.approveAs(instance.ID, "u-1")

	rollbackTask := s.pendingTaskOf(instance.ID, "u-2")
	_, err = s.rollback.Handle(s.ctx, command.RollbackTaskCmd{
		TaskID:       rollbackTask.ID,
		Operator:     approval.UserInfo{ID: "u-2", Name: "u-2"},
		Opinion:      "back to applicant",
		TargetNodeID: s.fixture.NodeIDs["start-1"],
		Caller:       approval.SystemCaller,
	})
	s.Require().NoError(err, "u-2 should roll the flow back to the applicant")
	s.Assert().Equal(approval.InstanceReturned, s.loadInstance(instance.ID).Status, "Rollback to start should pause the instance")

	_, err = s.resubmit.Handle(s.ctx, command.ResubmitInstanceCmd{
		InstanceID: instance.ID,
		Operator:   approval.UserInfo{ID: "applicant-1", Name: "Applicant"},
		FormData:   map[string]any{"reason": "fixed"},
		Caller:     approval.SystemCaller,
	})
	s.Require().NoError(err, "Applicant should resubmit")

	// Round 2: two of three approve — 66.7%, below the 67% threshold. Counting
	// contaminated by the stale round-1 approval would see 3 approved of 4
	// counted (75%) and pass the node a vote early.
	s.approveAs(instance.ID, "u-1")
	s.approveAs(instance.ID, "u-2")
	s.Assert().Equal(approval.InstanceRunning, s.loadInstance(instance.ID).Status,
		"Two of three round-2 approvals must keep the node open — the stale round-1 approval must not count")

	// The genuine third vote crosses the threshold.
	s.approveAs(instance.ID, "u-3")
	s.Assert().Equal(approval.InstanceApproved, s.loadInstance(instance.ID).Status, "Third round-2 approval should complete the instance")

	// The stale round-1 task is preserved as history, merely excluded from
	// counting.
	stale := approval.Task{}
	stale.ID = rollbackTask.ID
	s.Require().NoError(s.db.NewSelect().Model(&stale).WherePK().Scan(s.ctx), "Should reload the rolled-back task")
	s.Assert().Equal(approval.TaskRolledBack, stale.Status, "Round-1 rollback task keeps its status")

	approvedCount, err := s.db.NewSelect().Model((*approval.Task)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("instance_id", instance.ID).
				Equals("status", approval.TaskApproved)
		}).
		Count(s.ctx)
	s.Require().NoError(err, "Should count approved tasks")
	s.Assert().EqualValues(4, approvedCount, "Round-1 approval plus all three round-2 approvals remain on record")
}
