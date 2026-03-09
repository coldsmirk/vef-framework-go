package engine_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/engine"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &HandleProcessorTestSuite{
			ProcessorTestBase: ProcessorTestBase{
				Ctx: env.Ctx,
				DB:  env.DB,
			},
		}
	})
}

// HandleProcessorTestSuite tests HandleProcessor with a real database.
type HandleProcessorTestSuite struct {
	suite.Suite
	ProcessorTestBase

	processor *engine.HandleProcessor
}

func (s *HandleProcessorTestSuite) SetupSuite() {
	s.InitRegistry()
	s.processor = engine.NewHandleProcessor(nil)
	s.InitFKChain(s.T(), "handle-test", approval.NodeHandle, "Handle")
}

func (s *HandleProcessorTestSuite) TearDownTest() {
	s.CleanTransientData(s.T())
}

// --- Tests ---

func (s *HandleProcessorTestSuite) TestNodeKind() {
	s.Assert().Equal(approval.NodeHandle, s.processor.NodeKind(), "Should return NodeHandle kind")
}

func (s *HandleProcessorTestSuite) TestProcessWithAssignees() {
	instance := s.NewInstance(s.T(), "applicant-1")
	s.InsertAssigneeConfig(s.T(), []string{"user-1", "user-2"})

	pc := s.NewProcessContext(instance, s.NewNode())

	result, err := s.processor.Process(s.Ctx, pc)
	s.Require().NoError(err, "Should process without error")
	s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait for handle tasks")

	tasks := s.QueryTasks(s.T(), instance.ID)
	s.Require().Len(tasks, 2, "Should create 2 tasks")

	assigneeIDs := make([]string, len(tasks))
	for i, task := range tasks {
		assigneeIDs[i] = task.AssigneeID
		s.Assert().Equal(instance.ID, task.InstanceID, "Task should reference instance")
		s.Assert().Equal(s.NodeID, task.NodeID, "Task should reference node")
		s.Assert().Equal(approval.TaskPending, task.Status, "Task should be pending")
		s.Assert().Equal(0, task.SortOrder, "Handle tasks should have sort order 0")
	}

	s.Assert().ElementsMatch([]string{"user-1", "user-2"}, assigneeIDs, "Should create tasks for all assignees")
}

func (s *HandleProcessorTestSuite) TestProcessEmptyAssignee() {
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

	s.Run("TransferAdmin", func() {
		defer s.CleanTransientData(s.T())

		instance := s.NewInstance(s.T(), "applicant-1")

		pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
			n.EmptyAssigneeAction = approval.EmptyAssigneeTransferAdmin
			n.AdminUserIDs = []string{"admin-1"}
		}))

		result, err := s.processor.Process(s.Ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait when transferred to admin")

		tasks := s.QueryTasks(s.T(), instance.ID)
		s.Require().Len(tasks, 1, "Should create one task for admin")
		s.Assert().Equal("admin-1", tasks[0].AssigneeID, "Task should be assigned to admin")
	})

	s.Run("TransferSpecified", func() {
		defer s.CleanTransientData(s.T())

		instance := s.NewInstance(s.T(), "applicant-1")

		pc := s.NewProcessContext(instance, s.NewNode(func(n *approval.FlowNode) {
			n.EmptyAssigneeAction = approval.EmptyAssigneeTransferSpecified
			n.FallbackUserIDs = []string{"fallback-1"}
		}))

		result, err := s.processor.Process(s.Ctx, pc)
		s.Require().NoError(err, "Should process without error")
		s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait when transferred to specified user")

		tasks := s.QueryTasks(s.T(), instance.ID)
		s.Require().Len(tasks, 1, "Should create one task for fallback user")
		s.Assert().Equal("fallback-1", tasks[0].AssigneeID, "Task should be assigned to fallback user")
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

func (s *HandleProcessorTestSuite) TestProcessFormSnapshot() {
	instance := s.NewInstance(s.T(), "applicant-1")
	instance.FormData = map[string]any{"title": "test document"}
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

func (s *HandleProcessorTestSuite) TestProcessDeduplication() {
	instance := s.NewInstance(s.T(), "applicant-1")
	s.InsertAssigneeConfig(s.T(), []string{"user-1", "user-1", "user-2"})

	pc := s.NewProcessContext(instance, s.NewNode())

	result, err := s.processor.Process(s.Ctx, pc)
	s.Require().NoError(err, "Should process without error")
	s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait for handle tasks")

	tasks := s.QueryTasks(s.T(), instance.ID)
	s.Require().Len(tasks, 2, "Should create 2 tasks after deduplication")

	assigneeIDs := make([]string, len(tasks))
	for i, task := range tasks {
		assigneeIDs[i] = task.AssigneeID
	}

	s.Assert().ElementsMatch([]string{"user-1", "user-2"}, assigneeIDs, "Should deduplicate assignees")
}

func (s *HandleProcessorTestSuite) TestProcessMultipleAssigneeConfigs() {
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
		IDs:       []string{"user-2"},
		SortOrder: 2,
	}
	_, err = s.DB.NewInsert().Model(cfg2).Exec(s.Ctx)
	s.Require().NoError(err, "Should insert second assignee config")

	pc := s.NewProcessContext(instance, s.NewNode())

	result, err := s.processor.Process(s.Ctx, pc)
	s.Require().NoError(err, "Should process without error")
	s.Assert().Equal(engine.NodeActionWait, result.Action, "Should wait for handle tasks")

	tasks := s.QueryTasks(s.T(), instance.ID)
	s.Require().Len(tasks, 2, "Should create tasks from all assignee configs")

	assigneeIDs := []string{tasks[0].AssigneeID, tasks[1].AssigneeID}
	s.Assert().ElementsMatch([]string{"user-1", "user-2"}, assigneeIDs, "Should resolve assignees from all configs")
}

func (s *HandleProcessorTestSuite) TestDBError() {
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
