package command_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &DesignerDefaultsTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// DesignerDefaultsTestSuite is the regression net for the designer-default
// contract: a flow whose approval node carries only the fields the designer
// actually serializes for untouched controls (name + assignees) must deploy
// with complete defaults and run the full approve lifecycle. This is the
// exact payload shape that previously deployed fine and then stalled every
// instance with "pass rule strategy not found".
type DesignerDefaultsTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	engine  *engine.FlowEngine
	approve cqrs.Handler[command.ApproveTaskCmd, cqrs.Unit]
}

// bareApprovalFlowDef mirrors the designer's untouched-node output: no
// approval method, no pass rule, no permission toggles — only a name and one
// concrete assignee.
func bareApprovalFlowDef(approverID string) approval.FlowDefinition {
	return approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{BaseNodeData: approval.BaseNodeData{Name: "开始"}})},
			{ID: "approval-1", Kind: approval.NodeApproval, Data: mustMarshal(approval.ApprovalNodeData{
				BaseNodeData: approval.BaseNodeData{Name: "审批节点"},
				TaskNodeData: approval.TaskNodeData{
					Assignees: []approval.AssigneeDefinition{
						{Kind: approval.AssigneeUser, IDs: []string{approverID}, SortOrder: 1},
					},
				},
			})},
			{ID: "end-1", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{BaseNodeData: approval.BaseNodeData{Name: "结束"}})},
		},
		Edges: []approval.EdgeDefinition{
			{ID: "edge-1", Source: "start-1", Target: "approval-1"},
			{ID: "edge-2", Source: "approval-1", Target: "end-1"},
		},
	}
}

func (s *DesignerDefaultsTestSuite) SetupSuite() {
	s.engine = buildTestEngine(s.db)
	taskSvc, nodeSvc, validSvc := buildTestServices(s.engine)
	s.approve = wrapWithBusAndDB(s.db, eventtest.NewFakeBus(), command.NewApproveTaskHandler(s.db, taskSvc, nodeSvc, validSvc, nil))
}

func (s *DesignerDefaultsTestSuite) TearDownTest() {
	cleanAllApprovalData(s.ctx, s.db)
}

// startInstance inserts a running instance for the fixture flow and drives
// the engine from the start node, mirroring StartInstanceHandler's engine
// hand-off without its form/title machinery.
func (s *DesignerDefaultsTestSuite) startInstance(fixture *FlowFixture, instanceNo string) *approval.Instance {
	instance := &approval.Instance{
		TenantID:      "default",
		FlowID:        fixture.FlowID,
		FlowVersionID: fixture.VersionID,
		Title:         "Designer Defaults " + instanceNo,
		InstanceNo:    instanceNo,
		ApplicantID:   "applicant-1",
		ApplicantName: "Applicant",
		Status:        approval.InstanceRunning,
	}
	_, err := s.db.NewInsert().Model(instance).Exec(s.ctx)
	s.Require().NoError(err, "Instance should insert successfully")

	s.Require().NoError(s.engine.StartProcess(s.ctx, s.db, instance), "Engine should process the flow from the start node")

	return instance
}

func (s *DesignerDefaultsTestSuite) TestBareApprovalNode() {
	fixture := deployAndPublishFlow(s.T(), s.ctx, s.db, "designer-defaults", bareApprovalFlowDef("dd-approver"))

	s.Run("DeployResolvesDesignerDefaults", func() {
		var node approval.FlowNode

		node.ID = fixture.NodeIDs["approval-1"]
		s.Require().NoError(s.db.NewSelect().Model(&node).WherePK().Scan(s.ctx), "Approval node should load")

		s.Assert().Equal(approval.DefaultExecutionType, node.ExecutionType, "ExecutionType should default to manual")
		s.Assert().Equal(approval.DefaultApprovalMethod, node.ApprovalMethod, "ApprovalMethod should default to parallel")
		s.Assert().Equal(approval.DefaultPassRule, node.PassRule, "PassRule should default to all")
		s.Assert().Equal(approval.DefaultEmptyAssigneeAction, node.EmptyAssigneeAction, "EmptyAssigneeAction should default to auto_pass")
		s.Assert().Equal(approval.DefaultSameApplicantAction, node.SameApplicantAction, "SameApplicantAction should default to self_approve")
		s.Assert().Equal(approval.DefaultConsecutiveApproverAction, node.ConsecutiveApproverAction, "ConsecutiveApproverAction should default to none")
		s.Assert().Equal(approval.DefaultRollbackType, node.RollbackType, "RollbackType should default to previous")
		s.Assert().Equal(approval.DefaultRollbackDataStrategy, node.RollbackDataStrategy, "RollbackDataStrategy should default to keep")
		s.Assert().Equal(approval.DefaultTimeoutAction, node.TimeoutAction, "TimeoutAction should default to none")
		s.Assert().True(node.IsTransferAllowed, "IsTransferAllowed should default to allowed")
		s.Assert().True(node.IsRollbackAllowed, "IsRollbackAllowed should default to allowed")
		s.Assert().True(node.IsAddAssigneeAllowed, "IsAddAssigneeAllowed should default to allowed")
		s.Assert().True(node.IsRemoveAssigneeAllowed, "IsRemoveAssigneeAllowed should default to allowed")
		s.Assert().True(node.IsManualCCAllowed, "IsManualCCAllowed should default to allowed")
	})

	s.Run("FullLifecycleRunsToApproval", func() {
		instance := s.startInstance(fixture, "DD-001")

		var task approval.Task
		s.Require().NoError(s.db.NewSelect().Model(&task).
			Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instance.ID) }).
			Scan(s.ctx), "Start should create the approval task")
		s.Require().Equal(approval.TaskPending, task.Status, "Created task should be pending")
		s.Require().Equal("dd-approver", task.AssigneeID, "Task should target the configured assignee")

		_, err := s.approve.Handle(s.ctx, command.ApproveTaskCmd{
			TaskID:   task.ID,
			Operator: approval.OperatorInfo{ID: "dd-approver", Name: "Approver"},
			Opinion:  "ok",
			Caller:   approval.SystemCaller,
		})
		s.Require().NoError(err, "Approving the only task must succeed on a defaults-only node")

		var updated approval.Instance

		updated.ID = instance.ID
		s.Require().NoError(s.db.NewSelect().Model(&updated).WherePK().Scan(s.ctx), "Instance should reload")
		s.Assert().Equal(approval.InstanceApproved, updated.Status, "Instance should complete as approved")
	})
}

func (s *DesignerDefaultsTestSuite) TestAutoExecutionTypes() {
	s.Run("AutoPassSkipsNode", func() {
		def := bareApprovalFlowDef("ignored")
		def.Nodes[1].Data = mustMarshal(approval.ApprovalNodeData{
			BaseNodeData: approval.BaseNodeData{Name: "自动通过"},
			TaskNodeData: approval.TaskNodeData{ExecutionType: approval.ExecutionAutoPass},
		})

		fixture := deployAndPublishFlow(s.T(), s.ctx, s.db, "auto-pass", def)
		instance := s.startInstance(fixture, "AP-001")

		var updated approval.Instance

		updated.ID = instance.ID
		s.Require().NoError(s.db.NewSelect().Model(&updated).WherePK().Scan(s.ctx), "Instance should reload")
		s.Assert().Equal(approval.InstanceApproved, updated.Status, "Auto-pass node should let the instance run straight to approved")

		taskCount, err := s.db.NewSelect().Model((*approval.Task)(nil)).
			Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instance.ID) }).
			Count(s.ctx)
		s.Require().NoError(err, "Task count query should succeed")
		s.Assert().Zero(taskCount, "Auto-pass node must not create tasks")
	})

	s.Run("AutoRejectCompletesInstanceRejected", func() {
		def := bareApprovalFlowDef("ignored")
		def.Nodes[1].Data = mustMarshal(approval.ApprovalNodeData{
			BaseNodeData: approval.BaseNodeData{Name: "自动拒绝"},
			TaskNodeData: approval.TaskNodeData{ExecutionType: approval.ExecutionAutoReject},
		})

		fixture := deployAndPublishFlow(s.T(), s.ctx, s.db, "auto-reject", def)
		instance := s.startInstance(fixture, "AR-001")

		var updated approval.Instance

		updated.ID = instance.ID
		s.Require().NoError(s.db.NewSelect().Model(&updated).WherePK().Scan(s.ctx), "Instance should reload")
		s.Assert().Equal(approval.InstanceRejected, updated.Status, "Auto-reject node should complete the instance as rejected")
	})
}
