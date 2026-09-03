package service_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
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
	formSeq int
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
		s.Assert().ErrorIs(err, approval.ErrInvalidTaskTransition, "Should reject invalid transition")
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
		s.Assert().ErrorIs(err, approval.ErrTaskNotPending, "Should reject stale in-memory task status")
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

		node := &approval.FlowNode{}
		node.ID = nodeID

		events, err := s.svc.CancelRemainingTasks(s.ctx, s.db, inst, node, "test cancel")
		s.Require().NoError(err, "Should cancel remaining tasks without error")
		s.Require().Len(events, 2, "Should emit one TaskCanceledEvent per canceled task")

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

		events, err := s.svc.CancelInstanceTasks(s.ctx, s.db, inst, "test cancel")
		s.Require().NoError(err, "Should cancel instance tasks without error")
		s.Require().Len(events, 2, "Should emit one TaskCanceledEvent per canceled task")

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

		events, err := s.svc.ActivateNextSequentialTask(s.ctx, s.db, instance, node)
		s.Require().NoError(err, "Should activate next sequential task without error")

		s.Require().Len(events, 1, "Advancing the queue should announce exactly one activation")

		activated, ok := events[0].(*approval.TaskActivatedEvent)
		s.Require().True(ok, "Activation event should be *TaskActivatedEvent")
		s.Assert().Equal(approval.TaskActivationQueueAdvanced, activated.Reason,
			"A promoted queue task is activated because the queue advanced")

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

		events, err := s.svc.ActivateNextSequentialTask(s.ctx, s.db, instance, node)
		s.Assert().NoError(err, "Should not error when no waiting tasks exist")
		s.Assert().Empty(events, "Nothing was activated, so nothing should be announced")
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

		_, err = s.svc.ActivateNextSequentialTask(s.ctx, s.db, instance, node)
		s.Require().NoError(err, "Should activate next sequential task with timeout")

		var reloaded approval.Task

		reloaded.ID = activatedTask.ID
		s.Require().NoError(
			s.db.NewSelect().Model(&reloaded).WherePK().Scan(s.ctx),
			"Should load activated task after sequential activation",
		)

		s.Assert().Equal(approval.TaskPending, reloaded.Status, "Waiting task should transition to pending")
		s.Require().NotNil(reloaded.Deadline, "Activated pending task should have deadline set")
		// Timezone-agnostic: compare deadline to the same row's CreatedAt (both pass through the
		// same driver Scan path so any timezone drift is identical). Node TimeoutHours=2,
		// activation happens within the same test, so the recalculated deadline must be at
		// least 1h past CreatedAt; if the implementation incorrectly inherited the seeded
		// stale deadline (now-8h) it would fall well before CreatedAt.
		s.Assert().True(
			reloaded.Deadline.Unwrap().After(reloaded.CreatedAt.AddHours(1).Unwrap()),
			"Activated task deadline should be recalculated from activation time",
		)
	})

	s.Run("AutoPassesApplicantSeatAndAdvances", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		nodeID := s.fixture.NodeIDs[0]
		applicantTask := insertTaskWithAssignee(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskWaiting, 1, "applicant")
		insertTaskWithAssignee(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskWaiting, 2, "user-b")

		instance := &approval.Instance{}
		instance.ID = inst.ID
		instance.ApplicantID = inst.ApplicantID
		node := &approval.FlowNode{
			Kind:                approval.NodeApproval,
			SameApplicantAction: approval.SameApplicantAutoPass,
		}
		node.ID = nodeID

		events, err := s.svc.ActivateNextSequentialTask(s.ctx, s.db, instance, node)
		s.Require().NoError(err, "Should advance the queue without error")

		s.Require().Len(events, 2, "Should announce the system approval and the next activation")

		approved, ok := events[0].(*approval.TaskApprovedEvent)
		s.Require().True(ok, "First event should be the applicant seat's system approval")
		s.Assert().Equal(applicantTask.ID, approved.TaskID, "System approval should target the applicant's seat")

		activated, ok := events[1].(*approval.TaskActivatedEvent)
		s.Require().True(ok, "Second event should be the next seat's activation")
		s.Assert().Equal("user-b", activated.Assignee.ID, "Queue should advance past the applicant to the next approver")

		s.Assert().Equal(approval.TaskApproved, s.loadTaskStatus(inst.ID, "applicant"), "Applicant's seat should be approved without action")
		s.Assert().Equal(approval.TaskPending, s.loadTaskStatus(inst.ID, "user-b"), "Next seat should become pending")
	})

	s.Run("AutoPassSkipsExplicitlyAddedApplicantSeat", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		nodeID := s.fixture.NodeIDs[0]
		added := insertTaskWithAssignee(s.T(), s.ctx, s.db, inst.ID, nodeID, approval.TaskWaiting, 1, "applicant")

		addType := approval.AddAssigneeAfter
		_, err := s.db.NewUpdate().
			Model((*approval.Task)(nil)).
			Set("add_assignee_type", addType).
			Where(func(cb orm.ConditionBuilder) { cb.PKEquals(added.ID) }).
			Exec(s.ctx)
		s.Require().NoError(err, "Should mark the seat as an add-assignee addition")

		instance := &approval.Instance{}
		instance.ID = inst.ID
		instance.ApplicantID = inst.ApplicantID
		node := &approval.FlowNode{
			Kind:                approval.NodeApproval,
			SameApplicantAction: approval.SameApplicantAutoPass,
		}
		node.ID = nodeID

		events, err := s.svc.ActivateNextSequentialTask(s.ctx, s.db, instance, node)
		s.Require().NoError(err, "Should advance the queue without error")

		s.Require().Len(events, 1, "An explicitly added applicant seat should activate normally")
		_, ok := events[0].(*approval.TaskActivatedEvent)
		s.Assert().True(ok, "The only event should be the activation")
		s.Assert().Equal(approval.TaskPending, s.loadTaskStatus(inst.ID, "applicant"), "Explicitly added seat should stay manual")
	})
}

// loadTaskStatus reloads a task's status by instance and assignee.
func (s *TaskServiceTestSuite) loadTaskStatus(instanceID, assigneeID string) approval.TaskStatus {
	var task approval.Task

	s.Require().NoError(
		s.db.NewSelect().Model(&task).
			Where(func(cb orm.ConditionBuilder) {
				cb.Equals("instance_id", instanceID).Equals("assignee_id", assigneeID)
			}).
			Limit(1).
			Scan(s.ctx),
		"Should load task for assignee "+assigneeID,
	)

	return task.Status
}

// --- PrepareOperation ---

func (s *TaskServiceTestSuite) TestPrepareOperation() {
	s.Run("Success", func() {
		nodeID, instanceID, taskID := setupPrepareOperationData(s.T(), s.ctx, s.db, s.fixture, approval.InstanceRunning, approval.TaskPending, "op-user-1")

		tc, err := s.svc.PrepareOperation(s.ctx, s.db, taskID, approval.UserInfo{ID: "op-user-1"}, approval.SystemCaller, nil)
		s.Require().NoError(err, "Should prepare operation context")
		s.Assert().Equal(instanceID, tc.Instance.ID, "Prepared instance ID should match")
		s.Assert().Equal(taskID, tc.Task.ID, "Prepared task ID should match")
		s.Assert().Equal(nodeID, tc.Node.ID, "Prepared node ID should match")
	})

	s.Run("TaskNotFound", func() {
		_, err := s.svc.PrepareOperation(s.ctx, s.db, "non-existent", approval.UserInfo{ID: "op-user-1"}, approval.SystemCaller, nil)
		s.Assert().ErrorIs(err, approval.ErrTaskNotFound, "Should return task not found for missing task ID")
	})

	s.Run("InstanceCompleted", func() {
		_, _, taskID := setupPrepareOperationData(s.T(), s.ctx, s.db, s.fixture, approval.InstanceApproved, approval.TaskPending, "op-user-2")

		_, err := s.svc.PrepareOperation(s.ctx, s.db, taskID, approval.UserInfo{ID: "op-user-2"}, approval.SystemCaller, nil)
		s.Assert().ErrorIs(err, approval.ErrInstanceCompleted, "Should reject operation on completed instance")
	})

	s.Run("NotAssignee", func() {
		_, _, taskID := setupPrepareOperationData(s.T(), s.ctx, s.db, s.fixture, approval.InstanceRunning, approval.TaskPending, "op-user-3")

		_, err := s.svc.PrepareOperation(s.ctx, s.db, taskID, approval.UserInfo{ID: "wrong-user"}, approval.SystemCaller, nil)
		s.Assert().ErrorIs(err, approval.ErrNotAssignee, "Should reject non-assignee operator")
	})

	s.Run("TaskNotPending", func() {
		_, _, taskID := setupPrepareOperationData(s.T(), s.ctx, s.db, s.fixture, approval.InstanceRunning, approval.TaskApproved, "op-user-4")

		_, err := s.svc.PrepareOperation(s.ctx, s.db, taskID, approval.UserInfo{ID: "op-user-4"}, approval.SystemCaller, nil)
		s.Assert().ErrorIs(err, approval.ErrTaskNotPending, "Should reject non-pending task")
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

		_, err = s.svc.PrepareOperation(s.ctx, s.db, taskID, approval.UserInfo{ID: "op-user-5"}, approval.SystemCaller, nil)
		s.Assert().ErrorIs(err, approval.ErrTaskNotPending, "Should reject operations on tasks outside current node")
	})
}

// --- PrepareOperation: submitted-value validation ---

// setupFormValidationTask seeds a draft version carrying the given form schema,
// a node carrying the given field permissions, and a running instance + pending
// task on it, returning the task ID. A draft version sidesteps the one-published-
// version-per-flow unique index while still exercising PrepareOperation's schema
// load, which reads form_fields regardless of version status.
func (s *TaskServiceTestSuite) setupFormValidationTask(
	fields []approval.FormFieldDefinition,
	permissions map[string]approval.Permission,
	instanceFormData map[string]any,
	assigneeID string,
) string {
	s.formSeq++

	version := &approval.FlowVersion{
		FlowID: s.fixture.FlowID, Version: 1000 + s.formSeq, Status: approval.VersionDraft,
		FormFields: fields,
	}
	_, err := s.db.NewInsert().Model(version).Exec(s.ctx)
	s.Require().NoError(err, "should insert form-validation flow version")

	node := &approval.FlowNode{
		FlowVersionID:    version.ID,
		Key:              "form-val-node-" + assigneeID,
		Kind:             approval.NodeApproval,
		Name:             "Form Val Node",
		FieldPermissions: permissions,
	}
	_, err = s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "should insert form-validation node")

	instance := &approval.Instance{
		TenantID: "default", FlowID: s.fixture.FlowID, FlowVersionID: version.ID,
		Title: "Form Val", InstanceNo: "FORMVAL-" + assigneeID,
		ApplicantID: "applicant", Status: approval.InstanceRunning,
		CurrentNodeID: &node.ID, FormData: instanceFormData,
	}
	_, err = s.db.NewInsert().Model(instance).Exec(s.ctx)
	s.Require().NoError(err, "should insert form-validation instance")

	task := &approval.Task{
		TenantID: "default", InstanceID: instance.ID, NodeID: node.ID,
		VisitID:    ensureActiveVisit(s.T(), s.ctx, s.db, instance.ID, node.ID).ID,
		AssigneeID: assigneeID, SortOrder: 1, Status: approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "should insert form-validation task")

	return task.ID
}

func (s *TaskServiceTestSuite) TestPrepareOperationFormValidation() {
	fields := []approval.FormFieldDefinition{
		{Key: "reason", Kind: approval.FieldInput, Label: "Reason", Validation: &approval.ValidationRule{MinLength: new(3)}},
		{Key: "locked", Kind: approval.FieldInput, Label: "Locked", Validation: &approval.ValidationRule{MinLength: new(3)}},
	}
	editable := map[string]approval.Permission{"reason": approval.PermissionEditable}

	s.Run("RejectsInvalidEditableValue", func() {
		taskID := s.setupFormValidationTask(fields, editable, nil, "form-val-invalid")

		_, err := s.svc.PrepareOperation(s.ctx, s.db, taskID, approval.UserInfo{ID: "form-val-invalid"}, approval.SystemCaller, map[string]any{"reason": "no"})

		var re result.Error
		s.Require().ErrorAs(err, &re, "an editable value violating its schema rule must be rejected")
		s.Assert().Equal(approval.ErrCodeFormValidationFailed, re.Code, "should carry the form validation error code")
	})

	s.Run("IgnoresInvalidNonEditableValue", func() {
		taskID := s.setupFormValidationTask(fields, editable, nil, "form-val-noneditable")

		// "locked" is submitted but not granted editable permission, so it is
		// filtered out before validation — and never merged.
		tc, err := s.svc.PrepareOperation(s.ctx, s.db, taskID, approval.UserInfo{ID: "form-val-noneditable"}, approval.SystemCaller, map[string]any{"locked": "no"})
		s.Require().NoError(err, "a non-editable submitted key is filtered before validation")
		s.Assert().NotContains(tc.Instance.FormData, "locked", "the non-editable value must not be merged")
	})

	s.Run("RejectsUnknownEditableKey", func() {
		perms := map[string]approval.Permission{"ghost": approval.PermissionEditable}
		taskID := s.setupFormValidationTask(fields, perms, nil, "form-val-unknown")

		_, err := s.svc.PrepareOperation(s.ctx, s.db, taskID, approval.UserInfo{ID: "form-val-unknown"}, approval.SystemCaller, map[string]any{"ghost": "value"})

		var re result.Error
		s.Require().ErrorAs(err, &re, "an editable key with no schema field must be rejected")
		s.Assert().Equal(approval.ErrCodeFormValidationFailed, re.Code, "should carry the form validation error code")
	})

	s.Run("AllowsEmptyEditableValue", func() {
		taskID := s.setupFormValidationTask(fields, editable, nil, "form-val-empty")

		tc, err := s.svc.PrepareOperation(s.ctx, s.db, taskID, approval.UserInfo{ID: "form-val-empty"}, approval.SystemCaller, map[string]any{"reason": ""})
		s.Require().NoError(err, "emptiness is deferred to the required-permission check, not rejected at this layer")
		s.Assert().Equal("", tc.Instance.FormData["reason"], "the empty editable value is still merged")
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
			Caller:             approval.SystemCaller,
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
			Caller:                  approval.SystemCaller,
		})
		s.Assert().ErrorIs(err, approval.ErrNotAssignee, "Should reject non-assignee when assignee constraint is enabled")
	})
}

// --- BuildActionLog ---

func (s *TaskServiceTestSuite) TestBuildActionLog() {
	s.Run("WithAllFields", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		task := insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, s.fixture.NodeIDs[0], approval.TaskPending, 1)
		operator := approval.UserInfo{ID: "log-user-1", Name: "Logger"}

		log := s.svc.BuildActionLog(inst.ID, task, operator, approval.ActionApprove, service.ActionLogParams{
			Opinion:          "looks good",
			TransferTo:       &approval.UserInfo{ID: "transfer-to-1", Name: "Transfer User", DepartmentName: new("研发部")},
			RollbackToNodeID: "rollback-node-1",
		})

		s.Assert().Equal(approval.ActionApprove, log.Action, "Action should be approve")
		s.Assert().Equal("log-user-1", log.OperatorID, "Operator ID should match")
		s.Assert().NotNil(log.NodeID, "Node ID should be populated from task")
		s.Assert().Equal(task.NodeID, *log.NodeID, "Node ID should match task")
		s.Assert().NotNil(log.TaskID, "Task ID should be populated from task")
		s.Assert().Equal(task.ID, *log.TaskID, "Task ID should match task")
		s.Assert().NotNil(log.Opinion, "Opinion should be set when provided")
		s.Assert().Equal("looks good", *log.Opinion, "Opinion value should match input")
		s.Assert().NotNil(log.TransferToID, "Transfer target should be set when provided")
		s.Assert().Equal("transfer-to-1", *log.TransferToID, "Transfer target should match input")
		s.Assert().NotNil(log.TransferToName, "Transfer target name should be set when provided")
		s.Assert().Equal("Transfer User", *log.TransferToName, "Transfer target name should match input")
		s.Assert().NotNil(log.TransferToDepartmentName, "Transfer target department should be snapshotted when provided")
		s.Assert().Equal("研发部", *log.TransferToDepartmentName, "Transfer target department should match input")
		s.Assert().NotNil(log.RollbackToNodeID, "Rollback target should be set when provided")
		s.Assert().Equal("rollback-node-1", *log.RollbackToNodeID, "Rollback target should match input")
	})

	s.Run("WithEmptyOptionalFields", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		task := insertTaskWithDetails(s.T(), s.ctx, s.db, inst.ID, s.fixture.NodeIDs[1], approval.TaskPending, 1)
		operator := approval.UserInfo{ID: "log-user-2", Name: "Logger2"}

		log := s.svc.BuildActionLog(inst.ID, task, operator, approval.ActionSubmit, service.ActionLogParams{})

		s.Assert().Nil(log.Opinion, "Should not set opinion when empty")
		s.Assert().Nil(log.TransferToID, "Should not set transfer_to_id when empty")
		s.Assert().Nil(log.TransferToName, "Should not set transfer_to_name when empty")
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

		result, err := s.svc.IsAuthorizedForNodeOperation(s.ctx, s.db, peerTask.InstanceID, peerTask.NodeID, "peer-user")
		s.Require().NoError(err, "Authorization check should not error")
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

		result, err := s.svc.IsAuthorizedForNodeOperation(s.ctx, s.db, inst.ID, s.fixture.NodeIDs[1], "admin-user")
		s.Require().NoError(err, "Authorization check should not error")
		s.Assert().True(result, "Flow admin should be authorized")
	})

	s.Run("Unauthorized", func() {
		result, err := s.svc.IsAuthorizedForNodeOperation(s.ctx, s.db, "non-existent", "non-existent-node", "random-user")
		s.Require().NoError(err, "A non-existent instance is a denial, not an error")
		s.Assert().False(result, "Random user should not be authorized")
	})
}

// --- IsUrgeAuthorized ---

// TestIsUrgeAuthorized pins the whole vocabulary of who may nudge the people
// deciding an instance. The rule is "on the hook for the decision": the
// applicant and everyone a task was ever opened on, however the work was
// handed around, and nobody who is merely watching.
func (s *TaskServiceTestSuite) TestIsUrgeAuthorized() {
	s.Run("Applicant", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)

		authorized, err := s.svc.IsUrgeAuthorized(s.ctx, s.db, inst.ID, "applicant")
		s.Require().NoError(err, "Authorization check should not error")
		s.Assert().True(authorized, "The applicant is waiting on the decision and may urge")
	})

	s.Run("CurrentAssignee", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		insertTaskWithAssignee(s.T(), s.ctx, s.db, inst.ID, s.fixture.NodeIDs[0], approval.TaskPending, 1, "holder")

		authorized, err := s.svc.IsUrgeAuthorized(s.ctx, s.db, inst.ID, "holder")
		s.Require().NoError(err, "Authorization check should not error")
		s.Assert().True(authorized, "A pending assignee may urge peers on the same instance")
	})

	s.Run("PastAssignee", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		insertTaskWithAssignee(s.T(), s.ctx, s.db, inst.ID, s.fixture.NodeIDs[0], approval.TaskApproved, 1, "earlier")

		authorized, err := s.svc.IsUrgeAuthorized(s.ctx, s.db, inst.ID, "earlier")
		s.Require().NoError(err, "Authorization check should not error")
		s.Assert().True(authorized,
			"Someone who already approved still waits on the instance and may ask why it is stuck")
	})

	s.Run("Delegator", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		task := insertTaskWithAssignee(s.T(), s.ctx, s.db, inst.ID, s.fixture.NodeIDs[0],
			approval.TaskPending, 1, "delegate")

		_, err := s.db.NewUpdate().
			Model((*approval.Task)(nil)).
			Set("delegator_id", "original-approver").
			Where(func(cb orm.ConditionBuilder) { cb.Equals("id", task.ID) }).
			Exec(s.ctx)
		s.Require().NoError(err, "Delegating the task away should succeed")

		authorized, err := s.svc.IsUrgeAuthorized(s.ctx, s.db, inst.ID, "original-approver")
		s.Require().NoError(err, "Authorization check should not error")
		s.Assert().True(authorized,
			"The slot is still the delegator's; handing execution to a delegate must not cost them the right to urge")
	})

	s.Run("CCRecipientDenied", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)
		insertTaskWithAssignee(s.T(), s.ctx, s.db, inst.ID, s.fixture.NodeIDs[0], approval.TaskPending, 1, "holder")

		_, err := s.db.NewInsert().
			Model(&approval.CCRecord{InstanceID: inst.ID, CCUserID: "observer"}).
			Exec(s.ctx)
		s.Require().NoError(err, "Inserting the CC record should succeed")

		authorized, err := s.svc.IsUrgeAuthorized(s.ctx, s.db, inst.ID, "observer")
		s.Require().NoError(err, "Authorization check should not error")
		s.Assert().False(authorized,
			"A CC recipient only watches: this is the one leg that separates urging from reading the instance")
	})

	s.Run("StrangerDenied", func() {
		inst := s.fixture.createInstance(s.T(), s.ctx, s.db, approval.InstanceRunning)

		authorized, err := s.svc.IsUrgeAuthorized(s.ctx, s.db, inst.ID, "nobody")
		s.Require().NoError(err, "Authorization check should not error")
		s.Assert().False(authorized, "An unrelated user may not urge")
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
		canRemove, err := s.svc.CanRemoveAssigneeTask(s.ctx, s.db, engine.NewFlowEngine(nil, nil, nil, nil, nil, nil), node, *task1)
		s.Require().NoError(err, "Should evaluate removability without error")
		s.Assert().True(canRemove, "Should allow removal when other actionable tasks exist")
	})
}
