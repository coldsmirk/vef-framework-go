package command_test

import (
	"context"
	"fmt"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &DependentActivationTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// DependentActivationTestSuite exercises add-assignee dependency activation on a
// parallel node (the F2 fix). On a parallel all/ratio node the old code never
// reactivated a "before"-suspended original (and never activated an "after"
// child), leaving the node deadlocked at PassRulePending. The fix drives those
// transitions via the parent/child link.
type DependentActivationTestSuite struct {
	suite.Suite

	ctx        context.Context
	db         orm.DB
	addHandler cqrs.Handler[command.AddAssigneeCmd, cqrs.Unit]
	approve    cqrs.Handler[command.ApproveTaskCmd, cqrs.Unit]
	fixture    *MinimalFixture
	nodeID     string

	seq int
}

func (s *DependentActivationTestSuite) SetupSuite() {
	eng := buildTestEngine()
	taskSvc, nodeSvc, validSvc := buildTestServices(eng)

	s.addHandler = wrapWithBusAndDB(s.db, eventtest.NewFakeBus(), command.NewAddAssigneeHandler(s.db, taskSvc, nil))
	s.approve = wrapWithBusAndDB(s.db, eventtest.NewFakeBus(), command.NewApproveTaskHandler(s.db, taskSvc, nodeSvc, validSvc))
	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "dep-activation")

	// Parallel approval node, all must approve — the configuration that
	// deadlocked when a Waiting add-assignee task lingered uncounted-as-approved.
	node := &approval.FlowNode{
		FlowVersionID:        s.fixture.VersionID,
		Key:                  "parallel-all-node",
		Kind:                 approval.NodeApproval,
		Name:                 "Parallel All Node",
		ApprovalMethod:       approval.ApprovalParallel,
		PassRule:             approval.PassAll,
		IsAddAssigneeAllowed: true,
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should create parallel-all node")
	s.nodeID = node.ID

	// End node + edge so node completion can advance to a terminal status,
	// proving the absence of a deadlock end-to-end.
	endNode := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "parallel-all-end",
		Kind:          approval.NodeEnd,
		Name:          "End",
	}
	_, err = s.db.NewInsert().Model(endNode).Exec(s.ctx)
	s.Require().NoError(err, "Should create end node")

	edge := &approval.FlowEdge{
		FlowVersionID: s.fixture.VersionID,
		Key:           "parallel-all-edge",
		SourceNodeID:  node.ID,
		SourceNodeKey: node.Key,
		TargetNodeID:  endNode.ID,
		TargetNodeKey: endNode.Key,
	}
	_, err = s.db.NewInsert().Model(edge).Exec(s.ctx)
	s.Require().NoError(err, "Should create edge to end node")
}

func (s *DependentActivationTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *DependentActivationTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *DependentActivationTestSuite) seedInstance(assignees ...string) *approval.Instance {
	s.seq++
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Dependent Activation Test",
		InstanceNo:    fmt.Sprintf("DA-%04d", s.seq),
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &s.nodeID,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should create running instance")

	for i, a := range assignees {
		task := &approval.Task{
			TenantID:   "default",
			InstanceID: inst.ID,
			NodeID:     s.nodeID,
			AssigneeID: a,
			SortOrder:  i + 1,
			Status:     approval.TaskPending,
		}
		_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
		s.Require().NoError(err, "Should create pending task for "+a)
	}

	return inst
}

func (s *DependentActivationTestSuite) taskFor(instanceID, assigneeID string) approval.Task {
	var t approval.Task

	s.Require().NoError(
		s.db.NewSelect().Model(&t).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("instance_id", instanceID).Equals("assignee_id", assigneeID)
			}).
			Limit(1).
			Scan(s.ctx),
		"Should load task for assignee "+assigneeID,
	)

	return t
}

func (s *DependentActivationTestSuite) approveAs(taskID, userID string) {
	_, err := s.approve.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   taskID,
		Operator: approval.OperatorInfo{ID: userID, Name: userID},
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "Approve by "+userID+" should not error")
}

func (s *DependentActivationTestSuite) instanceStatus(instanceID string) approval.InstanceStatus {
	var inst approval.Instance

	inst.ID = instanceID
	s.Require().NoError(s.db.NewSelect().Model(&inst).WherePK().Scan(s.ctx), "Should reload instance")

	return inst.Status
}

// TestBeforeAddAssigneeOnParallelNodeReactivatesOriginal is the F2 regression
// test: before the fix the suspended original was never reactivated on a
// parallel node, so the node stalled at PassRulePending forever.
func (s *DependentActivationTestSuite) TestBeforeAddAssigneeOnParallelNodeReactivatesOriginal() {
	inst := s.seedInstance("orig-A", "peer-B")
	origA := s.taskFor(inst.ID, "orig-A")

	_, err := s.addHandler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   origA.ID,
		UserIDs:  []string{"before-C"},
		AddType:  approval.AddAssigneeBefore,
		Operator: approval.OperatorInfo{ID: "orig-A", Name: "orig-A"},
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "Add-before should succeed")

	origA = s.taskFor(inst.ID, "orig-A")
	s.Require().Equal(approval.TaskWaiting, origA.Status, "Original should be suspended to Waiting after add-before")

	// The pre-approver acts → the original must come back to Pending.
	beforeC := s.taskFor(inst.ID, "before-C")
	s.approveAs(beforeC.ID, "before-C")

	origA = s.taskFor(inst.ID, "orig-A")
	s.Assert().Equal(approval.TaskPending, origA.Status, "Original must be reactivated to Pending once the before-approver acts")
	s.Assert().Equal(approval.InstanceRunning, s.instanceStatus(inst.ID), "Instance should still be running before the rest approve")

	// And the node can actually be driven to completion — no deadlock.
	s.approveAs(origA.ID, "orig-A")
	s.approveAs(s.taskFor(inst.ID, "peer-B").ID, "peer-B")

	s.Assert().Equal(approval.InstanceApproved, s.instanceStatus(inst.ID),
		"Parallel all node must complete after the before-add-assignee path instead of deadlocking")
}

// TestAfterAddAssigneeOnParallelNodeActivatesChild proves the complementary
// "after" path: a queued after-child becomes actionable once its parent acts.
func (s *DependentActivationTestSuite) TestAfterAddAssigneeOnParallelNodeActivatesChild() {
	inst := s.seedInstance("orig-A", "peer-B")
	origA := s.taskFor(inst.ID, "orig-A")

	_, err := s.addHandler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   origA.ID,
		UserIDs:  []string{"after-C"},
		AddType:  approval.AddAssigneeAfter,
		Operator: approval.OperatorInfo{ID: "orig-A", Name: "orig-A"},
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "Add-after should succeed")

	afterC := s.taskFor(inst.ID, "after-C")
	s.Require().Equal(approval.TaskWaiting, afterC.Status, "After-child starts queued as Waiting")

	// Parent acts → after-child must become actionable.
	s.approveAs(origA.ID, "orig-A")

	afterC = s.taskFor(inst.ID, "after-C")
	s.Assert().Equal(approval.TaskPending, afterC.Status, "After-child must be activated once its parent finishes")

	// Drive to completion to confirm no deadlock.
	s.approveAs(afterC.ID, "after-C")
	s.approveAs(s.taskFor(inst.ID, "peer-B").ID, "peer-B")

	s.Assert().Equal(approval.InstanceApproved, s.instanceStatus(inst.ID),
		"Parallel all node must complete after the after-add-assignee path instead of deadlocking")
}
