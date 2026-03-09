package command_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/dispatcher"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &AddAssigneeTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// AddAssigneeTestSuite tests the AddAssigneeHandler.
type AddAssigneeTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *command.AddAssigneeHandler
	fixture *MinimalFixture
	nodeID  string

	instanceSeq int
}

func (s *AddAssigneeTestSuite) SetupSuite() {
	s.handler = command.NewAddAssigneeHandler(s.db, service.NewTaskService(), dispatcher.NewEventPublisher(), nil)
	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "add-assignee")

	node := &approval.FlowNode{
		FlowVersionID:        s.fixture.VersionID,
		Key:                  "add-assignee-node",
		Kind:                 approval.NodeApproval,
		Name:                 "Add Assignee Node",
		IsAddAssigneeAllowed: true,
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")
	s.nodeID = node.ID
}

func (s *AddAssigneeTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *AddAssigneeTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *AddAssigneeTestSuite) setupData(assigneeID string) (*approval.Instance, *approval.Task) {
	s.instanceSeq++
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Add Assignee Test",
		InstanceNo:    fmt.Sprintf("AA-%04d", s.instanceSeq),
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &s.nodeID,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     s.nodeID,
		AssigneeID: assigneeID,
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	return inst, task
}

func (s *AddAssigneeTestSuite) TestAddAssigneeSuccess() {
	_, task := s.setupData("operator-1")

	operator := approval.OperatorInfo{ID: "operator-1", Name: "Operator"}
	_, err := s.handler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   task.ID,
		UserIDs:  []string{"new-user-1", "new-user-2"},
		AddType:  approval.AddAssigneeBefore,
		Operator: operator,
	})
	s.Require().NoError(err, "Should add assignees without error")

	// Verify new tasks created
	var tasks []approval.Task
	s.Require().NoError(s.db.NewSelect().Model(&tasks).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", task.InstanceID) }).
		OrderBy("sort_order").
		Scan(s.ctx), "Should not return error")
	s.Assert().GreaterOrEqual(len(tasks), 3, "Should have at least 3 tasks (1 original + 2 new)")
}

func (s *AddAssigneeTestSuite) TestAddAssigneeNotAllowed() {
	// Create node with add-assignee disabled
	node := &approval.FlowNode{
		FlowVersionID:        s.fixture.VersionID,
		Key:                  "no-add-node",
		Kind:                 approval.NodeApproval,
		Name:                 "No Add Node",
		IsAddAssigneeAllowed: false,
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
		Title:         "No Add Test",
		InstanceNo:    "AA-002",
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
		AssigneeID: "operator-2",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	operator := approval.OperatorInfo{ID: "operator-2", Name: "Operator"}
	_, err = s.handler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   task.ID,
		UserIDs:  []string{"new-user-1"},
		AddType:  approval.AddAssigneeBefore,
		Operator: operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrAddAssigneeNotAllowed, "Should return ErrAddAssigneeNotAllowed")
}

func (s *AddAssigneeTestSuite) TestAddAssigneeNotAssignee() {
	_, task := s.setupData("operator-1")

	operator := approval.OperatorInfo{ID: "wrong-user", Name: "Wrong"}
	_, err := s.handler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   task.ID,
		UserIDs:  []string{"new-user-1"},
		AddType:  approval.AddAssigneeBefore,
		Operator: operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrNotAssignee, "Should return ErrNotAssignee")
}

func (s *AddAssigneeTestSuite) TestAddAssigneeTaskNotFound() {
	operator := approval.OperatorInfo{ID: "operator-1", Name: "Operator"}
	_, err := s.handler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   "non-existent",
		UserIDs:  []string{"new-user-1"},
		AddType:  approval.AddAssigneeBefore,
		Operator: operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrTaskNotFound, "Should return ErrTaskNotFound")
}

func (s *AddAssigneeTestSuite) TestAddAssigneeInstanceCompleted() {
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Completed Instance",
		InstanceNo:    "AA-003",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceApproved,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     s.nodeID,
		AssigneeID: "operator-3",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	operator := approval.OperatorInfo{ID: "operator-3", Name: "Operator"}
	_, err = s.handler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   task.ID,
		UserIDs:  []string{"new-user-1"},
		AddType:  approval.AddAssigneeBefore,
		Operator: operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrInstanceCompleted, "Should return ErrInstanceCompleted")
}

func (s *AddAssigneeTestSuite) TestAddAssigneeRejectsDisallowedConfiguredType() {
	node := &approval.FlowNode{
		FlowVersionID:        s.fixture.VersionID,
		Key:                  "add-assignee-limited",
		Kind:                 approval.NodeApproval,
		Name:                 "Limited Add Assignee Node",
		IsAddAssigneeAllowed: true,
		AddAssigneeTypes:     []approval.AddAssigneeType{approval.AddAssigneeBefore},
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)

	s.Require().NoError(err, "Should create node with restricted add assignee types")
	defer func() {
		_, _ = s.db.NewDelete().Model(node).WherePK().Exec(s.ctx)
	}()

	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Limited Add Type Test",
		InstanceNo:    "AA-LIMITED-001",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &node.ID,
	}
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should create instance")

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     node.ID,
		AssigneeID: "operator-limited",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should create pending task")

	operator := approval.OperatorInfo{ID: "operator-limited", Name: "Operator"}
	_, err = s.handler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   task.ID,
		UserIDs:  []string{"new-user-1"},
		AddType:  approval.AddAssigneeAfter,
		Operator: operator,
	})
	s.Require().Error(err, "Should reject disallowed add assignee type")
	s.Assert().ErrorIs(err, shared.ErrInvalidAddAssigneeType, "Should return ErrInvalidAddAssigneeType")
}

func (s *AddAssigneeTestSuite) TestAddAssigneeTaskNotPending() {
	_, task := s.setupData("operator-4")

	_, err := s.db.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("status", approval.TaskApproved).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	operator := approval.OperatorInfo{ID: "operator-4", Name: "Operator"}
	_, err = s.handler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   task.ID,
		UserIDs:  []string{"new-user-1"},
		AddType:  approval.AddAssigneeBefore,
		Operator: operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrTaskNotPending, "Should reject adding assignee for non-pending task")
}

func (s *AddAssigneeTestSuite) TestAddAssigneeTaskNotCurrentNode() {
	inst, task := s.setupData("operator-5")

	otherNode := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "other-current-node",
		Kind:          approval.NodeApproval,
		Name:          "Other Current Node",
	}
	_, err := s.db.NewInsert().Model(otherNode).Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	_, err = s.db.NewUpdate().
		Model((*approval.Instance)(nil)).
		Set("current_node_id", otherNode.ID).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(inst.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should not return error")

	operator := approval.OperatorInfo{ID: "operator-5", Name: "Operator"}
	_, err = s.handler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   task.ID,
		UserIDs:  []string{"new-user-1"},
		AddType:  approval.AddAssigneeBefore,
		Operator: operator,
	})
	s.Require().Error(err, "Should return error")
	s.Assert().ErrorIs(err, shared.ErrTaskNotPending, "Should reject adding assignee for non-current node task")
}

func (s *AddAssigneeTestSuite) TestAddAssigneeShouldStartTimeoutWhenNewTaskIsPending() {
	_, task := s.setupData("operator-deadline")

	_, err := s.db.NewUpdate().
		Model((*approval.FlowNode)(nil)).
		Set("timeout_hours", 4).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.NodeID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should configure node timeout hours")

	originalDeadline := timex.Now().AddHours(8)
	_, err = s.db.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("deadline", originalDeadline).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should set original task deadline")

	operator := approval.OperatorInfo{ID: "operator-deadline", Name: "Operator"}
	startedAt := timex.Now()
	_, err = s.handler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   task.ID,
		UserIDs:  []string{"new-deadline-user"},
		AddType:  approval.AddAssigneeParallel,
		Operator: operator,
	})
	s.Require().NoError(err, "Should add assignee without error")

	var newTasks []approval.Task
	s.Require().NoError(
		s.db.NewSelect().
			Model(&newTasks).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("parent_task_id", task.ID)
			}).
			Scan(s.ctx),
		"Should query newly added tasks",
	)
	s.Require().Len(newTasks, 1, "Should create one added assignee task")
	s.Assert().Equal(approval.TaskPending, newTasks[0].Status, "Parallel add should create a pending task")
	s.Require().NotNil(newTasks[0].Deadline, "Pending task should start timeout immediately")
	s.Assert().True(
		newTasks[0].Deadline.Unwrap().After(startedAt.AddHours(3).Unwrap()),
		"Pending task deadline should be calculated from add-assignee time",
	)
	s.Assert().NotEqual(
		originalDeadline.Unwrap().Unix(),
		newTasks[0].Deadline.Unwrap().Unix(),
		"Pending task deadline should not inherit original task deadline directly",
	)
}

func (s *AddAssigneeTestSuite) TestAddAssigneeShouldKeepWaitingTaskDeadlineEmpty() {
	_, task := s.setupData("operator-waiting-deadline")

	_, err := s.db.NewUpdate().
		Model((*approval.FlowNode)(nil)).
		Set("timeout_hours", 4).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.NodeID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should configure node timeout hours")

	originalDeadline := timex.Now().AddHours(8)
	_, err = s.db.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("deadline", originalDeadline).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should set original task deadline")

	operator := approval.OperatorInfo{ID: "operator-waiting-deadline", Name: "Operator"}
	_, err = s.handler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   task.ID,
		UserIDs:  []string{"new-waiting-user"},
		AddType:  approval.AddAssigneeAfter,
		Operator: operator,
	})
	s.Require().NoError(err, "Should add assignee without error")

	var newTasks []approval.Task
	s.Require().NoError(
		s.db.NewSelect().
			Model(&newTasks).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("parent_task_id", task.ID)
			}).
			Scan(s.ctx),
		"Should query newly added tasks",
	)
	s.Require().Len(newTasks, 1, "Should create one added assignee task")
	s.Assert().Equal(approval.TaskWaiting, newTasks[0].Status, "Add-after should create waiting task")
	s.Assert().Nil(newTasks[0].Deadline, "Waiting task should not start timeout before activation")
}

func (s *AddAssigneeTestSuite) TestAddAssigneeBeforeShouldResetOriginalTaskDeadline() {
	_, task := s.setupData("operator-before-deadline")

	_, err := s.db.NewUpdate().
		Model((*approval.FlowNode)(nil)).
		Set("timeout_hours", 4).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.NodeID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should configure node timeout hours")

	_, err = s.db.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("deadline", timex.Now().AddHours(6)).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should set original task deadline")

	operator := approval.OperatorInfo{ID: "operator-before-deadline", Name: "Operator"}
	_, err = s.handler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   task.ID,
		UserIDs:  []string{"new-before-user"},
		AddType:  approval.AddAssigneeBefore,
		Operator: operator,
	})
	s.Require().NoError(err, "Should add assignee without error")

	var originalTask approval.Task

	originalTask.ID = task.ID
	s.Require().NoError(
		s.db.NewSelect().Model(&originalTask).WherePK().Scan(s.ctx),
		"Should reload original task after add-before",
	)
	s.Assert().Equal(approval.TaskWaiting, originalTask.Status, "Original task should become waiting after add-before")
	s.Assert().Nil(originalTask.Deadline, "Original waiting task should clear deadline until re-activation")
}

func (s *AddAssigneeTestSuite) TestAddAssigneeShouldDeduplicateUserIDsAndIgnoreEmpty() {
	_, task := s.setupData("operator-dedup")

	operator := approval.OperatorInfo{ID: "operator-dedup", Name: "Operator"}
	_, err := s.handler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   task.ID,
		UserIDs:  []string{"new-user-1", "", "new-user-1", "new-user-2"},
		AddType:  approval.AddAssigneeParallel,
		Operator: operator,
	})
	s.Require().NoError(err, "Should add assignees without error")

	var addedTasks []approval.Task
	s.Require().NoError(
		s.db.NewSelect().
			Model(&addedTasks).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("parent_task_id", task.ID)
			}).
			OrderBy("sort_order").
			Scan(s.ctx),
		"Should query newly added tasks",
	)
	s.Require().Len(addedTasks, 2, "Should create only unique non-empty assignee tasks")
	s.Assert().Equal("new-user-1", addedTasks[0].AssigneeID, "First added task should keep first-seen assignee order")
	s.Assert().Equal("new-user-2", addedTasks[1].AssigneeID, "Second added task should keep first-seen assignee order")

	var outbox approval.EventOutbox
	s.Require().NoError(
		s.db.NewSelect().
			Model(&outbox).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("event_type", "approval.task.assignees_added")
			}).
			OrderByDesc("created_at").
			Limit(1).
			Scan(s.ctx),
		"Should query latest assignee-added event",
	)

	rawIDs, ok := outbox.Payload["assigneeIds"].([]any)
	s.Require().True(ok, "Event payload should contain assigneeIds array")
	s.Require().Len(rawIDs, 2, "Event payload should contain deduplicated assignee IDs")
	s.Assert().Equal("new-user-1", rawIDs[0], "Event payload should preserve first-seen assignee order")
	s.Assert().Equal("new-user-2", rawIDs[1], "Event payload should preserve first-seen assignee order")
}

func (s *AddAssigneeTestSuite) TestAddAssigneeShouldSkipExistingActiveAssignee() {
	inst, task := s.setupData("operator-existing")

	existingTask := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     s.nodeID,
		AssigneeID: "new-user-1",
		SortOrder:  2,
		Status:     approval.TaskPending,
	}
	_, err := s.db.NewInsert().Model(existingTask).Exec(s.ctx)
	s.Require().NoError(err, "Should insert existing active assignee task")

	operator := approval.OperatorInfo{ID: "operator-existing", Name: "Operator"}
	_, err = s.handler.Handle(s.ctx, command.AddAssigneeCmd{
		TaskID:   task.ID,
		UserIDs:  []string{"new-user-1", "new-user-2"},
		AddType:  approval.AddAssigneeParallel,
		Operator: operator,
	})
	s.Require().NoError(err, "Should add assignees without error")

	var addedTasks []approval.Task
	s.Require().NoError(
		s.db.NewSelect().
			Model(&addedTasks).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("parent_task_id", task.ID)
			}).
			Scan(s.ctx),
		"Should query newly added tasks",
	)
	s.Require().Len(addedTasks, 1, "Should skip assignee that already has active task on node")
	s.Assert().Equal("new-user-2", addedTasks[0].AssigneeID, "Should only create task for non-existing active assignee")

	var outbox approval.EventOutbox
	s.Require().NoError(
		s.db.NewSelect().
			Model(&outbox).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("event_type", "approval.task.assignees_added")
			}).
			OrderByDesc("created_at").
			Limit(1).
			Scan(s.ctx),
		"Should query latest assignee-added event",
	)

	rawIDs, ok := outbox.Payload["assigneeIds"].([]any)
	s.Require().True(ok, "Event payload should contain assigneeIds array")
	s.Require().Len(rawIDs, 1, "Event payload should only include newly inserted assignee")
	s.Assert().Equal("new-user-2", rawIDs[0], "Event payload should exclude existing active assignee")
}

func (s *AddAssigneeTestSuite) TestAddAssigneeShouldBeConcurrencySafe() {
	skipSQLiteConcurrencyTest(s.T(), s.ctx, s.db, "SQLite returns SQLITE_BUSY under write races in this concurrency scenario")

	_, task := s.setupData("operator-concurrency")
	operator := approval.OperatorInfo{ID: "operator-concurrency", Name: "Operator"}

	lockReady, releaseLock, lockDone := holdSharedTableLock(s.ctx, s.db, "apv_task")

	<-lockReady

	start := make(chan struct{})
	errCh := make(chan error, 2)

	var wg sync.WaitGroup

	runOne := func() {
		<-start

		err := s.db.RunInTX(s.ctx, func(ctx context.Context, tx orm.DB) error {
			txCtx := contextx.SetDB(ctx, tx)
			_, err := s.handler.Handle(txCtx, command.AddAssigneeCmd{
				TaskID:   task.ID,
				UserIDs:  []string{"new-user-concurrency"},
				AddType:  approval.AddAssigneeParallel,
				Operator: operator,
			})

			return err
		})
		errCh <- err
	}

	wg.Go(runOne)
	wg.Go(runOne)
	close(start)

	time.Sleep(200 * time.Millisecond)
	close(releaseLock)

	s.Require().NoError(<-lockDone, "Table lock transaction should complete without error")
	wg.Wait()
	close(errCh)

	for err := range errCh {
		s.Require().NoError(err, "Concurrent add-assignee operations should complete without error")
	}

	var activeTasks []approval.Task
	s.Require().NoError(
		s.db.NewSelect().
			Model(&activeTasks).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("instance_id", task.InstanceID).
					Equals("node_id", task.NodeID).
					Equals("assignee_id", "new-user-concurrency").
					In("status", []approval.TaskStatus{approval.TaskPending, approval.TaskWaiting})
			}).
			Scan(s.ctx),
		"Should query active tasks for concurrently added assignee",
	)
	s.Assert().Len(activeTasks, 1, "Concurrent add-assignee should create only one active task for the same assignee")
}

func (s *AddAssigneeTestSuite) TestAddAssigneeAndPrepareOperationShouldAvoidDeadlock() {
	_, task := s.setupData("operator-lock-order")
	taskSvc := service.NewTaskService()

	lockReady := make(chan struct{})
	releaseLock := make(chan struct{})
	lockDone := make(chan error, 1)

	go func() {
		lockDone <- s.db.RunInTX(s.ctx, func(ctx context.Context, tx orm.DB) error {
			lockedTask := approval.Task{}
			lockedTask.ID = task.ID

			if err := tx.NewSelect().
				Model(&lockedTask).
				WherePK().
				ForUpdate().
				Scan(ctx); err != nil {
				return err
			}

			close(lockReady)
			<-releaseLock

			return nil
		})
	}()

	<-lockReady

	addDone := make(chan error, 1)
	go func() {
		addDone <- s.db.RunInTX(s.ctx, func(ctx context.Context, tx orm.DB) error {
			txCtx := contextx.SetDB(ctx, tx)
			_, err := s.handler.Handle(txCtx, command.AddAssigneeCmd{
				TaskID:   task.ID,
				UserIDs:  []string{"lock-order-user"},
				AddType:  approval.AddAssigneeParallel,
				Operator: approval.OperatorInfo{ID: "operator-lock-order", Name: "Operator"},
			})

			return err
		})
	}()

	prepareDone := make(chan error, 1)
	go func() {
		prepareDone <- s.db.RunInTX(s.ctx, func(ctx context.Context, tx orm.DB) error {
			txCtx := contextx.SetDB(ctx, tx)
			_, err := taskSvc.PrepareOperation(txCtx, tx, task.ID, "operator-lock-order", nil)

			return err
		})
	}()

	// Allow both goroutines to start and block on the task lock
	time.Sleep(200 * time.Millisecond)
	close(releaseLock)

	select {
	case err := <-lockDone:
		s.Require().NoError(err, "Task locker transaction should complete without error")
	case <-time.After(5 * time.Second):
		s.FailNow("Task locker transaction should not block indefinitely")
	}

	select {
	case err := <-addDone:
		s.Require().NoError(err, "Add-assignee operation should complete without deadlock")
	case <-time.After(5 * time.Second):
		s.FailNow("Add-assignee operation should not block indefinitely")
	}

	select {
	case err := <-prepareDone:
		s.Require().NoError(err, "PrepareOperation should complete without deadlock")
	case <-time.After(5 * time.Second):
		s.FailNow("PrepareOperation should not block indefinitely")
	}
}
