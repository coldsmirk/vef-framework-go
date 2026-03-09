package service_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &TaskServiceTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// TaskServiceTestSuite tests the TaskService.
type TaskServiceTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	svc     *service.TaskService
	fixture *SvcFixture
}

func (s *TaskServiceTestSuite) SetupSuite() {
	s.svc = service.NewTaskService()
	s.fixture = setupSvcFixture(s.T(), s.ctx, s.db)
}

func (s *TaskServiceTestSuite) TearDownTest() {
	deleteAll(s.ctx, s.db,
		(*approval.ActionLog)(nil),
		(*approval.Task)(nil),
		(*approval.Instance)(nil),
	)
}

func (s *TaskServiceTestSuite) TearDownSuite() {
	cleanAllServiceData(s.ctx, s.db)
}

// --- FinishTask ---

func (s *TaskServiceTestSuite) TestFinishTask() {
	s.Run("ValidTransition", func() {
		task := insertTask(s.T(), s.ctx, s.db, s.fixture, approval.TaskPending)
		err := s.svc.FinishTask(s.ctx, s.db, task, approval.TaskApproved)
		s.Require().NoError(err, "Should finish task")

		s.Assert().Equal(approval.TaskApproved, task.Status, "Should update status in memory")
		s.Assert().NotNil(task.FinishedAt, "Should set FinishedAt")

		// Verify DB
		var dbTask approval.Task

		dbTask.ID = task.ID
		s.Require().NoError(
			s.db.NewSelect().Model(&dbTask).WherePK().Scan(s.ctx),
			"Should load task from DB after finishing",
		)
		s.Assert().Equal(approval.TaskApproved, dbTask.Status, "DB should reflect new status")
		s.Assert().NotNil(dbTask.FinishedAt, "DB should have FinishedAt")
	})

	s.Run("InvalidTransition", func() {
		task := insertTask(s.T(), s.ctx, s.db, s.fixture, approval.TaskApproved)
		err := s.svc.FinishTask(s.ctx, s.db, task, approval.TaskPending)
		s.Assert().ErrorIs(err, shared.ErrInvalidTaskTransition, "Should reject invalid transition")
	})

	s.Run("StaleTaskStatusShouldFail", func() {
		task := insertTask(s.T(), s.ctx, s.db, s.fixture, approval.TaskPending)

		_, err := s.db.NewUpdate().
			Model((*approval.Task)(nil)).
			Set("status", approval.TaskApproved).
			Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.ID) }).
			Exec(s.ctx)
		s.Require().NoError(err, "Should update task status directly in DB")

		err = s.svc.FinishTask(s.ctx, s.db, task, approval.TaskRejected)
		s.Assert().ErrorIs(err, shared.ErrTaskNotPending, "Should reject stale in-memory task status")
	})
}

// --- CancelRemainingTasks ---

func (s *TaskServiceTestSuite) TestCancelRemainingTasks() {
	s.Run("CancelsPendingAndWaiting", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		nodeID := s.fixture.NodeIDs[0]
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskPending, 1)
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskWaiting, 2)
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskApproved, 3)

		err := s.svc.CancelRemainingTasks(s.ctx, s.db, inst.ID, nodeID)
		s.Require().NoError(err, "Should cancel remaining tasks without error")

		var tasks []approval.Task
		s.Require().NoError(s.db.NewSelect().
			Model(&tasks).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("instance_id", inst.ID).Equals("node_id", nodeID)
			}).
			OrderBy("sort_order").
			Scan(s.ctx), "Should query node tasks after cancellation")

		s.Assert().Equal(approval.TaskCanceled, tasks[0].Status, "Pending should be canceled")
		s.Assert().Equal(approval.TaskCanceled, tasks[1].Status, "Waiting should be canceled")
		s.Assert().Equal(approval.TaskApproved, tasks[2].Status, "Approved should remain unchanged")
	})
}

// --- CancelInstanceTasks ---

func (s *TaskServiceTestSuite) TestCancelInstanceTasks() {
	s.Run("CancelsAllPendingWaitingForInstance", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, s.fixture.NodeIDs[0], approval.TaskPending, 1)
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, s.fixture.NodeIDs[1], approval.TaskWaiting, 1)
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, s.fixture.NodeIDs[0], approval.TaskRejected, 2)

		err := s.svc.CancelInstanceTasks(s.ctx, s.db, inst.ID)
		s.Require().NoError(err, "Should cancel instance tasks without error")

		var tasks []approval.Task
		s.Require().NoError(s.db.NewSelect().
			Model(&tasks).
			Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
			Scan(s.ctx), "Should query instance tasks after cancellation")

		canceledCount := 0
		for _, task := range tasks {
			if task.Status == approval.TaskCanceled {
				canceledCount++
			}
		}

		s.Assert().Equal(2, canceledCount, "Should cancel 2 tasks")
	})
}

// --- ActivateNextSequentialTask ---

func (s *TaskServiceTestSuite) TestActivateNextSequentialTask() {
	s.Run("ActivatesNextWaitingTask", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		nodeID := s.fixture.NodeIDs[0]
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskWaiting, 1)
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskWaiting, 2)

		instance := &approval.Instance{}
		instance.ID = inst.ID
		node := &approval.FlowNode{}
		node.ID = nodeID

		err := s.svc.ActivateNextSequentialTask(s.ctx, s.db, instance, node)
		s.Require().NoError(err, "Should activate next sequential task without error")

		var tasks []approval.Task
		s.Require().NoError(s.db.NewSelect().
			Model(&tasks).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("instance_id", inst.ID).Equals("node_id", nodeID)
			}).
			OrderBy("sort_order").
			Scan(s.ctx), "Should query tasks after sequential activation")

		s.Assert().Equal(approval.TaskPending, tasks[0].Status, "First waiting task should become pending")
		s.Assert().Equal(approval.TaskWaiting, tasks[1].Status, "Second waiting task should remain waiting")
	})

	s.Run("NoWaitingTasks", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)

		instance := &approval.Instance{}
		instance.ID = inst.ID
		node := &approval.FlowNode{}
		node.ID = s.fixture.NodeIDs[1]

		err := s.svc.ActivateNextSequentialTask(s.ctx, s.db, instance, node)
		s.Assert().NoError(err, "Should not error when no waiting tasks exist")
	})

	s.Run("ActivatedTaskShouldStartTimeoutFromPending", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		nodeID := s.fixture.NodeIDs[0]
		activatedTask := insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskWaiting, 1)
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskWaiting, 2)

		staleDeadline := timex.Now().AddHours(-8)
		_, err := s.db.NewUpdate().
			Model((*approval.Task)(nil)).
			Set("deadline", staleDeadline).
			Where(func(cb orm.ConditionBuilder) { cb.PKEquals(activatedTask.ID) }).
			Exec(s.ctx)
		s.Require().NoError(err, "Should seed waiting task with a stale deadline")

		instance := &approval.Instance{}
		instance.ID = inst.ID
		node := &approval.FlowNode{TimeoutHours: 2}
		node.ID = nodeID
		startedAt := timex.Now()

		err = s.svc.ActivateNextSequentialTask(s.ctx, s.db, instance, node)
		s.Require().NoError(err, "Should activate next sequential task with timeout")

		var reloaded approval.Task

		reloaded.ID = activatedTask.ID
		s.Require().NoError(
			s.db.NewSelect().Model(&reloaded).WherePK().Scan(s.ctx),
			"Should load activated task after sequential activation",
		)

		s.Assert().Equal(approval.TaskPending, reloaded.Status, "Waiting task should transition to pending")
		s.Require().NotNil(reloaded.Deadline, "Activated pending task should have deadline set")
		s.Assert().True(
			reloaded.Deadline.Unwrap().After(startedAt.AddHours(1).Unwrap()),
			"Activated task deadline should be recalculated from activation time",
		)
	})
}

// --- PrepareOperation ---

func (s *TaskServiceTestSuite) TestPrepareOperation() {
	s.Run("Success", func() {
		nodeID, instanceID, taskID := setupPrepareOperationData(s.T(), s.ctx, s.db, s.fixture, approval.InstanceRunning, approval.TaskPending, "op-user-1")

		tc, err := s.svc.PrepareOperation(s.ctx, s.db, taskID, "op-user-1", nil)
		s.Require().NoError(err, "Should prepare operation context")
		s.Assert().Equal(instanceID, tc.Instance.ID, "Prepared instance ID should match")
		s.Assert().Equal(taskID, tc.Task.ID, "Prepared task ID should match")
		s.Assert().Equal(nodeID, tc.Node.ID, "Prepared node ID should match")
	})

	s.Run("TaskNotFound", func() {
		_, err := s.svc.PrepareOperation(s.ctx, s.db, "non-existent", "op-user-1", nil)
		s.Assert().ErrorIs(err, shared.ErrTaskNotFound, "Should return task not found for missing task ID")
	})

	s.Run("InstanceCompleted", func() {
		_, _, taskID := setupPrepareOperationData(s.T(), s.ctx, s.db, s.fixture, approval.InstanceApproved, approval.TaskPending, "op-user-2")

		_, err := s.svc.PrepareOperation(s.ctx, s.db, taskID, "op-user-2", nil)
		s.Assert().ErrorIs(err, shared.ErrInstanceCompleted, "Should reject operation on completed instance")
	})

	s.Run("NotAssignee", func() {
		_, _, taskID := setupPrepareOperationData(s.T(), s.ctx, s.db, s.fixture, approval.InstanceRunning, approval.TaskPending, "op-user-3")

		_, err := s.svc.PrepareOperation(s.ctx, s.db, taskID, "wrong-user", nil)
		s.Assert().ErrorIs(err, shared.ErrNotAssignee, "Should reject non-assignee operator")
	})

	s.Run("TaskNotPending", func() {
		_, _, taskID := setupPrepareOperationData(s.T(), s.ctx, s.db, s.fixture, approval.InstanceRunning, approval.TaskApproved, "op-user-4")

		_, err := s.svc.PrepareOperation(s.ctx, s.db, taskID, "op-user-4", nil)
		s.Assert().ErrorIs(err, shared.ErrTaskNotPending, "Should reject non-pending task")
	})

	s.Run("TaskNotCurrentNode", func() {
		nodeID, instanceID, taskID := setupPrepareOperationData(s.T(), s.ctx, s.db, s.fixture, approval.InstanceRunning, approval.TaskPending, "op-user-5")

		otherNode := &approval.FlowNode{
			FlowVersionID: s.fixture.VersionID,
			Key:           "prep-other-current-node",
			Kind:          approval.NodeApproval,
			Name:          "Prep Other Current Node",
		}
		_, err := s.db.NewInsert().Model(otherNode).Exec(s.ctx)
		s.Require().NoError(err, "Should create another node as current node")

		s.Require().NotEqual(nodeID, otherNode.ID, "Other node should differ from task node")

		_, err = s.db.NewUpdate().
			Model((*approval.Instance)(nil)).
			Set("current_node_id", otherNode.ID).
			Where(func(cb orm.ConditionBuilder) { cb.PKEquals(instanceID) }).
			Exec(s.ctx)
		s.Require().NoError(err, "Should move instance current node away from task node")

		_, err = s.svc.PrepareOperation(s.ctx, s.db, taskID, "op-user-5", nil)
		s.Assert().ErrorIs(err, shared.ErrTaskNotPending, "Should reject operations on tasks outside current node")
	})
}

// --- LoadTaskContextForNodeOperation ---

func (s *TaskServiceTestSuite) TestLoadTaskContextForNodeOperation() {
	s.Run("AllowWaitingTaskWhenPendingNotRequired", func() {
		nodeID, instanceID, taskID := setupPrepareOperationData(
			s.T(), s.ctx, s.db, s.fixture, approval.InstanceRunning, approval.TaskWaiting, "node-op-user-1",
		)

		tc, err := s.svc.LoadTaskContextForNodeOperation(s.ctx, s.db, taskID, service.TaskContextLoadOptions{
			RequireCurrentNode: true,
		})
		s.Require().NoError(err, "Should load context when pending constraint is not required")
		s.Assert().Equal(instanceID, tc.Instance.ID, "Loaded instance ID should match")
		s.Assert().Equal(taskID, tc.Task.ID, "Loaded task ID should match")
		s.Assert().Equal(nodeID, tc.Node.ID, "Loaded node ID should match")
	})

	s.Run("RequireAssigneeShouldRejectNonAssignee", func() {
		_, _, taskID := setupPrepareOperationData(
			s.T(), s.ctx, s.db, s.fixture, approval.InstanceRunning, approval.TaskPending, "node-op-user-2",
		)

		_, err := s.svc.LoadTaskContextForNodeOperation(s.ctx, s.db, taskID, service.TaskContextLoadOptions{
			OperatorID:              "other-user",
			RequireOperatorAssignee: true,
			RequireTaskPending:      true,
			RequireCurrentNode:      true,
		})
		s.Assert().ErrorIs(err, shared.ErrNotAssignee, "Should reject non-assignee when assignee constraint is enabled")
	})
}

// --- InsertActionLog ---

func (s *TaskServiceTestSuite) TestInsertActionLog() {
	s.Run("WithAllFields", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		task := insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, s.fixture.NodeIDs[0], approval.TaskPending, 1)
		operator := approval.OperatorInfo{ID: "log-user-1", Name: "Logger"}

		err := s.svc.InsertActionLog(s.ctx, s.db, inst.ID, task, operator, approval.ActionApprove, service.ActionLogParams{
			Opinion: "looks good", TransferToID: "transfer-to-1", TransferToName: "Transfer User", RollbackToNodeID: "rollback-node-1",
		})
		s.Require().NoError(err, "Should insert action log with all optional fields")

		var log approval.ActionLog
		s.Require().NoError(s.db.NewSelect().
			Model(&log).
			Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
			Scan(s.ctx), "Should query inserted action log")

		s.Assert().Equal(approval.ActionApprove, log.Action, "Action should be approve")
		s.Assert().Equal("log-user-1", log.OperatorID, "Operator ID should match")
		s.Assert().NotNil(log.Opinion, "Opinion should be persisted when provided")
		s.Assert().Equal("looks good", *log.Opinion, "Opinion value should match input")
		s.Assert().NotNil(log.TransferToID, "Transfer target should be persisted when provided")
		s.Assert().Equal("transfer-to-1", *log.TransferToID, "Transfer target should match input")
		s.Assert().NotNil(log.TransferToName, "Transfer target name should be persisted when provided")
		s.Assert().Equal("Transfer User", *log.TransferToName, "Transfer target name should match input")
		s.Assert().NotNil(log.RollbackToNodeID, "Rollback target should be persisted when provided")
		s.Assert().Equal("rollback-node-1", *log.RollbackToNodeID, "Rollback target should match input")
	})

	s.Run("WithEmptyOptionalFields", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		task := insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, s.fixture.NodeIDs[1], approval.TaskPending, 1)
		operator := approval.OperatorInfo{ID: "log-user-2", Name: "Logger2"}

		err := s.svc.InsertActionLog(s.ctx, s.db, inst.ID, task, operator, approval.ActionSubmit, service.ActionLogParams{})
		s.Require().NoError(err, "Should insert action log without optional fields")

		var log approval.ActionLog
		s.Require().NoError(s.db.NewSelect().
			Model(&log).
			Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
			Scan(s.ctx), "Should query inserted action log")

		s.Assert().Nil(log.Opinion, "Should not set opinion when empty")
		s.Assert().Nil(log.TransferToID, "Should not set transfer_to_id when empty")
		s.Assert().Nil(log.RollbackToNodeID, "Should not set rollback_to_node_id when empty")
	})
}

// --- IsAuthorizedForNodeOperation ---

func (s *TaskServiceTestSuite) TestIsAuthorizedForNodeOperation() {
	s.Run("PeerAssignee", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		nodeID := s.fixture.NodeIDs[0]
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskPending, 1)
		peerTask := insertTaskWithAssignee(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskPending, 2, "peer-user")

		result := s.svc.IsAuthorizedForNodeOperation(s.ctx, s.db, *peerTask, "peer-user")
		s.Assert().True(result, "Peer assignee should be authorized")
	})

	s.Run("FlowAdmin", func() {
		// Update fixture flow with admin users
		_, err := s.db.NewUpdate().
			Model((*approval.Flow)(nil)).
			Set("admin_user_ids", []string{"admin-user"}).
			Where(func(cb orm.ConditionBuilder) { cb.Equals("id", s.fixture.FlowID) }).
			Exec(s.ctx)
		s.Require().NoError(err, "Should set flow admin users for authorization test")

		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)

		task := approval.Task{
			InstanceID: inst.ID,
			NodeID:     s.fixture.NodeIDs[1],
			AssigneeID: "other-user",
			Status:     approval.TaskPending,
		}

		result := s.svc.IsAuthorizedForNodeOperation(s.ctx, s.db, task, "admin-user")
		s.Assert().True(result, "Flow admin should be authorized")
	})

	s.Run("Unauthorized", func() {
		task := approval.Task{
			InstanceID: "non-existent",
			NodeID:     "non-existent-node",
			AssigneeID: "other",
			Status:     approval.TaskPending,
		}

		result := s.svc.IsAuthorizedForNodeOperation(s.ctx, s.db, task, "random-user")
		s.Assert().False(result, "Random user should not be authorized")
	})
}

// --- CanRemoveAssigneeTask ---

func (s *TaskServiceTestSuite) TestCanRemoveAssigneeTask() {
	s.Run("HasOtherActionableTasks", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		nodeID := s.fixture.NodeIDs[0]
		task1 := insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskPending, 1)
		insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskPending, 2)

		node := &approval.FlowNode{PassRule: approval.PassAll}
		node.ID = nodeID
		canRemove, err := s.svc.CanRemoveAssigneeTask(s.ctx, s.db, engine.NewFlowEngine(nil, nil, nil, nil), node, *task1)
		s.Require().NoError(err, "Should evaluate removability without error")
		s.Assert().True(canRemove, "Should allow removal when other actionable tasks exist")
	})
}
