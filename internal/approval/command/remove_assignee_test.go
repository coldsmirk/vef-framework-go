package command_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &RemoveAssigneeTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// RemoveAssigneeTestSuite tests the RemoveAssigneeHandler.
type RemoveAssigneeTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.RemoveAssigneeHandler
	fixture *MinimalFixture
	nodeID  string

	instanceSeq int
}

func (s *RemoveAssigneeTestSuite) SetupSuite() {
	eng := buildTestEngine()
	taskSvc, nodeSvc, _ := buildTestServices(eng)
	pub := dispatcher.NewEventPublisher()

	s.handler = command.NewRemoveAssigneeHandler(s.db, taskSvc, nodeSvc, eng, pub)
	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "remove")

	node := &approval.FlowNode{
		FlowVersionID:           s.fixture.VersionID,
		Key:                     "remove-node",
		Kind:                    approval.NodeApproval,
		Name:                    "Remove Assignee Node",
		PassRule:                approval.PassAll,
		IsRemoveAssigneeAllowed: true,
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")
	s.nodeID = node.ID
}

func (s *RemoveAssigneeTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *RemoveAssigneeTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *RemoveAssigneeTestSuite) setupData() (inst *approval.Instance, task1, task2 *approval.Task) {
	s.instanceSeq++
	inst = &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Remove Assignee Test",
		InstanceNo:    fmt.Sprintf("RM-%04d", s.instanceSeq),
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &s.nodeID,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	task1 = &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     s.nodeID,
		AssigneeID: "assignee-1",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task1).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	task2 = &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     s.nodeID,
		AssigneeID: "assignee-2",
		SortOrder:  2,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task2).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	return inst, task1, task2
}

func (s *RemoveAssigneeTestSuite) TestRemoveSuccess() {
	_, _, task2 := s.setupData()

	// assignee-1 removes assignee-2 (peer operation)
	operator := approval.OperatorInfo{ID: "assignee-1", Name: "Assignee 1"}
	_, err := s.handler.Handle(s.ctx, command.RemoveAssigneeCmd{
		TaskID:   task2.ID,
		Operator: operator,
	})
	s.Require().NoError(err, "Should remove assignee without error")

	// Verify task2 is removed
	var updated approval.Task

	updated.ID = task2.ID
	s.Require().NoError(s.db.NewSelect().Model(&updated).WherePK().Scan(s.ctx), "Should not return error")
	s.Assert().Equal(approval.TaskRemoved, updated.Status, "Task should be removed")

	// Verify action log
	var logs []approval.ActionLog
	s.Require().NoError(s.db.NewSelect().Model(&logs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", task2.InstanceID) }).
		Scan(s.ctx), "Should not return error")

	found := false
	for _, log := range logs {
		if log.Action == approval.ActionRemoveAssignee {
			found = true
		}
	}

	s.Assert().True(found, "Should have a remove_assignee action log")
}

func (s *RemoveAssigneeTestSuite) TestRemoveWaitingTaskShouldSucceed() {
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Remove Waiting Task Test",
		InstanceNo:    "RM-WAIT-001",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &s.nodeID,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert running instance")

	pendingTask := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     s.nodeID,
		AssigneeID: "assignee-pending",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(pendingTask).Exec(s.ctx)
	s.Require().NoError(err, "Should insert pending peer task")

	waitingTask := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     s.nodeID,
		AssigneeID: "assignee-waiting",
		SortOrder:  2,
		Status:     approval.TaskWaiting,
	}
	_, err = s.db.NewInsert().Model(waitingTask).Exec(s.ctx)
	s.Require().NoError(err, "Should insert waiting task to remove")

	operator := approval.OperatorInfo{ID: "assignee-pending", Name: "Pending Assignee"}
	_, err = s.handler.Handle(s.ctx, command.RemoveAssigneeCmd{
		TaskID:   waitingTask.ID,
		Operator: operator,
	})
	s.Require().NoError(err, "Removing waiting task should be supported")

	reloadedWaiting := approval.Task{}
	reloadedWaiting.ID = waitingTask.ID
	s.Require().NoError(
		s.db.NewSelect().Model(&reloadedWaiting).WherePK().Scan(s.ctx),
		"Should reload removed waiting task",
	)
	s.Assert().Equal(approval.TaskRemoved, reloadedWaiting.Status, "Waiting task should transition to removed")

	reloadedPending := approval.Task{}
	reloadedPending.ID = pendingTask.ID
	s.Require().NoError(
		s.db.NewSelect().Model(&reloadedPending).WherePK().Scan(s.ctx),
		"Should reload peer pending task",
	)
	s.Assert().Equal(approval.TaskPending, reloadedPending.Status, "Peer pending task should stay actionable")
}

func (s *RemoveAssigneeTestSuite) TestRemoveNotAllowed() {
	// Create node with remove disabled
	node := &approval.FlowNode{
		FlowVersionID:           s.fixture.VersionID,
		Key:                     "no-remove-node",
		Kind:                    approval.NodeApproval,
		Name:                    "No Remove Node",
		IsRemoveAssigneeAllowed: false,
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)

	s.Require().NoError(err, "Should not return error")
	defer func() {
		_, _ = s.db.NewDelete().Model(node).WherePK().Exec(s.ctx)
	}()

	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "No Remove Test",
		InstanceNo:    "RM-002",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &node.ID,
	}
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     node.ID,
		AssigneeID: "assignee-3",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	operator := approval.OperatorInfo{ID: "assignee-3", Name: "Assignee"}
	_, err = s.handler.Handle(s.ctx, command.RemoveAssigneeCmd{
		TaskID:   task.ID,
		Operator: operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrRemoveAssigneeNotAllowed, "Should return expected error")
}

func (s *RemoveAssigneeTestSuite) TestRemoveTaskNotFound() {
	operator := approval.OperatorInfo{ID: "assignee-1", Name: "Assignee"}
	_, err := s.handler.Handle(s.ctx, command.RemoveAssigneeCmd{
		TaskID:   "non-existent",
		Operator: operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrTaskNotFound, "Should return expected error")
}

func (s *RemoveAssigneeTestSuite) TestRemoveInstanceCompleted() {
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Completed Remove Test",
		InstanceNo:    "RM-003",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceApproved,
		CurrentNodeID: &s.nodeID,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert completed instance")

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     s.nodeID,
		AssigneeID: "assignee-3",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should insert task on completed instance")

	operator := approval.OperatorInfo{ID: "assignee-3", Name: "Assignee"}
	_, err = s.handler.Handle(s.ctx, command.RemoveAssigneeCmd{
		TaskID:   task.ID,
		Operator: operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrInstanceCompleted, "Should reject remove on completed instance")
}

func (s *RemoveAssigneeTestSuite) TestRemoveTaskNotCurrentNode() {
	inst, _, task2 := s.setupData()

	otherNode := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "remove-other-current-node",
		Kind:          approval.NodeApproval,
		Name:          "Remove Other Current Node",
	}
	_, err := s.db.NewInsert().Model(otherNode).Exec(s.ctx)
	s.Require().NoError(err, "Should insert other current node")

	_, err = s.db.NewUpdate().
		Model((*approval.Instance)(nil)).
		Set("current_node_id", otherNode.ID).
		Where(func(cb orm.ConditionBuilder) {
			cb.PKEquals(inst.ID)
		}).
		Exec(s.ctx)
	s.Require().NoError(err, "Should update instance current node")

	operator := approval.OperatorInfo{ID: "assignee-1", Name: "Assignee 1"}
	_, err = s.handler.Handle(s.ctx, command.RemoveAssigneeCmd{
		TaskID:   task2.ID,
		Operator: operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrTaskNotPending, "Should reject remove on non-current node task")
}

func (s *RemoveAssigneeTestSuite) TestRemoveAssigneeShouldBeConcurrencySafe() {
	skipSQLiteConcurrencyTest(s.T(), s.ctx, s.db, "SQLite returns SQLITE_BUSY under write races in this concurrency scenario")

	_, task1, task2 := s.setupData()

	lockReady, releaseLock, lockDone := holdSharedTableLock(s.ctx, s.db, "apv_task")

	<-lockReady

	start := make(chan struct{})
	errCh := make(chan error, 2)

	var wg sync.WaitGroup

	runOne := func(taskID, operatorID string) {
		<-start

		err := s.db.RunInTX(s.ctx, func(ctx context.Context, tx orm.DB) error {
			txCtx := contextx.SetDB(ctx, tx)
			_, err := s.handler.Handle(txCtx, command.RemoveAssigneeCmd{
				TaskID:   taskID,
				Operator: approval.OperatorInfo{ID: operatorID, Name: operatorID},
			})

			return err
		})
		errCh <- err
	}

	wg.Go(func() { runOne(task1.ID, "assignee-2") })
	wg.Go(func() { runOne(task2.ID, "assignee-1") })
	close(start)

	time.Sleep(200 * time.Millisecond)
	close(releaseLock)

	s.Require().NoError(<-lockDone, "Table lock transaction should complete without error")
	wg.Wait()
	close(errCh)

	successCount := 0
	safeRejectedCount := 0

	otherErrors := make([]string, 0, 1)
	for err := range errCh {
		if err == nil {
			successCount++

			continue
		}

		if errors.Is(err, shared.ErrLastAssigneeRemoval) || errors.Is(err, shared.ErrTaskNotPending) || errors.Is(err, shared.ErrNotAssignee) {
			safeRejectedCount++

			continue
		}

		otherErrors = append(otherErrors, err.Error())
	}

	s.Assert().Equal(1, successCount, "Concurrent remove-assignee should allow only one successful removal")
	s.Assert().Equal(
		1,
		safeRejectedCount,
		"Concurrent remove-assignee should reject one request to avoid removing all actionable assignees, unexpected errors: %v",
		otherErrors,
	)

	var tasks []approval.Task
	s.Require().NoError(
		s.db.NewSelect().
			Model(&tasks).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("instance_id", task1.InstanceID).
					Equals("node_id", task1.NodeID)
			}).
			Scan(s.ctx),
		"Should query node tasks after concurrent remove attempts",
	)

	removedCount := 0

	actionableCount := 0
	for _, task := range tasks {
		if task.Status == approval.TaskRemoved {
			removedCount++
		}

		if task.Status == approval.TaskPending || task.Status == approval.TaskWaiting {
			actionableCount++
		}
	}

	s.Assert().Equal(1, removedCount, "Concurrent remove-assignee should only remove one task")
	s.Assert().Equal(1, actionableCount, "Concurrent remove-assignee should keep one actionable task to avoid deadlock")
}
