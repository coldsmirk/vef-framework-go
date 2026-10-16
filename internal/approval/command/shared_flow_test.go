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

// routedFlowDef routes its one approval to a named user — a person of whatever
// tenant that user belongs to, which a flow spanning tenants may do.
func routedFlowDef(userID string) approval.FlowDefinition {
	return singleApprovalFlowDef(approval.ApprovalNodeData{
		Assignees: []approval.AssigneeDefinition{{Kind: approval.AssigneeUser, IDs: []string{userID}, SortOrder: 1}},
	})
}

// SharedFlowTestSuite covers a shared flow end to end: the tenant its instance
// runs in, who may act on that instance — the starting tenant, and the people of
// other tenants its nodes route work to — how a tenant flow of the same code
// replaces it, how the starting tenant is resolved, and that its nodes may name
// people of any tenant. The shared flows are owned by sharedOwnerTenant.
type SharedFlowTestSuite struct {
	suite.Suite

	ctx       context.Context
	db        orm.DB
	numbering *MockInstanceNoGenerator
	start     cqrs.Handler[command.StartInstanceCmd, *approval.Instance]
	approve   cqrs.Handler[command.ApproveTaskCmd, cqrs.Unit]
	deploy    *command.DeployFlowHandler
	reassign  cqrs.Handler[command.ReassignTaskCmd, cqrs.Unit]

	shared         *FlowFixture
	routed         *FlowFixture
	sharedReplaced *FlowFixture
	tenantOwn      *FlowFixture
	ownerOnly      *FlowFixture
}

// sharedOwnerTenant owns the suite's shared flows.
const sharedOwnerTenant = "tenant-owner"

func (s *SharedFlowTestSuite) SetupSuite() {
	s.shared = deployAndPublishFlowIn(s.T(), s.ctx, s.db, sharedOwnerTenant, true, "shr", sharedSafeFlowDef())
	// The default tenant replaces this shared flow with one of the same code.
	s.sharedReplaced = deployAndPublishFlowIn(s.T(), s.ctx, s.db, sharedOwnerTenant, true, "shr-rep", sharedSafeFlowDef())
	s.tenantOwn = deployAndPublishFlowIn(s.T(), s.ctx, s.db, approval.DefaultTenantID, false, "shr-rep", sharedSafeFlowDef())
	// A flow routing its approval to user-b, who belongs to some other tenant.
	s.routed = deployAndPublishFlowIn(s.T(), s.ctx, s.db, sharedOwnerTenant, true, "rtd", routedFlowDef("user-b"))
	// A flow the owning tenant keeps to itself.
	s.ownerOnly = deployAndPublishFlowIn(s.T(), s.ctx, s.db, sharedOwnerTenant, false, "own", sharedSafeFlowDef())

	eng := buildTestEngine(s.db)
	taskSvc, nodeSvc, validSvc := buildTestServices(eng)

	s.numbering = new(MockInstanceNoGenerator)
	s.start = wrapWithBusAndDB(s.db, eventtest.NewFakeBus(),
		command.NewStartInstanceHandler(s.db, eng, s.numbering, validSvc, binding.NewNoopRefProvider(), nil, nil))
	s.approve = wrapWithBusAndDB(s.db, eventtest.NewFakeBus(),
		command.NewApproveTaskHandler(s.db, taskSvc, nodeSvc, validSvc, nil))
	s.deploy = command.NewDeployFlowHandler(s.db, service.NewFlowDefinitionService(), formeditor.NewParser())
	s.reassign = wrapWithBusAndDB(s.db, eventtest.NewFakeBus(), command.NewReassignTaskHandler(s.db, taskSvc, nil))
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
		"The instance belongs to the starting tenant, never to the one owning the flow")
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
		Operator: approval.UserInfo{ID: "user-z", Name: "user-z"},
		Caller:   approval.CallerContext{TenantID: "tenant-b"},
	}

	_, err = s.approve.Handle(s.ctx, cmd)
	s.Require().ErrorIs(err, approval.ErrTaskNotFound,
		"Another tenant's people the work was not routed to must not reach an instance of a shared flow")

	cmd.Operator = approval.UserInfo{ID: "user-a", Name: "user-a"}
	cmd.Caller = approval.CallerContext{TenantID: "tenant-a"}
	_, err = s.approve.Handle(s.ctx, cmd)
	s.Require().NoError(err, "The starting tenant acts on its instance like on any of its own")

	var stored approval.Instance

	stored.ID = instance.ID
	s.Require().NoError(s.db.NewSelect().Model(&stored).WherePK().Scan(s.ctx), "Should reload the instance")
	s.Assert().Equal(approval.InstanceApproved, stored.Status, "The single approval should complete the instance")
}

// TestRoutedUserActsFromAnotherTenant pins the cross-tenant hand-off a shared
// flow exists for: tenant-a's instance routes its approval to user-b of
// tenant-b, who decides it from tenant-b — while nobody else of tenant-b
// reaches the instance and an administrative operation stays within the
// instance's tenant.
func (s *SharedFlowTestSuite) TestRoutedUserActsFromAnotherTenant() {
	instance, err := s.startAs(approval.CallerContext{TenantID: "tenant-a"}, "", "rtd-flow", "user-a")
	s.Require().NoError(err, "Should start the flow routing to user-b")

	tasks := s.tasksOf(instance.ID)
	s.Require().Len(tasks, 1, "The user rule opens one task")
	s.Require().Equal("user-b", tasks[0].AssigneeID, "The task goes to the named user")
	s.Require().Equal("tenant-a", tasks[0].TenantID, "The task stays in the instance's tenant")

	userB := approval.UserInfo{ID: "user-b", Name: "user-b"}

	s.Run("RejectsOthersOfTheTenant", func() {
		_, err := s.approve.Handle(s.ctx, command.ApproveTaskCmd{
			TaskID:   tasks[0].ID,
			Operator: approval.UserInfo{ID: "user-c", Name: "user-c"},
			Caller:   approval.CallerContext{TenantID: "tenant-b"},
		})
		s.Require().ErrorIs(err, approval.ErrTaskNotFound, "Only the people the work was routed to cross the tenant guard")
	})

	s.Run("RejectsACallerWithoutTenant", func() {
		_, err := s.approve.Handle(s.ctx, command.ApproveTaskCmd{
			TaskID:   tasks[0].ID,
			Operator: userB,
			Caller:   approval.CallerContext{},
		})
		s.Require().ErrorIs(err, approval.ErrTaskNotFound, "A zero caller stays denied even for the assignee")
	})

	s.Run("KeepsAdministrationWithinTheTenant", func() {
		_, err := s.reassign.Handle(s.ctx, command.ReassignTaskCmd{
			TaskID:        tasks[0].ID,
			NewAssigneeID: "user-d",
			Operator:      userB,
			Caller:        approval.CallerContext{TenantID: "tenant-b"},
		})
		s.Require().ErrorIs(err, approval.ErrTaskNotFound, "Reassigning is the instance's tenant's, even for the assignee")
	})

	s.Run("AcceptsTheAssigneeOfAnotherTenant", func() {
		_, err := s.approve.Handle(s.ctx, command.ApproveTaskCmd{
			TaskID:   tasks[0].ID,
			Operator: userB,
			Caller:   approval.CallerContext{TenantID: "tenant-b"},
		})
		s.Require().NoError(err, "The assignee decides the task from the tenant they act in")

		var stored approval.Instance

		stored.ID = instance.ID
		s.Require().NoError(s.db.NewSelect().Model(&stored).WherePK().Scan(s.ctx), "Should reload the instance")
		s.Assert().Equal(approval.InstanceApproved, stored.Status, "The single approval should complete the instance")
		s.Assert().Equal("tenant-a", stored.TenantID, "The instance stays in the tenant that started it")
	})
}

func (s *SharedFlowTestSuite) TestTenantFlowReplacesSharedFlow() {
	own, err := s.startAs(approval.CallerContext{TenantID: approval.DefaultTenantID}, "", "shr-rep-flow", "user-d")
	s.Require().NoError(err, "The default tenant should start its own flow of the code")
	s.Assert().Equal(s.tenantOwn.FlowID, own.FlowID, "A tenant flow takes the shared flow's place in its tenant")

	shared, err := s.startAs(approval.CallerContext{TenantID: "tenant-b"}, "", "shr-rep-flow", "user-b")
	s.Require().NoError(err, "Another tenant should still start the shared flow")
	s.Assert().Equal(s.sharedReplaced.FlowID, shared.FlowID, "Only the replacing tenant loses the shared flow")
}

func (s *SharedFlowTestSuite) TestUnsharedFlowStaysInItsTenant() {
	_, err := s.startAs(approval.CallerContext{TenantID: "tenant-a"}, "", "own-flow", "user-a")
	s.Require().ErrorIs(err, approval.ErrFlowNotFound, "Another tenant must not start a flow its owner did not share")

	instance, err := s.startAs(approval.CallerContext{TenantID: sharedOwnerTenant}, "", "own-flow", "user-o")
	s.Require().NoError(err, "The owning tenant should start its own flow")
	s.Assert().Equal(s.ownerOnly.FlowID, instance.FlowID, "The owner's flow should run")
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

	s.Run("OwnerStartsItsSharedFlow", func() {
		instance, err := s.startAs(approval.CallerContext{TenantID: sharedOwnerTenant}, "", "shr-flow", "user-o")
		s.Require().NoError(err, "The owning tenant should start its shared flow")
		s.Assert().Equal(s.shared.FlowID, instance.FlowID, "The owner's shared flow is one of its own flows")
		s.Assert().Equal(sharedOwnerTenant, instance.TenantID, "The instance runs in the owning tenant")
	})

	s.Run("FailsClosedWithoutTenant", func() {
		_, err := s.startAs(approval.CallerContext{}, "", "shr-flow", "user-z")
		s.Require().ErrorIs(err, approval.ErrFlowNotFound, "A caller without a tenant starts nothing")
	})
}

// TestDeployAcceptsPeopleOfAnyTenant pins that a shared flow is deployed like
// any other: it may name specific users, roles and departments, copy specific
// people and hand work to node admins or fallback users, since a flow spanning
// tenants routes to people of other tenants than the one that starts it.
func (s *SharedFlowTestSuite) TestDeployAcceptsPeopleOfAnyTenant() {
	withTask := func(task approval.TaskNodeData) approval.FlowDefinition {
		if len(task.Assignees) == 0 {
			task.Assignees = []approval.AssigneeDefinition{{Kind: approval.AssigneeSelf, SortOrder: 1}}
		}

		return singleApprovalFlowDef(approval.ApprovalNodeData{TaskNodeData: task})
	}

	definitions := map[string]approval.FlowDefinition{
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
	}

	for name, def := range definitions {
		s.Run(name, func() {
			_, err := s.deploy.Handle(s.ctx, command.DeployFlowCmd{FlowID: s.shared.FlowID, FlowDefinition: def})
			s.Require().NoError(err, "A shared flow may route work to specific people")
		})
	}
}
