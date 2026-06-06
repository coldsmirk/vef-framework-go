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
	reject     cqrs.Handler[command.RejectTaskCmd, cqrs.Unit]
	transfer   cqrs.Handler[command.TransferTaskCmd, cqrs.Unit]
	remove     cqrs.Handler[command.RemoveAssigneeCmd, cqrs.Unit]
	fixture    *MinimalFixture
	nodeID     string
	seqNodeID  string
	anyNodeID  string

	seq int
}

func (s *DependentActivationTestSuite) SetupSuite() {
	eng := buildTestEngine()
	taskSvc, nodeSvc, validSvc := buildTestServices(eng)

	s.addHandler = wrapWithBusAndDB(s.db, eventtest.NewFakeBus(), command.NewAddAssigneeHandler(s.db, taskSvc, nil))
	s.approve = wrapWithBusAndDB(s.db, eventtest.NewFakeBus(), command.NewApproveTaskHandler(s.db, taskSvc, nodeSvc, validSvc))
	s.reject = wrapWithBusAndDB(s.db, eventtest.NewFakeBus(), command.NewRejectTaskHandler(s.db, taskSvc, nodeSvc, validSvc))
	s.transfer = wrapWithBusAndDB(s.db, eventtest.NewFakeBus(), command.NewTransferTaskHandler(s.db, taskSvc, validSvc, nil))
	s.remove = wrapWithBusAndDB(s.db, eventtest.NewFakeBus(), command.NewRemoveAssigneeHandler(s.db, taskSvc, nodeSvc, eng))
	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "dep-activation")

	// Parallel approval node, all must approve — the configuration that
	// deadlocked when a Waiting add-assignee task lingered uncounted-as-approved.
	// Transfer is allowed so the regression tests can prove a transfer keeps the
	// add-assignee parent/child link intact.
	node := &approval.FlowNode{
		FlowVersionID:        s.fixture.VersionID,
		Key:                  "parallel-all-node",
		Kind:                 approval.NodeApproval,
		Name:                 "Parallel All Node",
		ApprovalMethod:       approval.ApprovalParallel,
		PassRule:             approval.PassAll,
		IsAddAssigneeAllowed: true,
		IsTransferAllowed:    true,
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should create parallel-all node")
	s.nodeID = node.ID

	// Sequential counterpart with the same capabilities, to prove the
	// sort-ordered queue reactivates a before-suspended parent without the
	// explicit parent/child resolution the parallel path needs.
	seqNode := &approval.FlowNode{
		FlowVersionID:           s.fixture.VersionID,
		Key:                     "sequential-all-node",
		Kind:                    approval.NodeApproval,
		Name:                    "Sequential All Node",
		ApprovalMethod:          approval.ApprovalSequential,
		PassRule:                approval.PassAll,
		IsAddAssigneeAllowed:    true,
		IsTransferAllowed:       true,
		IsRemoveAssigneeAllowed: true,
	}
	_, err = s.db.NewInsert().Model(seqNode).Exec(s.ctx)
	s.Require().NoError(err, "Should create sequential-all node")
	s.seqNodeID = seqNode.ID

	// Parallel "any" node, where a single reject leaves the node running so the
	// reject path's dependent activation is observable (under PassAll a reject
	// fails the node outright and cancels everything).
	anyNode := &approval.FlowNode{
		FlowVersionID:        s.fixture.VersionID,
		Key:                  "parallel-any-node",
		Kind:                 approval.NodeApproval,
		Name:                 "Parallel Any Node",
		ApprovalMethod:       approval.ApprovalParallel,
		PassRule:             approval.PassAny,
		IsAddAssigneeAllowed: true,
	}
	_, err = s.db.NewInsert().Model(anyNode).Exec(s.ctx)
	s.Require().NoError(err, "Should create parallel-any node")
	s.anyNodeID = anyNode.ID

	// End node + edges so node completion can advance to a terminal status,
	// proving the absence of a deadlock end-to-end.
	endNode := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "parallel-all-end",
		Kind:          approval.NodeEnd,
		Name:          "End",
	}
	_, err = s.db.NewInsert().Model(endNode).Exec(s.ctx)
	s.Require().NoError(err, "Should create end node")

	for _, src := range []*approval.FlowNode{node, seqNode, anyNode} {
		edge := &approval.FlowEdge{
			FlowVersionID: s.fixture.VersionID,
			Key:           src.Key + "-edge",
			SourceNodeID:  src.ID,
			SourceNodeKey: src.Key,
			TargetNodeID:  endNode.ID,
			TargetNodeKey: endNode.Key,
		}
		_, err = s.db.NewInsert().Model(edge).Exec(s.ctx)
		s.Require().NoError(err, "Should create edge to end node from "+src.Key)
	}
}

func (s *DependentActivationTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *DependentActivationTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

// seedInstance creates a running instance on the parallel-all node with every
// assignee Pending (the parallel start state).
func (s *DependentActivationTestSuite) seedInstance(assignees ...string) *approval.Instance {
	return s.seedInstanceOnNode(s.nodeID, false, assignees...)
}

// seedSequentialInstance creates a running instance on the sequential-all node
// with only the first assignee Pending and the rest Waiting (the sequential
// start state).
func (s *DependentActivationTestSuite) seedSequentialInstance(assignees ...string) *approval.Instance {
	return s.seedInstanceOnNode(s.seqNodeID, true, assignees...)
}

// seedInstanceOnNode creates a running instance on nodeID. When sequential, only
// the first task starts Pending and the rest Waiting; otherwise all are Pending.
func (s *DependentActivationTestSuite) seedInstanceOnNode(nodeID string, sequential bool, assignees ...string) *approval.Instance {
	s.seq++
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Dependent Activation Test",
		InstanceNo:    fmt.Sprintf("DA-%04d", s.seq),
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &nodeID,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should create running instance")

	for i, a := range assignees {
		status := approval.TaskPending
		if sequential && i > 0 {
			status = approval.TaskWaiting
		}

		task := &approval.Task{
			TenantID:   "default",
			InstanceID: inst.ID,
			NodeID:     nodeID,
			AssigneeID: a,
			SortOrder:  i + 1,
			Status:     status,
		}
		_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
		s.Require().NoError(err, "Should create task for "+a)
	}

	return inst
}

func (s *DependentActivationTestSuite) addAssignees(taskID, operatorID string, addType approval.AddAssigneeType, userIDs ...string) {
	_, err := s.addHandler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   taskID,
		UserIDs:  userIDs,
		AddType:  addType,
		Operator: approval.OperatorInfo{ID: operatorID, Name: operatorID},
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "Add-assignee should succeed")
}

func (s *DependentActivationTestSuite) transferTo(taskID, fromUserID, toUserID string) {
	_, err := s.transfer.Handle(s.ctx, command.TransferTaskCmd{
		TaskID:       taskID,
		Operator:     approval.OperatorInfo{ID: fromUserID, Name: fromUserID},
		TransferToID: toUserID,
		Caller:       approval.SystemCaller,
	})
	s.Require().NoError(err, fmt.Sprintf("Transfer from %s to %s should succeed", fromUserID, toUserID))
}

func (s *DependentActivationTestSuite) rejectAs(taskID, userID string) {
	_, err := s.reject.Handle(s.ctx, command.RejectTaskCmd{
		TaskID:   taskID,
		Operator: approval.OperatorInfo{ID: userID, Name: userID},
		Opinion:  "rejected by " + userID,
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "Reject by "+userID+" should not error")
}

func (s *DependentActivationTestSuite) removeAs(taskID, operatorID string) {
	_, err := s.remove.Handle(s.ctx, command.RemoveAssigneeCmd{
		TaskID:   taskID,
		Operator: approval.OperatorInfo{ID: operatorID, Name: operatorID},
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "Remove by "+operatorID+" should not error")
}

func (s *DependentActivationTestSuite) taskStatus(instanceID, assigneeID string) approval.TaskStatus {
	return s.taskFor(instanceID, assigneeID).Status
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

// TestTransferringBeforeChildStillReactivatesParent guards the transfer path
// against re-introducing the parallel-node deadlock: a transferred "before"
// child must keep its parent link so the transferee's approval still
// reactivates the suspended parent.
func (s *DependentActivationTestSuite) TestTransferringBeforeChildStillReactivatesParent() {
	inst := s.seedInstance("orig-A", "peer-B")
	origA := s.taskFor(inst.ID, "orig-A")

	s.addAssignees(origA.ID, "orig-A", approval.AddAssigneeBefore, "before-C")
	s.Require().Equal(approval.TaskWaiting, s.taskStatus(inst.ID, "orig-A"), "Original suspends to Waiting after add-before")

	// Before the fix the replacement dropped ParentTaskID/AddAssigneeType, so
	// the suspended parent was stranded forever once the transferee approved.
	beforeC := s.taskFor(inst.ID, "before-C")
	s.transferTo(beforeC.ID, "before-C", "before-D")
	s.Require().Equal(approval.TaskPending, s.taskStatus(inst.ID, "before-D"), "Transferee should be pending")

	s.approveAs(s.taskFor(inst.ID, "before-D").ID, "before-D")
	s.Assert().Equal(approval.TaskPending, s.taskStatus(inst.ID, "orig-A"),
		"Parent must be reactivated once the transferred before-child approves")

	// No deadlock: the node still drives to completion.
	s.approveAs(s.taskFor(inst.ID, "orig-A").ID, "orig-A")
	s.approveAs(s.taskFor(inst.ID, "peer-B").ID, "peer-B")
	s.Assert().Equal(approval.InstanceApproved, s.instanceStatus(inst.ID),
		"Parallel all node must complete after transferring a before-child")
}

// TestTransferringAfterParentStillActivatesAfterChild guards the symmetric
// case: transferring the parent that "after" children wait on must re-point
// those children onto the stand-in so they are not orphaned.
func (s *DependentActivationTestSuite) TestTransferringAfterParentStillActivatesAfterChild() {
	inst := s.seedInstance("orig-A", "peer-B")
	origA := s.taskFor(inst.ID, "orig-A")

	s.addAssignees(origA.ID, "orig-A", approval.AddAssigneeAfter, "after-C")
	s.Require().Equal(approval.TaskWaiting, s.taskStatus(inst.ID, "after-C"), "After-child starts queued as Waiting")

	// Before the fix the after-child kept pointing at the finished original and
	// was never activated by the transferee's completion.
	s.transferTo(origA.ID, "orig-A", "orig-D")
	s.approveAs(s.taskFor(inst.ID, "orig-D").ID, "orig-D")

	s.Assert().Equal(approval.TaskPending, s.taskStatus(inst.ID, "after-C"),
		"After-child must be activated once the transferee (its new parent) finishes")

	s.approveAs(s.taskFor(inst.ID, "after-C").ID, "after-C")
	s.approveAs(s.taskFor(inst.ID, "peer-B").ID, "peer-B")
	s.Assert().Equal(approval.InstanceApproved, s.instanceStatus(inst.ID),
		"Parallel all node must complete after transferring the parent of an after-child")
}

// TestRejectingBeforeChildReactivatesParent covers the reject path's dependent
// activation: on a node a single reject leaves running (PassAny), rejecting a
// before-child must still reactivate the suspended parent rather than strand it.
func (s *DependentActivationTestSuite) TestRejectingBeforeChildReactivatesParent() {
	inst := s.seedInstanceOnNode(s.anyNodeID, false, "orig-A", "peer-B")
	origA := s.taskFor(inst.ID, "orig-A")

	s.addAssignees(origA.ID, "orig-A", approval.AddAssigneeBefore, "before-C")
	s.Require().Equal(approval.TaskWaiting, s.taskStatus(inst.ID, "orig-A"), "Original suspends to Waiting after add-before")

	s.rejectAs(s.taskFor(inst.ID, "before-C").ID, "before-C")

	s.Assert().Equal(approval.InstanceRunning, s.instanceStatus(inst.ID), "PassAny node stays running after a single reject")
	s.Assert().Equal(approval.TaskPending, s.taskStatus(inst.ID, "orig-A"),
		"Suspended parent must be reactivated after its before-child is rejected, not stranded")

	s.approveAs(s.taskFor(inst.ID, "orig-A").ID, "orig-A")
	s.Assert().Equal(approval.InstanceApproved, s.instanceStatus(inst.ID),
		"One approval drives the PassAny node to completion")
}

// TestSequentialBeforeAddAssigneeReactivatesViaQueue pins the claim that a
// sequential node needs no explicit parent/child resolution: its sort-ordered
// queue alone brings a before-suspended parent back to Pending.
func (s *DependentActivationTestSuite) TestSequentialBeforeAddAssigneeReactivatesViaQueue() {
	inst := s.seedSequentialInstance("orig-A")
	origA := s.taskFor(inst.ID, "orig-A")

	s.addAssignees(origA.ID, "orig-A", approval.AddAssigneeBefore, "before-C")
	s.Require().Equal(approval.TaskWaiting, s.taskStatus(inst.ID, "orig-A"), "Original suspends to Waiting after add-before")

	s.approveAs(s.taskFor(inst.ID, "before-C").ID, "before-C")
	s.Assert().Equal(approval.TaskPending, s.taskStatus(inst.ID, "orig-A"),
		"Sequential queue must reactivate the suspended parent after the before-approver acts")

	s.approveAs(s.taskFor(inst.ID, "orig-A").ID, "orig-A")
	s.Assert().Equal(approval.InstanceApproved, s.instanceStatus(inst.ID),
		"Sequential all node completes through the before-add-assignee path")
}

// TestBeforeParentStaysSuspendedUntilAllBeforeChildrenFinish covers the
// multi-child no-op branch: the parent is reactivated only once every
// before-child has finished, not after the first.
func (s *DependentActivationTestSuite) TestBeforeParentStaysSuspendedUntilAllBeforeChildrenFinish() {
	inst := s.seedInstance("orig-A", "peer-B")
	origA := s.taskFor(inst.ID, "orig-A")

	s.addAssignees(origA.ID, "orig-A", approval.AddAssigneeBefore, "before-C", "before-D")
	s.Require().Equal(approval.TaskWaiting, s.taskStatus(inst.ID, "orig-A"), "Original suspends to Waiting after add-before")

	s.approveAs(s.taskFor(inst.ID, "before-C").ID, "before-C")
	s.Assert().Equal(approval.TaskWaiting, s.taskStatus(inst.ID, "orig-A"),
		"Parent must stay suspended while a second before-child is still active")

	s.approveAs(s.taskFor(inst.ID, "before-D").ID, "before-D")
	s.Assert().Equal(approval.TaskPending, s.taskStatus(inst.ID, "orig-A"),
		"Parent must be reactivated only once all before-children have finished")
}

// TestSequentialActivationGuardKeepsSingleTaskActive covers the pending-exists
// guard in ActivateNextSequentialTask: removing a queued task while another is
// still Pending must not promote a third task, so the sequential node never has
// two tasks active at once.
func (s *DependentActivationTestSuite) TestSequentialActivationGuardKeepsSingleTaskActive() {
	inst := s.seedSequentialInstance("seq-A", "seq-B", "seq-C")
	s.Require().Equal(approval.TaskPending, s.taskStatus(inst.ID, "seq-A"), "First sequential task starts Pending")
	s.Require().Equal(approval.TaskWaiting, s.taskStatus(inst.ID, "seq-C"), "Third sequential task starts Waiting")

	// Remove the middle queued task while seq-A is still Pending. Removal calls
	// ActivateDependentTasks; the guard must keep it a no-op so seq-C is not
	// promoted ahead of seq-A.
	s.removeAs(s.taskFor(inst.ID, "seq-B").ID, "seq-A")

	s.Assert().Equal(approval.TaskRemoved, s.taskStatus(inst.ID, "seq-B"), "Removed task should be Removed")
	s.Assert().Equal(approval.TaskPending, s.taskStatus(inst.ID, "seq-A"), "The active task stays Pending")
	s.Assert().Equal(approval.TaskWaiting, s.taskStatus(inst.ID, "seq-C"),
		"The guard must keep the next queued task Waiting while one is already Pending")
}
