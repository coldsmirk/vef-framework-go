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
		return &NodeVisitTrailTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// NodeVisitTrailTestSuite drives full lifecycles through the real engine and
// asserts the visit trail it records — the authoritative source for the
// timeline and flow-graph projections.
type NodeVisitTrailTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	fixture *FlowFixture

	start     cqrs.Handler[command.StartInstanceCmd, *approval.Instance]
	approve   cqrs.Handler[command.ApproveTaskCmd, cqrs.Unit]
	reject    cqrs.Handler[command.RejectTaskCmd, cqrs.Unit]
	rollback  cqrs.Handler[command.RollbackTaskCmd, cqrs.Unit]
	resubmit  cqrs.Handler[command.ResubmitInstanceCmd, cqrs.Unit]
	withdraw  cqrs.Handler[command.WithdrawInstanceCmd, cqrs.Unit]
	terminate cqrs.Handler[command.TerminateInstanceCmd, cqrs.Unit]
}

func (s *NodeVisitTrailTestSuite) SetupSuite() {
	s.fixture = setupApprovalFlow(s.T(), s.ctx, s.db)

	eng := buildTestEngine(s.db)
	taskSvc, nodeSvc, validSvc := buildTestServices(eng)
	instanceSvc := service.NewInstanceService(nil)
	bus := eventtest.NewFakeBus()

	s.start = wrapWithBusAndDB(
		s.db,
		bus,
		command.NewStartInstanceHandler(
			s.db,
			eng,
			&MockInstanceNoGenerator{},
			validSvc,
			nil,
			binding.NewProjector(binding.NewIdentityResolver(), binding.NewWriter(), nil),
			nil,
		),
	)
	s.approve = wrapWithBusAndDB(s.db, bus, command.NewApproveTaskHandler(s.db, taskSvc, nodeSvc, validSvc, nil))
	s.reject = wrapWithBusAndDB(s.db, bus, command.NewRejectTaskHandler(s.db, taskSvc, nodeSvc, validSvc, nil))
	s.rollback = wrapWithBusAndDB(s.db, bus, command.NewRollbackTaskHandler(s.db, taskSvc, instanceSvc, validSvc, eng, nil))
	s.resubmit = wrapWithBusAndDB(s.db, bus, command.NewResubmitInstanceHandler(s.db, eng, validSvc, instanceSvc, nil, nil))
	s.withdraw = wrapWithBusAndDB(s.db, bus, command.NewWithdrawInstanceHandler(s.db, taskSvc, instanceSvc))
	s.terminate = wrapWithBusAndDB(s.db, bus, command.NewTerminateInstanceHandler(s.db, taskSvc, instanceSvc))
}

func (s *NodeVisitTrailTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *NodeVisitTrailTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

// startInstance runs the real start command (engine traversal included) and
// returns the created instance.
func (s *NodeVisitTrailTestSuite) startInstance() *approval.Instance {
	instance, err := s.start.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode:  "apv-cmd-test-flow",
		Applicant: approval.UserInfo{ID: "applicant-1", Name: "Applicant"},
		FormData:  map[string]any{"reason": "trail"},
		Caller:    approval.SystemCaller,
	})
	s.Require().NoError(err, "Should start instance through the engine")

	return instance
}

// loadVisits returns the instance's visit trail in sequence order.
func (s *NodeVisitTrailTestSuite) loadVisits(instanceID string) []approval.NodeVisit {
	var visits []approval.NodeVisit

	s.Require().NoError(s.db.NewSelect().Model(&visits).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instanceID) }).
		OrderBy("sequence").
		Scan(s.ctx), "Should load node visits")

	return visits
}

// pendingTask returns the pending task of the given assignee on the instance.
func (s *NodeVisitTrailTestSuite) pendingTask(instanceID, assigneeID string) *approval.Task {
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

// assertVisit asserts one trail entry's node, sequence, status, and — for
// concluded visits — that the finish time is stamped.
func (s *NodeVisitTrailTestSuite) assertVisit(v approval.NodeVisit, nodeKey string, sequence int, status approval.NodeVisitStatus) {
	s.Assert().Equal(s.fixture.NodeIDs[nodeKey], v.NodeID, "Visit %d should be on node %s", sequence, nodeKey)
	s.Assert().Equal(sequence, v.Sequence, "Visit sequence should be %d", sequence)
	s.Assert().Equal(status, v.Status, "Visit %d status should be %s", sequence, status)

	if status == approval.NodeVisitActive {
		s.Assert().Nil(v.FinishedAt, "Open visit must not carry a finish time")
	} else {
		s.Assert().NotNil(v.FinishedAt, "Concluded visit must carry a finish time")
	}
}

func (s *NodeVisitTrailTestSuite) TestStartRecordsTrail() {
	instance := s.startInstance()

	visits := s.loadVisits(instance.ID)
	s.Require().Len(visits, 2, "Start should record the start visit and the open approval visit")
	s.assertVisit(visits[0], "start-1", 1, approval.NodeVisitPassed)
	s.assertVisit(visits[1], "approval-1", 2, approval.NodeVisitActive)

	// Every task created during the visit — the actionable first approver and
	// the queued second one — binds to it.
	var tasks []approval.Task

	s.Require().NoError(s.db.NewSelect().Model(&tasks).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", instance.ID) }).
		Scan(s.ctx), "Should load tasks")
	s.Require().Len(tasks, 2, "Sequential node should create both tasks up front")

	for _, task := range tasks {
		s.Assert().Equal(visits[1].ID, task.VisitID, "Task should bind to the visit that created it")
	}
}

func (s *NodeVisitTrailTestSuite) TestApprovalCompletionConcludesTrail() {
	instance := s.startInstance()

	for _, userID := range []string{"user-1", "user-2"} {
		task := s.pendingTask(instance.ID, userID)
		_, err := s.approve.Handle(s.ctx, command.ApproveTaskCmd{
			TaskID:   task.ID,
			Operator: approval.UserInfo{ID: userID, Name: userID},
			Opinion:  "ok",
			Caller:   approval.SystemCaller,
		})
		s.Require().NoError(err, "%s should approve", userID)
	}

	visits := s.loadVisits(instance.ID)
	s.Require().Len(visits, 3, "Completion should extend the trail through the end node")
	s.assertVisit(visits[0], "start-1", 1, approval.NodeVisitPassed)
	s.assertVisit(visits[1], "approval-1", 2, approval.NodeVisitPassed)
	s.assertVisit(visits[2], "end-1", 3, approval.NodeVisitPassed)
}

func (s *NodeVisitTrailTestSuite) TestRejectConcludesRejected() {
	instance := s.startInstance()

	task := s.pendingTask(instance.ID, "user-1")
	_, err := s.reject.Handle(s.ctx, command.RejectTaskCmd{
		TaskID:   task.ID,
		Operator: approval.UserInfo{ID: "user-1", Name: "user-1"},
		Opinion:  "no",
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "Should reject task")

	visits := s.loadVisits(instance.ID)
	s.Require().Len(visits, 2, "Rejection ends the trail at the deciding node")
	s.assertVisit(visits[1], "approval-1", 2, approval.NodeVisitRejected)
}

func (s *NodeVisitTrailTestSuite) TestRollbackReturnsAndResubmitReopens() {
	instance := s.startInstance()

	task := s.pendingTask(instance.ID, "user-1")
	_, err := s.rollback.Handle(s.ctx, command.RollbackTaskCmd{
		TaskID:       task.ID,
		Operator:     approval.UserInfo{ID: "user-1", Name: "user-1"},
		Opinion:      "back to applicant",
		TargetNodeID: s.fixture.NodeIDs["start-1"],
		Caller:       approval.SystemCaller,
	})
	s.Require().NoError(err, "Should roll back to the start node")

	visits := s.loadVisits(instance.ID)
	s.Require().Len(visits, 2, "Rollback to start pauses the instance without opening a new visit")
	s.assertVisit(visits[1], "approval-1", 2, approval.NodeVisitReturned)

	_, err = s.resubmit.Handle(s.ctx, command.ResubmitInstanceCmd{
		InstanceID: instance.ID,
		Operator:   approval.UserInfo{ID: "applicant-1", Name: "Applicant"},
		FormData:   map[string]any{"reason": "fixed"},
		Caller:     approval.SystemCaller,
	})
	s.Require().NoError(err, "Applicant should resubmit the returned instance")

	visits = s.loadVisits(instance.ID)
	s.Require().Len(visits, 4, "Resubmit should re-traverse from the start node")
	s.assertVisit(visits[2], "start-1", 3, approval.NodeVisitPassed)
	s.assertVisit(visits[3], "approval-1", 4, approval.NodeVisitActive)
}

func (s *NodeVisitTrailTestSuite) TestWithdrawCancelsOpenVisits() {
	instance := s.startInstance()

	_, err := s.withdraw.Handle(s.ctx, command.WithdrawInstanceCmd{
		InstanceID: instance.ID,
		Operator:   approval.UserInfo{ID: "applicant-1", Name: "Applicant"},
		Reason:     "changed my mind",
		Caller:     approval.SystemCaller,
	})
	s.Require().NoError(err, "Applicant should withdraw the running instance")

	visits := s.loadVisits(instance.ID)
	s.Require().Len(visits, 2, "Withdraw does not extend the trail")
	s.assertVisit(visits[0], "start-1", 1, approval.NodeVisitPassed)
	s.assertVisit(visits[1], "approval-1", 2, approval.NodeVisitCanceled)
}

func (s *NodeVisitTrailTestSuite) TestTerminateCancelsOpenVisits() {
	instance := s.startInstance()

	_, err := s.terminate.Handle(s.ctx, command.TerminateInstanceCmd{
		InstanceID: instance.ID,
		Operator:   approval.UserInfo{ID: "admin-1", Name: "Admin"},
		Reason:     "force close",
		Caller:     approval.SystemCaller,
	})
	s.Require().NoError(err, "Admin should terminate the running instance")

	visits := s.loadVisits(instance.ID)
	s.Require().Len(visits, 2, "Terminate does not extend the trail")
	s.assertVisit(visits[1], "approval-1", 2, approval.NodeVisitCanceled)
}

func (s *NodeVisitTrailTestSuite) startTwoStepFlow(code string, a, b approval.ApprovalNodeData) (*FlowFixture, *approval.Instance) {
	def := approval.FlowDefinition{
		Nodes: []approval.NodeDefinition{
			{ID: "start", Kind: approval.NodeStart, Data: mustMarshal(approval.StartNodeData{Name: "Start"})},
			{ID: "a", Kind: approval.NodeApproval, Data: mustMarshal(a)},
			{ID: "b", Kind: approval.NodeApproval, Data: mustMarshal(b)},
			{ID: "end", Kind: approval.NodeEnd, Data: mustMarshal(approval.EndNodeData{Name: "End"})},
		},
		Edges: []approval.EdgeDefinition{
			{ID: "sa", Source: "start", Target: "a"},
			{ID: "ab", Source: "a", Target: "b"},
			{ID: "be", Source: "b", Target: "end"},
		},
	}
	flow := deployAndPublishFlow(s.T(), s.ctx, s.db, code, def)
	instance, err := s.start.Handle(s.ctx, command.StartInstanceCmd{
		FlowCode: code + "-flow", Applicant: approval.UserInfo{ID: "applicant"}, Caller: approval.SystemCaller,
	})
	s.Require().NoError(err, "Should start the two-step flow")

	return flow, instance
}

func (s *NodeVisitTrailTestSuite) approvePending(instanceID, user string) {
	task := s.pendingTask(instanceID, user)
	_, err := s.approve.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID: task.ID, Operator: task.Assignee(), Caller: approval.SystemCaller,
	})
	s.Require().NoError(err, "The pending seat of %s should approve", user)
}

func (s *NodeVisitTrailTestSuite) rollBackPending(instanceID, user, targetID string) {
	task := s.pendingTask(instanceID, user)
	_, err := s.rollback.Handle(s.ctx, command.RollbackTaskCmd{
		TaskID: task.ID, Operator: task.Assignee(), TargetNodeID: targetID, Caller: approval.SystemCaller,
	})
	s.Require().NoError(err, "The pending seat of %s should roll back", user)
}

func (s *NodeVisitTrailTestSuite) TestEntryAutoPassCountsCurrentVisit() {
	flow, instance := s.startTwoStepFlow("visit-ratio", approval.ApprovalNodeData{
		Name: "A", Assignees: []approval.AssigneeDefinition{{Kind: approval.AssigneeUser, IDs: []string{"applicant", "x", "y"}}},
		PassRule: approval.PassRatio, PassRatio: decimal.NewFromInt(60), SameApplicantAction: approval.SameApplicantAutoPass,
	}, approval.ApprovalNodeData{
		Name: "B", Assignees: []approval.AssigneeDefinition{{Kind: approval.AssigneeUser, IDs: []string{"z"}}},
	})

	s.approvePending(instance.ID, "x")
	s.rollBackPending(instance.ID, "z", flow.NodeIDs["a"])
	visits := s.loadVisits(instance.ID)
	s.Require().Len(visits, 4, "One auto-pass out of three on A's second visit must leave it open")
	s.Equal(approval.NodeVisitPassed, visits[1].Status, "Two approvals passed A's first visit")
	s.Equal(approval.NodeVisitActive, visits[3].Status, "Historical approvals must not meet the new visit's threshold")
	s.Equal(visits[3].ID, s.pendingTask(instance.ID, "x").VisitID, "The prior approver must decide again")
	s.approvePending(instance.ID, "y")
	s.Equal(flow.NodeIDs["b"], s.pendingTask(instance.ID, "z").NodeID, "Two current approvals should advance")
}

func (s *NodeVisitTrailTestSuite) TestConsecutiveApproversFollowVisitTrail() {
	for _, tc := range []struct{ name, target string }{
		{"Rollback", "a"},
		{"Resubmit", "start"},
	} {
		s.Run(tc.name, func() {
			flow, instance := s.startTwoStepFlow("consecutive-"+tc.target, approval.ApprovalNodeData{
				Name: "A", Assignees: []approval.AssigneeDefinition{{Kind: approval.AssigneeUser, IDs: []string{"x", "y"}}},
				PassRule: approval.PassAny, ConsecutiveApproverAction: approval.ConsecutiveApproverAutoPass,
			}, approval.ApprovalNodeData{
				Name: "B", Assignees: []approval.AssigneeDefinition{{Kind: approval.AssigneeUser, IDs: []string{"x", "y", "z"}}},
				PassRule: approval.PassAll, ConsecutiveApproverAction: approval.ConsecutiveApproverAutoPass, RollbackType: approval.RollbackAny,
			})
			s.approvePending(instance.ID, "x")
			s.approvePending(instance.ID, "y")
			s.rollBackPending(instance.ID, "z", flow.NodeIDs[tc.target])

			if tc.target == "start" {
				_, err := s.resubmit.Handle(s.ctx, command.ResubmitInstanceCmd{
					InstanceID: instance.ID, Operator: instance.Applicant(), Caller: approval.SystemCaller,
				})
				s.Require().NoError(err, "Applicant should re-enter the flow")
			}

			visits := s.loadVisits(instance.ID)
			s.Equal(approval.NodeVisitReturned, visits[2].Status, "B's partial approvals belong to a returned visit")
			current := visits[len(visits)-1]
			s.Require().Equal(flow.NodeIDs["a"], current.NodeID, "A should await a fresh decision after the return")
			s.Equal(current.ID, s.pendingTask(instance.ID, "x").VisitID, "The previous automatic approval must not carry back")
			s.Equal(current.ID, s.pendingTask(instance.ID, "y").VisitID, "The previous manual approval must not carry back")

			s.approvePending(instance.ID, "y")
			s.Equal(flow.NodeIDs["b"], s.pendingTask(instance.ID, "x").NodeID, "X approved only an older visit to A")
			s.approvePending(instance.ID, "x")
			s.approvePending(instance.ID, "z")
			visits = s.loadVisits(instance.ID)
			s.Equal(flow.NodeIDs["end"], visits[len(visits)-1].NodeID, "Y's approval on A's latest visit should auto-pass its seat on B")
		})
	}
}
