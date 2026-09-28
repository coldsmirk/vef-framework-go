package command_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/binding"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// fixedPassCount is the threshold every flow in PassRuleFixedCountTestSuite
// configures.
const fixedPassCount = 3

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &PassRuleFixedCountTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// PassRuleFixedCountTestSuite drives the fixed_count pass rule end to end: the
// threshold survives deploy, the node concludes the moment it is met, a panel
// smaller than the threshold needs every member, and rejections fail the node
// only once the threshold is out of reach.
type PassRuleFixedCountTestSuite struct {
	suite.Suite

	ctx   context.Context
	db    orm.DB
	panel *FlowFixture
	pair  *FlowFixture

	start   cqrs.Handler[command.StartInstanceCmd, *approval.Instance]
	approve cqrs.Handler[command.ApproveTaskCmd, cqrs.Unit]
	reject  cqrs.Handler[command.RejectTaskCmd, cqrs.Unit]
}

// fixedCountFlowDef builds start → approval(parallel, fixed_count 3) → end
// with the given approvers.
func fixedCountFlowDef(approvers ...string) approval.FlowDefinition {
	return approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{Name: "开始"})},
			{
				ID:   "approval-1",
				Kind: approval.NodeApproval,
				Data: mustMarshal(approval.ApprovalNodeData{
					Name: "专家评审",
					Assignees: []approval.AssigneeDefinition{
						{Kind: approval.AssigneeUser, IDs: approvers, SortOrder: 1},
					},
					ApprovalMethod: approval.ApprovalParallel,
					PassRule:       approval.PassFixedCount,
					PassCount:      fixedPassCount,
				}),
			},
			{ID: "end-1", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{Name: "结束"})},
		},
		Edges: []approval.EdgeDefinition{
			{ID: "edge-1", Source: "start-1", Target: "approval-1"},
			{ID: "edge-2", Source: "approval-1", Target: "end-1"},
		},
	}
}

func (s *PassRuleFixedCountTestSuite) SetupSuite() {
	s.panel = deployAndPublishFlow(s.T(), s.ctx, s.db, "fixed-count-panel", fixedCountFlowDef("e-1", "e-2", "e-3", "e-4", "e-5"))
	s.pair = deployAndPublishFlow(s.T(), s.ctx, s.db, "fixed-count-pair", fixedCountFlowDef("e-1", "e-2"))

	eng := buildTestEngine(s.db)
	taskSvc, nodeSvc, validSvc := buildTestServices(eng)
	bus := eventtest.NewFakeBus()

	s.start = wrapWithBusAndDB(
		s.db,
		bus,
		command.NewStartInstanceHandler(
			s.db,
			eng,
			&MockInstanceNoGenerator{},
			validSvc,
			binding.NewNoopRefProvider(),
			binding.NewProjector(binding.NewIdentityResolver(), binding.NewWriter(), nil),
			nil,
		),
	)
	s.approve = wrapWithBusAndDB(s.db, bus, command.NewApproveTaskHandler(s.db, taskSvc, nodeSvc, validSvc, nil))
	s.reject = wrapWithBusAndDB(s.db, bus, command.NewRejectTaskHandler(s.db, taskSvc, nodeSvc, validSvc, nil))
}

func (s *PassRuleFixedCountTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *PassRuleFixedCountTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

// startInstance starts an instance of the flow deployed under code.
func (s *PassRuleFixedCountTestSuite) startInstance(code string) *approval.Instance {
	instance, err := s.start.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  code + "-flow",
		Applicant: approval.UserInfo{ID: "applicant-1", Name: "Applicant"},
		FormData:  map[string]any{},
		Caller:    approval.SystemCaller,
	})
	s.Require().NoError(err, "Instance should start")

	return instance
}

// instanceStatus reloads the instance's status.
func (s *PassRuleFixedCountTestSuite) instanceStatus(instanceID string) approval.InstanceStatus {
	instance := approval.Instance{ID: instanceID}
	s.Require().NoError(s.db.NewSelect().Model(&instance).WherePK().Scan(s.ctx), "Instance should reload")

	return instance.Status
}

// taskStatuses maps each assignee to the status of their task.
func (s *PassRuleFixedCountTestSuite) taskStatuses(instanceID string) map[string]approval.TaskStatus {
	var tasks []approval.Task
	s.Require().NoError(s.db.NewSelect().Model(&tasks).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("instance_id", instanceID)
	}).Scan(s.ctx), "Tasks should load")

	statuses := make(map[string]approval.TaskStatus, len(tasks))
	for _, task := range tasks {
		statuses[task.AssigneeID] = task.Status
	}

	return statuses
}

// pendingTaskID returns the ID of the assignee's pending task.
func (s *PassRuleFixedCountTestSuite) pendingTaskID(instanceID, assigneeID string) string {
	var task approval.Task
	s.Require().NoError(s.db.NewSelect().Model(&task).Where(func(cb orm.ConditionBuilder) {
		cb.Equals("instance_id", instanceID).
			Equals("assignee_id", assigneeID).
			Equals("status", approval.TaskPending)
	}).Scan(s.ctx), "%s should have a pending task", assigneeID)

	return task.ID
}

func (s *PassRuleFixedCountTestSuite) approveAs(instanceID, assigneeID string) {
	_, err := s.approve.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   s.pendingTaskID(instanceID, assigneeID),
		Operator: approval.UserInfo{ID: assigneeID, Name: assigneeID},
		Opinion:  "agree",
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "%s should approve", assigneeID)
}

func (s *PassRuleFixedCountTestSuite) rejectAs(instanceID, assigneeID string) {
	_, err := s.reject.Handle(s.ctx, command.RejectTaskCmd{
		TaskID:   s.pendingTaskID(instanceID, assigneeID),
		Operator: approval.UserInfo{ID: assigneeID, Name: assigneeID},
		Opinion:  "disagree",
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "%s should reject", assigneeID)
}

func (s *PassRuleFixedCountTestSuite) TestDeployKeepsThreshold() {
	node := approval.FlowNode{ID: s.panel.NodeIDs["approval-1"]}
	s.Require().NoError(s.db.NewSelect().Model(&node).WherePK().Scan(s.ctx), "Approval node should load")

	s.Assert().Equal(approval.PassFixedCount, node.PassRule, "The rule should be stored on the node")
	s.Assert().Equal(fixedPassCount, node.PassCount, "The threshold should be stored on the node")
}

func (s *PassRuleFixedCountTestSuite) TestPassesAtThreshold() {
	instance := s.startInstance("fixed-count-panel")

	s.approveAs(instance.ID, "e-1")
	s.approveAs(instance.ID, "e-2")
	s.Assert().Equal(approval.InstanceRunning, s.instanceStatus(instance.ID), "Two of three approvals should keep the node open")

	s.approveAs(instance.ID, "e-3")
	s.Assert().Equal(approval.InstanceApproved, s.instanceStatus(instance.ID), "The third approval should complete the flow")

	statuses := s.taskStatuses(instance.ID)
	s.Assert().Equal(approval.TaskCanceled, statuses["e-4"], "Seats left undecided at the threshold should be canceled")
	s.Assert().Equal(approval.TaskCanceled, statuses["e-5"], "Seats left undecided at the threshold should be canceled")
}

func (s *PassRuleFixedCountTestSuite) TestSmallPanelNeedsEveryone() {
	instance := s.startInstance("fixed-count-pair")

	s.approveAs(instance.ID, "e-1")
	s.Assert().Equal(approval.InstanceRunning, s.instanceStatus(instance.ID), "A capped threshold should still wait for the whole panel")

	s.approveAs(instance.ID, "e-2")
	s.Assert().Equal(approval.InstanceApproved, s.instanceStatus(instance.ID), "The whole panel approving should complete the flow")
}

func (s *PassRuleFixedCountTestSuite) TestRejectsOnceUnreachable() {
	instance := s.startInstance("fixed-count-panel")

	s.rejectAs(instance.ID, "e-1")
	s.rejectAs(instance.ID, "e-2")
	s.Assert().Equal(approval.InstanceRunning, s.instanceStatus(instance.ID), "Three remaining approvers can still reach the threshold")

	s.rejectAs(instance.ID, "e-3")
	s.Assert().Equal(approval.InstanceRejected, s.instanceStatus(instance.ID), "Two remaining approvers can no longer reach the threshold")
}
