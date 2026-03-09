package engine_test

import (
	"context"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &ApprovalProcessorTestSuite{
			ProcessorTestBase: ProcessorTestBase{
				Ctx: env.Ctx,
				DB:  env.DB,
			},
		}
	})
}

// ApprovalProcessorTestSuite tests ApprovalProcessor with a real database.
type ApprovalProcessorTestSuite struct {
	suite.Suite
	ProcessorTestBase

	processor *engine.ApprovalProcessor
}

func (s *ApprovalProcessorTestSuite) SetupSuite() {
	s.InitRegistry()
	s.processor = engine.NewApprovalProcessor(nil)
	s.InitFKChain(s.T(), "approval-test", approval.NodeApproval, "Approval")
}

func (s *ApprovalProcessorTestSuite) TearDownTest() {
	s.CleanTransientData(s.T())
}

// --- Tests ---

func (s *ApprovalProcessorTestSuite) TestNodeKind() {
	s.Assert().Equal(approval.NodeApproval, s.processor.NodeKind(), "Should return NodeApproval kind")
}

func (s *ApprovalProcessorTestSuite) TestProcessWithAssignees() {
	instance := s.NewInstance(s.T(), "applicant-1")
	s.InsertAssigneeConfig(s.T(), []string{"user-1", "user-2"})

	pc := s.NewProcessContext(instance, s.NewNode())

	result, err := s.processor.Process(s.Ctx, pc)
	s.Require().NoError(err, "Should process without error")
	s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait for approval tasks")

	tasks := s.QueryTasks(s.T(), instance.ID)
	s.Require().Len(tasks, 2, "Should create 2 tasks")

	assigneeIDs := make([]string, len(tasks))
	for i, task := range tasks {
		assigneeIDs[i] = task.AssigneeID
		s.Assert().Equal(instance.ID, task.InstanceID, "Task should reference instance")
		s.Assert().Equal(s.NodeID, task.NodeID, "Task should reference node")
		s.Assert().Equal(approval.TaskPending, task.Status, "Task should be pending")
	}

	s.Assert().ElementsMatch([]string{"user-1", "user-2"}, assigneeIDs, "Should create tasks for all assignees")
}

func (s *ApprovalProcessorTestSuite) TestProcessSequentialApproval() {
	instance := s.NewInstance(s.T(), "applicant-1")
	s.InsertAssigneeConfig(s.T(), []string{"user-1", "user-2", "user-3"})

	pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
		n.ApprovalMethod = approval.ApprovalSequential
	}))

	result, err := s.processor.Process(s.Ctx, pc)
	s.Require().NoError(err, "Should process without error")
	s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait for sequential approval")

	tasks := s.QueryTasks(s.T(), instance.ID)
	s.Require().Len(tasks, 3, "Should create 3 tasks")

	s.Assert().Equal(approval.TaskPending, tasks[0].Status, "First task should be pending")
	s.Assert().Equal(1, tasks[0].SortOrder, "First task should have sort order 1")

	s.Assert().Equal(approval.TaskWaiting, tasks[1].Status, "Second task should be waiting")
	s.Assert().Equal(2, tasks[1].SortOrder, "Second task should have sort order 2")

	s.Assert().Equal(approval.TaskWaiting, tasks[2].Status, "Third task should be waiting")
	s.Assert().Equal(3, tasks[2].SortOrder, "Third task should have sort order 3")
}

func (s *ApprovalProcessorTestSuite) TestProcessSequentialApprovalShouldStartTimeoutOnPendingOnly() {
	instance := s.NewInstance(s.T(), "applicant-1")
	s.InsertAssigneeConfig(s.T(), []string{"user-1", "user-2", "user-3"})

	startedAt := timex.Now()

	pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
		n.ApprovalMethod = approval.ApprovalSequential
		n.TimeoutHours = 24
	}))

	result, err := s.processor.Process(s.Ctx, pc)
	s.Require().NoError(err, "Should process sequential approval without error")
	s.Assert().Equal(engine.NodeActionWait, result.Action, "Sequential approval should wait for user actions")

	tasks := s.QueryTasks(s.T(), instance.ID)
	s.Require().Len(tasks, 3, "Should create three sequential approval tasks")

	s.Require().NotNil(tasks[0].Deadline, "First pending task should have deadline set immediately")
	s.Assert().True(
		tasks[0].Deadline.Unwrap().After(startedAt.Unwrap().Add(23*time.Hour)),
		"First task deadline should be close to now plus timeout hours",
	)
	s.Assert().Nil(tasks[1].Deadline, "Waiting tasks should not start timeout before activation")
	s.Assert().Nil(tasks[2].Deadline, "Waiting tasks should not start timeout before activation")
}

func (s *ApprovalProcessorTestSuite) TestProcessSetsTaskDeadlineFromTimeoutHours() {
	instance := s.NewInstance(s.T(), "applicant-1")
	s.InsertAssigneeConfig(s.T(), []string{"user-1"})

	startedAt := timex.Now()

	pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
		n.TimeoutHours = 24
	}))

	result, err := s.processor.Process(s.Ctx, pc)
	s.Require().NoError(err, "Should process without error")
	s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait for approval tasks")

	tasks := s.QueryTasks(s.T(), instance.ID)
	s.Require().Len(tasks, 1, "Should create exactly one task")
	s.Require().NotNil(tasks[0].Deadline, "Task deadline should be set from node timeout")

	deadline := tasks[0].Deadline.Unwrap()
	s.Assert().True(deadline.After(startedAt.Unwrap().Add(23*time.Hour)), "Task deadline should be near now plus timeout hours")
}

func (s *ApprovalProcessorTestSuite) TestProcessEmptyAssignee() {
	s.Run("AutoPass", func() {
		defer s.CleanTransientData(s.T())

		instance := s.NewInstance(s.T(), "applicant-1")

		pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
			n.EmptyAssigneeAction = approval.EmptyAssigneeAutoPass
		}))

		result, err := s.processor.Process(s.Ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionContinue, result.Action, "Should auto-pass when no assignees and EmptyAssigneeAutoPass")

		tasks := s.QueryTasks(s.T(), instance.ID)
		s.Assert().Empty(tasks, "Should not create any tasks")
	})

	s.Run("TransferApplicant", func() {
		defer s.CleanTransientData(s.T())

		instance := s.NewInstance(s.T(), "applicant-1")

		pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
			n.EmptyAssigneeAction = approval.EmptyAssigneeTransferApplicant
		}))

		result, err := s.processor.Process(s.Ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait when transferred to applicant")

		tasks := s.QueryTasks(s.T(), instance.ID)
		s.Require().Len(tasks, 1, "Should create one task for applicant")
		s.Assert().Equal("applicant-1", tasks[0].AssigneeID, "Task should be assigned to applicant")
	})

	s.Run("TransferSpecified", func() {
		defer s.CleanTransientData(s.T())

		instance := s.NewInstance(s.T(), "applicant-1")

		pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
			n.EmptyAssigneeAction = approval.EmptyAssigneeTransferSpecified
			n.FallbackUserIDs = []string{"fallback-user-1"}
		}))

		result, err := s.processor.Process(s.Ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait when transferred to specified user")

		tasks := s.QueryTasks(s.T(), instance.ID)
		s.Require().Len(tasks, 1, "Should create one task for fallback user")
		s.Assert().Equal("fallback-user-1", tasks[0].AssigneeID, "Task should be assigned to fallback user")
	})

	s.Run("TransferAdmin", func() {
		defer s.CleanTransientData(s.T())

		instance := s.NewInstance(s.T(), "applicant-1")

		pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
			n.EmptyAssigneeAction = approval.EmptyAssigneeTransferAdmin
			n.AdminUserIDs = []string{"admin-1", "admin-2"}
		}))

		result, err := s.processor.Process(s.Ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait when transferred to admin")

		tasks := s.QueryTasks(s.T(), instance.ID)
		s.Require().Len(tasks, 2, "Should create tasks for all admins")
		assigneeIDs := []string{tasks[0].AssigneeID, tasks[1].AssigneeID}
		s.Assert().ElementsMatch([]string{"admin-1", "admin-2"}, assigneeIDs, "Tasks should be assigned to admins")
	})

	s.Run("TransferSuperiorNilService", func() {
		defer s.CleanTransientData(s.T())

		instance := s.NewInstance(s.T(), "applicant-1")

		pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
			n.EmptyAssigneeAction = approval.EmptyAssigneeTransferSuperior
		}))

		_, err := s.processor.Process(s.Ctx, pc)
		s.Require().ErrorIs(err, engine.ErrAssigneeServiceNotConfigured, "Should return ErrAssigneeServiceNotConfigured when assignee service is nil")
	})

	s.Run("DefaultAction", func() {
		defer s.CleanTransientData(s.T())

		instance := s.NewInstance(s.T(), "applicant-1")

		pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
			n.EmptyAssigneeAction = "unknown_action"
		}))

		_, err := s.processor.Process(s.Ctx, pc)
		s.Require().ErrorIs(err, engine.ErrNoAssignee, "Should return ErrNoAssignee for unknown empty handler action")
	})
}

func (s *ApprovalProcessorTestSuite) TestProcessSameApplicant() {
	s.Run("AutoPass", func() {
		defer s.CleanTransientData(s.T())

		instance := s.NewInstance(s.T(), "user-1")
		s.InsertAssigneeConfig(s.T(), []string{"user-1"})

		pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
			n.SameApplicantAction = approval.SameApplicantAutoPass
		}))

		result, err := s.processor.Process(s.Ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionContinue, result.Action, "Should auto-pass when same applicant")

		tasks := s.QueryTasks(s.T(), instance.ID)
		s.Assert().Empty(tasks, "Should not create tasks when auto-passing")
	})

	s.Run("SelfApprove", func() {
		defer s.CleanTransientData(s.T())

		instance := s.NewInstance(s.T(), "user-1")
		s.InsertAssigneeConfig(s.T(), []string{"user-1"})

		pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
			n.SameApplicantAction = approval.SameApplicantSelfApprove
		}))

		result, err := s.processor.Process(s.Ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait for self-approval")

		tasks := s.QueryTasks(s.T(), instance.ID)
		s.Require().Len(tasks, 1, "Should create one task for self-approval")
		s.Assert().Equal("user-1", tasks[0].AssigneeID, "Task should be assigned to applicant")
	})

	s.Run("NotSameApplicant", func() {
		defer s.CleanTransientData(s.T())

		instance := s.NewInstance(s.T(), "applicant-1")
		s.InsertAssigneeConfig(s.T(), []string{"user-1", "user-2"})

		pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
			n.SameApplicantAction = approval.SameApplicantAutoPass
		}))

		result, err := s.processor.Process(s.Ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait when assignees differ from applicant")

		tasks := s.QueryTasks(s.T(), instance.ID)
		s.Assert().Len(tasks, 2, "Should create tasks normally when assignees differ")
	})

	s.Run("TransferSuperiorNilService", func() {
		defer s.CleanTransientData(s.T())

		instance := s.NewInstance(s.T(), "user-1")
		s.InsertAssigneeConfig(s.T(), []string{"user-1"})

		pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
			n.SameApplicantAction = approval.SameApplicantTransferSuperior
		}))

		_, err := s.processor.Process(s.Ctx, pc)
		s.Require().ErrorIs(err, engine.ErrAssigneeServiceNotConfigured, "Should return ErrAssigneeServiceNotConfigured when assignee service is nil")
	})

	s.Run("DefaultAction", func() {
		defer s.CleanTransientData(s.T())

		instance := s.NewInstance(s.T(), "user-1")
		s.InsertAssigneeConfig(s.T(), []string{"user-1"})

		pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
			n.SameApplicantAction = "unknown_action"
		}))

		result, err := s.processor.Process(s.Ctx, pc)
		s.Require().NoError(err, "Should process without error for unknown same-applicant action")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should default to creating tasks")

		tasks := s.QueryTasks(s.T(), instance.ID)
		s.Require().Len(tasks, 1, "Should create task for applicant in default branch")
		s.Assert().Equal("user-1", tasks[0].AssigneeID, "Task should be assigned to applicant")
	})
}

func (s *ApprovalProcessorTestSuite) TestProcessFormSnapshot() {
	instance := s.NewInstance(s.T(), "applicant-1")
	instance.FormData = map[string]any{"amount": float64(1000)}
	_, err := s.DB.NewUpdate().Model(instance).Select("form_data").WherePK().Exec(s.Ctx)
	s.Require().NoError(err, "Should update instance form data")

	s.InsertAssigneeConfig(s.T(), []string{"user-1"})

	pc := s.NewProcessContext(instance, s.NewNode())

	_, err = s.processor.Process(s.Ctx, pc)
	s.Require().NoError(err, "Should process without error")

	snapshots := s.QueryFormSnapshots(s.T(), instance.ID)
	s.Require().Len(snapshots, 1, "Should create one form snapshot")
	s.Assert().Equal(instance.ID, snapshots[0].InstanceID, "Snapshot should reference instance")
	s.Assert().Equal(s.NodeID, snapshots[0].NodeID, "Snapshot should reference node")
}

func (s *ApprovalProcessorTestSuite) TestProcessDeduplication() {
	instance := s.NewInstance(s.T(), "applicant-1")
	s.InsertAssigneeConfig(s.T(), []string{"user-1", "user-1", "user-2"})

	pc := s.NewProcessContext(instance, s.NewNode())

	result, err := s.processor.Process(s.Ctx, pc)
	s.Require().NoError(err, "Should process without error")
	s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait for approval")

	tasks := s.QueryTasks(s.T(), instance.ID)
	s.Require().Len(tasks, 2, "Should create 2 tasks after deduplication")

	assigneeIDs := make([]string, len(tasks))
	for i, task := range tasks {
		assigneeIDs[i] = task.AssigneeID
	}

	s.Assert().ElementsMatch([]string{"user-1", "user-2"}, assigneeIDs, "Should deduplicate assignees")
}

func (s *ApprovalProcessorTestSuite) TestProcessShouldIgnoreEmptyAssigneeIDs() {
	instance := s.NewInstance(s.T(), "applicant-1")
	s.InsertAssigneeConfig(s.T(), []string{"", "user-1", "", "user-1"})

	pc := s.NewProcessContext(instance, s.NewNode())

	result, err := s.processor.Process(s.Ctx, pc)
	s.Require().NoError(err, "Should process assignee config with empty user IDs")
	s.Assert().Equal(engine.NodeActionWait, result.Action, "Should still wait when at least one non-empty assignee exists")

	tasks := s.QueryTasks(s.T(), instance.ID)
	s.Require().Len(tasks, 1, "Should only create tasks for non-empty unique assignee IDs")
	s.Assert().Equal("user-1", tasks[0].AssigneeID, "Task assignee should be the non-empty resolved user ID")
}

func (s *ApprovalProcessorTestSuite) TestProcessMultipleAssigneeConfigs() {
	instance := s.NewInstance(s.T(), "applicant-1")

	cfg1 := &approval.FlowNodeAssignee{
		NodeID:    s.NodeID,
		Kind:      approval.AssigneeUser,
		IDs:       []string{"user-1"},
		SortOrder: 1,
	}
	_, err := s.DB.NewInsert().Model(cfg1).Exec(s.Ctx)
	s.Require().NoError(err, "Should insert first assignee config")

	cfg2 := &approval.FlowNodeAssignee{
		NodeID:    s.NodeID,
		Kind:      approval.AssigneeUser,
		IDs:       []string{"user-2", "user-3"},
		SortOrder: 2,
	}
	_, err = s.DB.NewInsert().Model(cfg2).Exec(s.Ctx)
	s.Require().NoError(err, "Should insert second assignee config")

	pc := s.NewProcessContext(instance, s.NewNode())

	result, err := s.processor.Process(s.Ctx, pc)
	s.Require().NoError(err, "Should process without error")
	s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait for approval")

	tasks := s.QueryTasks(s.T(), instance.ID)
	s.Require().Len(tasks, 3, "Should create tasks from all assignee configs")

	assigneeIDs := make([]string, len(tasks))
	for i, task := range tasks {
		assigneeIDs[i] = task.AssigneeID
	}

	s.Assert().ElementsMatch([]string{"user-1", "user-2", "user-3"}, assigneeIDs, "Should resolve assignees from all configs")
}

func (s *ApprovalProcessorTestSuite) TestConsecutiveApproverAutoPass() {
	s.Run("AutoPassMatchingApprovers", func() {
		defer s.CleanTransientData(s.T())

		prevNodeID := s.InsertPreviousApprovalNode(s.T(), "prev-approval")
		instance := s.NewInstance(s.T(), "applicant-1")
		s.InsertApprovedTasks(s.T(), instance.ID, prevNodeID, []string{"user-1", "user-2"})
		s.InsertAssigneeConfig(s.T(), []string{"user-1", "user-2", "user-3"})

		pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
			n.ConsecutiveApproverAction = approval.ConsecutiveApproverAutoPass
		}))

		result, err := s.processor.Process(s.Ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait because user-3 still needs approval")

		tasks := s.QueryTasks(s.T(), instance.ID)
		// Filter tasks for current node only
		var currentNodeTasks []approval.Task
		for _, t := range tasks {
			if t.NodeID == s.NodeID {
				currentNodeTasks = append(currentNodeTasks, t)
			}
		}

		s.Require().Len(currentNodeTasks, 3, "Should create 3 tasks")

		approvedCount := 0

		pendingCount := 0
		for _, t := range currentNodeTasks {
			switch t.Status {
			case approval.TaskApproved:
				approvedCount++

				s.Assert().NotNil(t.FinishedAt, "Auto-passed task should have FinishedAt set")
			case approval.TaskPending:
				pendingCount++
			}
		}

		s.Assert().Equal(2, approvedCount, "Should auto-pass 2 matching approvers")
		s.Assert().Equal(1, pendingCount, "Should leave 1 task pending")
	})

	s.Run("NoneActionNoAutoPass", func() {
		defer s.CleanTransientData(s.T())

		prevNodeID := s.InsertPreviousApprovalNode(s.T(), "prev-approval-none")
		instance := s.NewInstance(s.T(), "applicant-1")
		s.InsertApprovedTasks(s.T(), instance.ID, prevNodeID, []string{"user-1"})
		s.InsertAssigneeConfig(s.T(), []string{"user-1", "user-2"})

		pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
			n.ConsecutiveApproverAction = approval.ConsecutiveApproverNone
		}))

		result, err := s.processor.Process(s.Ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait for manual approval")

		tasks := s.QueryTasks(s.T(), instance.ID)

		var currentNodeTasks []approval.Task
		for _, t := range tasks {
			if t.NodeID == s.NodeID {
				currentNodeTasks = append(currentNodeTasks, t)
			}
		}

		s.Require().Len(currentNodeTasks, 2, "Should create 2 tasks")

		for _, t := range currentNodeTasks {
			s.Assert().Equal(approval.TaskPending, t.Status, "All tasks should be pending with none action")
		}
	})

	s.Run("PreviousApproverRejected", func() {
		defer s.CleanTransientData(s.T())

		prevNodeID := s.InsertPreviousApprovalNode(s.T(), "prev-approval-reject")
		instance := s.NewInstance(s.T(), "applicant-1")
		s.InsertRejectedTasks(s.T(), instance.ID, prevNodeID, []string{"user-1"})
		s.InsertAssigneeConfig(s.T(), []string{"user-1", "user-2"})

		pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
			n.ConsecutiveApproverAction = approval.ConsecutiveApproverAutoPass
		}))

		result, err := s.processor.Process(s.Ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait for manual approval")

		tasks := s.QueryTasks(s.T(), instance.ID)

		var currentNodeTasks []approval.Task
		for _, t := range tasks {
			if t.NodeID == s.NodeID {
				currentNodeTasks = append(currentNodeTasks, t)
			}
		}

		for _, t := range currentNodeTasks {
			s.Assert().Equal(approval.TaskPending, t.Status, "No auto-pass when previous approver rejected")
		}
	})

	s.Run("NoPreviousApprovalNode", func() {
		defer s.CleanTransientData(s.T())

		instance := s.NewInstance(s.T(), "applicant-1")
		s.InsertAssigneeConfig(s.T(), []string{"user-1", "user-2"})

		pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
			n.ConsecutiveApproverAction = approval.ConsecutiveApproverAutoPass
		}))

		result, err := s.processor.Process(s.Ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait when no previous approval node")

		tasks := s.QueryTasks(s.T(), instance.ID)
		for _, t := range tasks {
			s.Assert().Equal(approval.TaskPending, t.Status, "All tasks should be pending with no previous node")
		}
	})

	s.Run("AllAutoPassedNodeContinues", func() {
		defer s.CleanTransientData(s.T())

		prevNodeID := s.InsertPreviousApprovalNode(s.T(), "prev-approval-all")
		instance := s.NewInstance(s.T(), "applicant-1")
		s.InsertApprovedTasks(s.T(), instance.ID, prevNodeID, []string{"user-1", "user-2"})
		s.InsertAssigneeConfig(s.T(), []string{"user-1", "user-2"})

		pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
			n.ConsecutiveApproverAction = approval.ConsecutiveApproverAutoPass
		}))

		result, err := s.processor.Process(s.Ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionContinue, result.Action, "Should continue when all tasks are auto-passed")
	})

	s.Run("SequentialAutoPassChain", func() {
		defer s.CleanTransientData(s.T())

		prevNodeID := s.InsertPreviousApprovalNode(s.T(), "prev-approval-seq")
		instance := s.NewInstance(s.T(), "applicant-1")
		s.InsertApprovedTasks(s.T(), instance.ID, prevNodeID, []string{"user-1", "user-2"})
		s.InsertAssigneeConfig(s.T(), []string{"user-1", "user-2", "user-3"})

		pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
			n.ApprovalMethod = approval.ApprovalSequential
			n.ConsecutiveApproverAction = approval.ConsecutiveApproverAutoPass
			n.TimeoutHours = 24
		}))

		result, err := s.processor.Process(s.Ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait because user-3 still needs approval")

		tasks := s.QueryTasks(s.T(), instance.ID)

		var currentNodeTasks []approval.Task
		for _, t := range tasks {
			if t.NodeID == s.NodeID {
				currentNodeTasks = append(currentNodeTasks, t)
			}
		}

		s.Require().Len(currentNodeTasks, 3, "Should create 3 sequential tasks")

		// user-1 (sort_order=1): auto-passed
		s.Assert().Equal(approval.TaskApproved, currentNodeTasks[0].Status, "First task should be auto-passed")
		// user-2 (sort_order=2): activated then auto-passed
		s.Assert().Equal(approval.TaskApproved, currentNodeTasks[1].Status, "Second task should be auto-passed")
		// user-3 (sort_order=3): activated to pending
		s.Assert().Equal(approval.TaskPending, currentNodeTasks[2].Status, "Third task should be activated to pending")
		s.Require().NotNil(currentNodeTasks[2].Deadline, "Activated pending task should start timeout from activation")
	})

	s.Run("SequentialAllAutoPassedContinues", func() {
		defer s.CleanTransientData(s.T())

		prevNodeID := s.InsertPreviousApprovalNode(s.T(), "prev-approval-seq-all")
		instance := s.NewInstance(s.T(), "applicant-1")
		s.InsertApprovedTasks(s.T(), instance.ID, prevNodeID, []string{"user-1", "user-2"})
		s.InsertAssigneeConfig(s.T(), []string{"user-1", "user-2"})

		pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
			n.ApprovalMethod = approval.ApprovalSequential
			n.ConsecutiveApproverAction = approval.ConsecutiveApproverAutoPass
		}))

		result, err := s.processor.Process(s.Ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionContinue, result.Action, "Should continue when all sequential tasks are auto-passed")
	})
}

func (s *ApprovalProcessorTestSuite) TestDBError() {
	instance := s.NewInstance(s.T(), "applicant-1")
	s.InsertAssigneeConfig(s.T(), []string{"user-1"})

	canceledCtx, cancel := context.WithCancel(s.Ctx)
	cancel()

	pc := &engine.ProcessContext{
		DB:          s.DB,
		Instance:    instance,
		Node:        s.NewNode(),
		FormData:    approval.NewFormData(nil),
		ApplicantID: instance.ApplicantID,
		Registry:    s.Registry,
	}

	_, err := s.processor.Process(canceledCtx, pc)
	s.Require().Error(err, "Should return error when context is canceled")
}
