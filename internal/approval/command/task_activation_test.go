package command_test

import (
	"context"
	"fmt"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &TaskActivationTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// TaskActivationTestSuite pins TaskActivatedEvent as the single answer to
// "whose turn is it now". A to-do notification subscribes to it alone, so the
// event must arrive for exactly the assignee who can act — never for a
// sequential approver still queued behind a predecessor.
type TaskActivationTestSuite struct {
	suite.Suite

	ctx      context.Context
	db       orm.DB
	bus      *eventtest.FakeBus
	approve  cqrs.Handler[command.ApproveTaskCmd, cqrs.Unit]
	transfer cqrs.Handler[command.TransferTaskCmd, cqrs.Unit]
	reassign cqrs.Handler[command.ReassignTaskCmd, cqrs.Unit]
	fixture  *MinimalFixture
	nodeID   string

	seq int
}

func (s *TaskActivationTestSuite) SetupSuite() {
	eng := buildTestEngine(s.db)
	taskSvc, nodeSvc, validSvc := buildTestServices(eng)

	s.bus = eventtest.NewFakeBus()
	s.approve = wrapWithBusAndDB(s.db, s.bus, command.NewApproveTaskHandler(s.db, taskSvc, nodeSvc, validSvc, nil))
	s.transfer = wrapWithBusAndDB(s.db, s.bus, command.NewTransferTaskHandler(s.db, taskSvc, validSvc, nil, nil))
	s.reassign = wrapWithBusAndDB(s.db, s.bus, command.NewReassignTaskHandler(s.db, taskSvc, nil))
	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "task-activation")

	node := &approval.FlowNode{
		FlowVersionID:             s.fixture.VersionID,
		Key:                       "activation-sequential",
		Kind:                      approval.NodeApproval,
		Name:                      "Sequential Node",
		ApprovalMethod:            approval.ApprovalSequential,
		PassRule:                  approval.PassAll,
		IsTransferAllowed:         true,
		ConsecutiveApproverAction: approval.ConsecutiveApproverNone,
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should create sequential node")
	s.nodeID = node.ID

	endNode := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "activation-end",
		Kind:          approval.NodeEnd,
		Name:          "End",
	}
	_, err = s.db.NewInsert().Model(endNode).Exec(s.ctx)
	s.Require().NoError(err, "Should create end node")

	edge := &approval.FlowEdge{
		FlowVersionID: s.fixture.VersionID,
		Key:           "activation-edge",
		SourceNodeID:  node.ID,
		SourceNodeKey: node.Key,
		TargetNodeID:  endNode.ID,
		TargetNodeKey: endNode.Key,
	}
	_, err = s.db.NewInsert().Model(edge).Exec(s.ctx)
	s.Require().NoError(err, "Should create edge to end node")
}

func (s *TaskActivationTestSuite) SetupTest() {
	s.bus.Reset()
}

func (s *TaskActivationTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *TaskActivationTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

// seedSequential creates a running instance whose sequential node holds one
// task per assignee: the first Pending, the rest queued as Waiting — the state
// the engine leaves behind after inserting a sequential node's tasks.
func (s *TaskActivationTestSuite) seedSequential(assignees ...string) *approval.Instance {
	s.seq++
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Task Activation Test",
		InstanceNo:    fmt.Sprintf("TA-%04d", s.seq),
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &s.nodeID,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should create running instance")

	visitID := ensureActiveVisit(s.T(), s.ctx, s.db, "default", inst.ID, s.nodeID).ID

	for i, a := range assignees {
		status := approval.TaskPending
		if i > 0 {
			status = approval.TaskWaiting
		}

		task := &approval.Task{
			TenantID:   "default",
			InstanceID: inst.ID,
			NodeID:     s.nodeID,
			VisitID:    visitID,
			AssigneeID: a,
			SortOrder:  i + 1,
			Status:     status,
		}
		_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
		s.Require().NoError(err, "Should create task for "+a)
	}

	return inst
}

func (s *TaskActivationTestSuite) taskFor(instanceID, assigneeID string) approval.Task {
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

// activations returns every TaskActivatedEvent captured so far.
func (s *TaskActivationTestSuite) activations() []*approval.TaskActivatedEvent {
	captured := s.bus.CapturedByType(approval.EventTypeTaskActivated)
	events := make([]*approval.TaskActivatedEvent, 0, len(captured))

	for _, evt := range captured {
		activated, ok := evt.(*approval.TaskActivatedEvent)
		s.Require().True(ok, "Captured event should be *TaskActivatedEvent")

		events = append(events, activated)
	}

	return events
}

// A queued approver must not be told it is their turn when the person ahead of
// them finishes someone else's step — only the actual queue advance activates.
func (s *TaskActivationTestSuite) TestSequentialApprovalActivatesOneApproverAtATime() {
	inst := s.seedSequential("seq-user-1", "seq-user-2", "seq-user-3")

	s.approveAs(s.taskFor(inst.ID, "seq-user-1").ID, "seq-user-1")

	activated := s.activations()
	s.Require().Len(activated, 1, "Finishing the first task should activate exactly one successor")
	s.Assert().Equal("seq-user-2", activated[0].Assignee.ID,
		"The next queued approver is the one activated")
	s.Assert().Equal(approval.TaskActivationQueueAdvanced, activated[0].Reason,
		"A promoted queue task is activated because the queue advanced")

	s.Assert().Equal(approval.TaskWaiting, s.taskFor(inst.ID, "seq-user-3").Status,
		"The third approver stays queued and is not announced")
}

// The recipient of a transfer learns it is their turn through
// TaskActivatedEvent; TaskTransferredEvent reports the act itself and names the
// outgoing assignee.
func (s *TaskActivationTestSuite) TestTransferActivatesTheRecipient() {
	inst := s.seedSequential("seq-user-1", "seq-user-2")

	s.transferTo(s.taskFor(inst.ID, "seq-user-1").ID, "seq-user-1", "transfer-target")

	activated := s.activations()
	s.Require().Len(activated, 1, "A transfer should activate exactly the recipient")
	s.Assert().Equal("transfer-target", activated[0].Assignee.ID,
		"The transfer recipient is the one activated")
	s.Assert().Equal(approval.TaskActivationTransferred, activated[0].Reason,
		"A transferred task is activated by the transfer")

	s.Assert().Len(s.bus.CapturedByType(approval.EventTypeTaskTransferred), 1,
		"The transfer itself is still reported separately")
}

// Reassignment moves an already-pending task to a different person, who must be
// told just as a transfer recipient is.
func (s *TaskActivationTestSuite) TestReassignActivatesTheNewAssignee() {
	inst := s.seedSequential("seq-user-1", "seq-user-2")

	_, err := s.reassign.Handle(s.ctx, command.ReassignTaskCmd{
		TaskID:        s.taskFor(inst.ID, "seq-user-1").ID,
		NewAssigneeID: "reassign-target",
		Operator:      approval.UserInfo{ID: "admin-1", Name: "admin-1"},
		Caller:        approval.SystemCaller,
	})
	s.Require().NoError(err, "Reassign should succeed")

	activated := s.activations()
	s.Require().Len(activated, 1, "A reassignment should activate exactly the new assignee")
	s.Assert().Equal("reassign-target", activated[0].Assignee.ID,
		"The new assignee is the one activated")
	s.Assert().Equal(approval.TaskActivationReassigned, activated[0].Reason,
		"A reassigned task is activated by the reassignment")
}

func (s *TaskActivationTestSuite) approveAs(taskID, userID string) {
	_, err := s.approve.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   taskID,
		Operator: approval.UserInfo{ID: userID, Name: userID},
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "Approve by "+userID+" should not error")
}

func (s *TaskActivationTestSuite) transferTo(taskID, fromUserID, toUserID string) {
	_, err := s.transfer.Handle(s.ctx, command.TransferTaskCmd{
		TaskID:       taskID,
		Operator:     approval.UserInfo{ID: fromUserID, Name: fromUserID},
		TransferToID: toUserID,
		Caller:       approval.SystemCaller,
	})
	s.Require().NoError(err, "Transfer should succeed")
}
