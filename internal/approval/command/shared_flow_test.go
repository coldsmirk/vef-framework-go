package command_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/binding"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/formeditor"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &SharedFlowTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// singleApprovalFlowDef is a Start → Approval → End flow around the given
// approval node.
func singleApprovalFlowDef(node approval.ApprovalNodeData) approval.FlowDefinition {
	node.Name = "审批"
	node.ExecutionType = approval.ExecutionManual
	node.ApprovalMethod = approval.ApprovalSequential
	node.PassRule = approval.PassAll

	return approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{Name: "开始"})},
			{ID: "approval-1", Kind: approval.NodeApproval, Data: mustMarshal(node)},
			{ID: "end-1", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{Name: "结束"})},
		},
		Edges: []approval.EdgeDefinition{
			{ID: "edge-1", Source: "start-1", Target: "approval-1"},
			{ID: "edge-2", Source: "approval-1", Target: "end-1"},
		},
	}
}

// sharedSafeFlowDef routes its one approval to the applicant — a rule resolved
// from the instance, which every tenant can run.
func sharedSafeFlowDef() approval.FlowDefinition {
	return singleApprovalFlowDef(approval.ApprovalNodeData{
		Assignees:           []approval.AssigneeDefinition{{Kind: approval.AssigneeSelf, SortOrder: 1}},
		EmptyAssigneeAction: approval.EmptyAssigneeAutoPass,
	})
}

// SharedFlowTestSuite covers a shared flow end to end: the tenant its instance
// runs in, who may act on that instance, how a tenant flow of the same code
// replaces it, how the starting tenant is resolved, and the deploy rule that
// keeps its nodes from routing work to one tenant's people.
type SharedFlowTestSuite struct {
	suite.Suite

	ctx       context.Context
	db        orm.DB
	numbering *MockInstanceNoGenerator
	start     cqrs.Handler[command.StartInstanceCmd, *approval.Instance]
	approve   cqrs.Handler[command.ApproveTaskCmd, cqrs.Unit]
	deploy    *command.DeployFlowHandler

	shared         *FlowFixture
	sharedReplaced *FlowFixture
	tenantOwn      *FlowFixture
}

func (s *SharedFlowTestSuite) SetupSuite() {
	s.shared = deployAndPublishFlowIn(s.T(), s.ctx, s.db, approval.SharedTenantID, "shr", sharedSafeFlowDef())
	// The default tenant replaces this shared flow with one of the same code.
	s.sharedReplaced = deployAndPublishFlowIn(s.T(), s.ctx, s.db, approval.SharedTenantID, "shr-rep", sharedSafeFlowDef())
	s.tenantOwn = deployAndPublishFlowIn(s.T(), s.ctx, s.db, approval.DefaultTenantID, "shr-rep", sharedSafeFlowDef())

	eng := buildTestEngine(s.db)
	taskSvc, nodeSvc, validSvc := buildTestServices(eng)

	s.numbering = new(MockInstanceNoGenerator)
	s.start = wrapWithBusAndDB(s.db, eventtest.NewFakeBus(),
		command.NewStartInstanceHandler(s.db, eng, s.numbering, validSvc, binding.NewNoopRefProvider(), nil, nil))
	s.approve = wrapWithBusAndDB(s.db, eventtest.NewFakeBus(),
		command.NewApproveTaskHandler(s.db, taskSvc, nodeSvc, validSvc, nil))
	s.deploy = command.NewDeployFlowHandler(s.db, service.NewFlowDefinitionService(), formeditor.NewParser())
}

func (s *SharedFlowTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *SharedFlowTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *SharedFlowTestSuite) startAs(caller approval.CallerContext, tenantID, code, applicantID string) (*approval.Instance, error) {
	return s.start.Handle(s.ctx, command.StartInstanceCmd{
		TenantID:  tenantID,
		FlowCode:  code,
		Applicant: approval.UserInfo{ID: applicantID, Name: applicantID},
		Caller:    caller,
	})
}

func (s *SharedFlowTestSuite) tasksOf(instanceID string) []approval.Task {
	var tasks []approval.Task

	s.Require().NoError(s.db.NewSelect().Model(&tasks).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instanceID) }).
		Scan(s.ctx), "Should load the instance's tasks")

	return tasks
}

func (s *SharedFlowTestSuite) TestInstanceRunsInStartingTenant() {
	instance, err := s.startAs(approval.CallerContext{TenantID: "tenant-a"}, "", "shr-flow", "user-a")
	s.Require().NoError(err, "A tenant should start a shared flow like one of its own")
	s.Assert().Equal(s.shared.FlowID, instance.FlowID, "The shared flow should run")
	s.Assert().Equal("tenant-a", instance.TenantID,
		"The instance belongs to the starting tenant, never to the shared one")
	s.Assert().Equal("tenant-a", s.numbering.tenants[len(s.numbering.tenants)-1],
		"The instance number is drawn for the starting tenant, so a host can number per tenant")

	tasks := s.tasksOf(instance.ID)
	s.Require().NotEmpty(tasks, "The approval node should open a task")

	for _, task := range tasks {
		s.Assert().Equal("tenant-a", task.TenantID, "Tasks inherit the instance's tenant")
	}

	var visits []approval.NodeVisit

	s.Require().NoError(s.db.NewSelect().Model(&visits).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instance.ID) }).
		Scan(s.ctx), "Should load the instance's visits")

	for _, visit := range visits {
		s.Assert().Equal("tenant-a", visit.TenantID, "Visits inherit the instance's tenant")
	}
}

func (s *SharedFlowTestSuite) TestStartingTenantActsOnInstance() {
	instance, err := s.startAs(approval.CallerContext{TenantID: "tenant-a"}, "", "shr-flow", "user-a")
	s.Require().NoError(err, "Should start the shared flow")

	tasks := s.tasksOf(instance.ID)
	s.Require().Len(tasks, 1, "The self rule opens one task, on the applicant")

	cmd := command.ApproveTaskCmd{
		TaskID:   tasks[0].ID,
		Operator: approval.UserInfo{ID: "user-a", Name: "user-a"},
		Caller:   approval.CallerContext{TenantID: "tenant-b"},
	}

	_, err = s.approve.Handle(s.ctx, cmd)
	s.Require().ErrorIs(err, approval.ErrTaskNotFound,
		"Another tenant must not reach an instance of a shared flow")

	cmd.Caller = approval.CallerContext{TenantID: "tenant-a"}
	_, err = s.approve.Handle(s.ctx, cmd)
	s.Require().NoError(err, "The starting tenant acts on its instance like on any of its own")

	var stored approval.Instance

	stored.ID = instance.ID
	s.Require().NoError(s.db.NewSelect().Model(&stored).WherePK().Scan(s.ctx), "Should reload the instance")
	s.Assert().Equal(approval.InstanceApproved, stored.Status, "The single approval should complete the instance")
}

func (s *SharedFlowTestSuite) TestTenantFlowReplacesSharedFlow() {
	own, err := s.startAs(approval.CallerContext{TenantID: approval.DefaultTenantID}, "", "shr-rep-flow", "user-d")
	s.Require().NoError(err, "The default tenant should start its own flow of the code")
	s.Assert().Equal(s.tenantOwn.FlowID, own.FlowID, "A tenant flow takes the shared flow's place in its tenant")

	shared, err := s.startAs(approval.CallerContext{TenantID: "tenant-b"}, "", "shr-rep-flow", "user-b")
	s.Require().NoError(err, "Another tenant should still start the shared flow")
	s.Assert().Equal(s.sharedReplaced.FlowID, shared.FlowID, "Only the replacing tenant loses the shared flow")
}

func (s *SharedFlowTestSuite) TestStartTenantResolution() {
	s.Run("PinsOrdinaryCallerToOwnTenant", func() {
		instance, err := s.startAs(approval.CallerContext{TenantID: "tenant-x"}, approval.DefaultTenantID, "shr-rep-flow", "user-x")
		s.Require().NoError(err, "The start should run in the caller's own tenant")
		s.Assert().Equal("tenant-x", instance.TenantID, "An ordinary caller cannot start in another tenant")
		s.Assert().Equal(s.sharedReplaced.FlowID, instance.FlowID,
			"Pinned to its own tenant, the caller gets the shared flow, not the default tenant's replacement")
	})

	s.Run("PrivilegedCallerStartsInRequestedTenant", func() {
		instance, err := s.startAs(approval.SystemCaller, "tenant-c", "shr-flow", "user-c")
		s.Require().NoError(err, "A privileged caller may start in any tenant")
		s.Assert().Equal("tenant-c", instance.TenantID, "The requested tenant should be honored")
	})

	s.Run("RejectsSharedTenant", func() {
		_, err := s.startAs(approval.SystemCaller, approval.SharedTenantID, "shr-flow", "user-s")
		s.Require().ErrorIs(err, approval.ErrFlowNotFound, "Nothing runs in the shared tenant")
	})

	s.Run("RejectsCallerClaimingSharedTenant", func() {
		_, err := s.startAs(approval.CallerContext{TenantID: approval.SharedTenantID}, "", "shr-flow", "user-s")
		s.Require().ErrorIs(err, approval.ErrFlowNotFound, "Nobody belongs to the shared tenant")
	})

	s.Run("FailsClosedWithoutTenant", func() {
		_, err := s.startAs(approval.CallerContext{}, "", "shr-flow", "user-z")
		s.Require().ErrorIs(err, approval.ErrFlowNotFound, "A caller without a tenant starts nothing")
	})
}

// TestDeployRule pins the deploy gate of shared flows: no rule may route tasks
// or CC to specific users, roles or departments, and no node may name admins or
// fallback assignees, while the same definition stays valid for a tenant flow.
func (s *SharedFlowTestSuite) TestDeployRule() {
	deploy := func(flowID string, def approval.FlowDefinition) error {
		_, err := s.deploy.Handle(s.ctx, command.DeployFlowCmd{
			FlowID:         flowID,
			FlowDefinition: def,
		})

		return err
	}

	withTask := func(task approval.TaskNodeData) approval.FlowDefinition {
		if len(task.Assignees) == 0 {
			task.Assignees = []approval.AssigneeDefinition{{Kind: approval.AssigneeSelf, SortOrder: 1}}
		}

		return singleApprovalFlowDef(approval.ApprovalNodeData{TaskNodeData: task})
	}

	violations := map[string]approval.FlowDefinition{
		"UserAssignee": withTask(approval.TaskNodeData{
			Assignees: []approval.AssigneeDefinition{{Kind: approval.AssigneeUser, IDs: []string{"user-1"}, SortOrder: 1}},
		}),
		"RoleAssignee": withTask(approval.TaskNodeData{
			Assignees: []approval.AssigneeDefinition{{Kind: approval.AssigneeRole, IDs: []string{"role-1"}, SortOrder: 1}},
		}),
		"DepartmentAssignee": withTask(approval.TaskNodeData{
			Assignees: []approval.AssigneeDefinition{{Kind: approval.AssigneeDepartment, IDs: []string{"dept-1"}, SortOrder: 1}},
		}),
		"UserCC": withTask(approval.TaskNodeData{
			CCs: []approval.CCDefinition{{Kind: approval.CCUser, IDs: []string{"cc-1"}, Timing: approval.CCTimingAlways}},
		}),
		"FallbackUsers": withTask(approval.TaskNodeData{
			EmptyAssigneeAction: approval.EmptyAssigneeTransferSpecified,
			FallbackUserIDs:     []string{"user-1"},
		}),
		"NodeAdmins": withTask(approval.TaskNodeData{
			EmptyAssigneeAction: approval.EmptyAssigneeTransferAdmin,
			AdminUserIDs:        []string{"admin-1"},
		}),
		"TimeoutToAdmins": withTask(approval.TaskNodeData{
			TimeoutHours:  24,
			TimeoutAction: approval.TimeoutActionTransferAdmin,
			AdminUserIDs:  []string{"admin-1"},
		}),
		"CCNodeRole": {
			Nodes: []approval.NodeDefinition{
				{ID: "start-1", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{Name: "开始"})},
				{ID: "cc-1", Kind: approval.NodeCC, Data: mustMarshal(approval.CCNodeData{
					CCs: []approval.CCDefinition{{Kind: approval.CCRole, IDs: []string{"role-1"}, Timing: approval.CCTimingAlways}},
				})},
				{ID: "end-1", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{Name: "结束"})},
			},
			Edges: []approval.EdgeDefinition{
				{ID: "edge-1", Source: "start-1", Target: "cc-1"},
				{ID: "edge-2", Source: "cc-1", Target: "end-1"},
			},
		},
	}

	for name, def := range violations {
		s.Run("Rejects"+name, func() {
			s.Require().ErrorIs(deploy(s.shared.FlowID, def), approval.ErrSharedFlowNamesPeople,
				"A shared flow must not route work or notifications to specific people")
		})

		s.Run("TenantFlowAccepts"+name, func() {
			s.Require().NoError(deploy(s.tenantOwn.FlowID, def), "The rule applies to shared flows only")
		})
	}

	s.Run("AcceptsRulesResolvedFromTheInstance", func() {
		s.Require().NoError(deploy(s.shared.FlowID, withTask(approval.TaskNodeData{
			Assignees: []approval.AssigneeDefinition{
				{Kind: approval.AssigneeSuperior, SortOrder: 1},
				{Kind: approval.AssigneeDepartmentLeader, SortOrder: 2},
			},
			EmptyAssigneeAction: approval.EmptyAssigneeTransferSuperior,
			TimeoutHours:        24,
			TimeoutAction:       approval.TimeoutActionNotify,
		})), "Rules resolved from the applicant run in every tenant")
	})
}
