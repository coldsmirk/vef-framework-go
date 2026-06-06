package command_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &RollbackTaskTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// RollbackTaskTestSuite tests the RollbackTaskHandler.
type RollbackTaskTestSuite struct {
	suite.Suite

	ctx          context.Context
	db           orm.DB
	handler      cqrs.Handler[command.RollbackTaskCmd, cqrs.Unit]
	fixture      *MinimalFixture
	rollbackNode *approval.FlowNode
	targetNode   *approval.FlowNode

	instanceSeq int
}

func (s *RollbackTaskTestSuite) SetupSuite() {
	eng := buildTestEngine()
	taskSvc := service.NewTaskService()
	validSvc := service.NewValidationService(nil)
	s.handler = wrapWithBusAndDB(s.db, eventtest.NewFakeBus(), command.NewRollbackTaskHandler(s.db, taskSvc, service.NewInstanceService(nil), validSvc, eng))

	s.fixture = setupMinimalFixture(s.T(), s.ctx, s.db, "rollback")

	s.rollbackNode = &approval.FlowNode{
		FlowVersionID:        s.fixture.VersionID,
		Key:                  "rollback-node",
		Kind:                 approval.NodeApproval,
		Name:                 "Rollback Node",
		IsRollbackAllowed:    true,
		RollbackType:         approval.RollbackAny,
		RollbackDataStrategy: approval.RollbackDataClear,
	}
	_, err := s.db.NewInsert().Model(s.rollbackNode).Exec(s.ctx)
	s.Require().NoError(err, "Should create rollback node")

	s.targetNode = &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "rollback-target",
		Kind:          approval.NodeApproval,
		Name:          "Rollback Target Node",
	}
	_, err = s.db.NewInsert().Model(s.targetNode).Exec(s.ctx)
	s.Require().NoError(err, "Should create rollback target node")

	// Give the intermediate target an assignee so a rollback that re-enters it
	// through ProcessNode (the non-start persistence branch) can create a task
	// and leave the instance running.
	_, err = s.db.NewInsert().Model(&approval.FlowNodeAssignee{
		NodeID:    s.targetNode.ID,
		Kind:      approval.AssigneeUser,
		IDs:       []string{"target-approver"},
		SortOrder: 1,
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should seed rollback target node assignee")
}

func (s *RollbackTaskTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *RollbackTaskTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *RollbackTaskTestSuite) setupData(assigneeID string) (*approval.Instance, *approval.Task) {
	s.instanceSeq++
	instanceNo := fmt.Sprintf("RB-%03d", s.instanceSeq)

	currentNodeID := s.rollbackNode.ID
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Rollback Test",
		InstanceNo:    instanceNo,
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &currentNodeID,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should create running instance")

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     s.rollbackNode.ID,
		AssigneeID: assigneeID,
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should create pending task on rollback node")

	return inst, task
}

func (s *RollbackTaskTestSuite) TestRollbackTaskNotFound() {
	operator := approval.OperatorInfo{ID: "rollback-operator", Name: "Rollback Operator"}
	_, err := s.handler.Handle(s.ctx, command.RollbackTaskCmd{
		TaskID:       "non-existent",
		Operator:     operator,
		TargetNodeID: s.targetNode.ID,
		Caller:       approval.SystemCaller,
	})
	s.Require().Error(err, "Should fail when task does not exist")
	s.Assert().ErrorIs(err, shared.ErrTaskNotFound, "Should return task not found")
}

func (s *RollbackTaskTestSuite) TestRollbackTaskNotCurrentNode() {
	inst, task := s.setupData("rollback-operator")

	_, err := s.db.NewUpdate().
		Model((*approval.Instance)(nil)).
		Set("current_node_id", s.targetNode.ID).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(inst.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should move instance current node away from task node")

	operator := approval.OperatorInfo{ID: "rollback-operator", Name: "Rollback Operator"}
	_, err = s.handler.Handle(s.ctx, command.RollbackTaskCmd{
		TaskID:       task.ID,
		Operator:     operator,
		TargetNodeID: s.targetNode.ID,
		Opinion:      "rollback",
		Caller:       approval.SystemCaller,
	})
	s.Require().Error(err, "Should fail when rolling back a task not in current node")
	s.Assert().ErrorIs(err, shared.ErrTaskNotPending, "Should return task not pending for stale node task")
}

func (s *RollbackTaskTestSuite) TestRollbackTargetRequired() {
	s.Run("EmptyTarget", func() {
		_, task := s.setupData("rollback-empty")

		operator := approval.OperatorInfo{ID: "rollback-empty", Name: "Rollback Operator"}
		_, err := s.handler.Handle(s.ctx, command.RollbackTaskCmd{
			TaskID:       task.ID,
			Operator:     operator,
			TargetNodeID: "",
			Opinion:      "rollback",
			Caller:       approval.SystemCaller,
		})
		s.Require().Error(err, "Should fail when rollback target is empty")
		s.Assert().ErrorIs(err, shared.ErrInvalidRollbackTarget, "Should return invalid rollback target for empty input")
	})

	s.Run("BlankTarget", func() {
		_, task := s.setupData("rollback-blank")

		operator := approval.OperatorInfo{ID: "rollback-blank", Name: "Rollback Operator"}
		_, err := s.handler.Handle(s.ctx, command.RollbackTaskCmd{
			TaskID:       task.ID,
			Operator:     operator,
			TargetNodeID: "   ",
			Opinion:      "rollback",
			Caller:       approval.SystemCaller,
		})
		s.Require().Error(err, "Should fail when rollback target is blank")
		s.Assert().ErrorIs(err, shared.ErrInvalidRollbackTarget, "Should return invalid rollback target for blank input")
	})
}

func (s *RollbackTaskTestSuite) TestRollbackTargetShouldNotBeCurrentNode() {
	_, task := s.setupData("rollback-current-node")

	operator := approval.OperatorInfo{ID: "rollback-current-node", Name: "Rollback Operator"}
	_, err := s.handler.Handle(s.ctx, command.RollbackTaskCmd{
		TaskID:       task.ID,
		Operator:     operator,
		TargetNodeID: s.rollbackNode.ID,
		Opinion:      "rollback to current",
		Caller:       approval.SystemCaller,
	})
	s.Require().Error(err, "Should fail when rollback target is current node")
	s.Assert().ErrorIs(err, shared.ErrInvalidRollbackTarget, "Should return invalid rollback target when target equals current node")

	var reloaded approval.Task

	reloaded.ID = task.ID
	s.Require().NoError(
		s.db.NewSelect().Model(&reloaded).WherePK().Scan(s.ctx),
		"Should reload task after rollback target validation failure",
	)
	s.Assert().Equal(approval.TaskPending, reloaded.Status, "Task should remain pending when rollback target validation fails")
}

// seedStartNode inserts a start node into the fixture version that a rollback
// can return to without re-entering the engine (the NodeStart path returns the
// instance to the applicant).
func (s *RollbackTaskTestSuite) seedStartNode(key string) *approval.FlowNode {
	startNode := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           key,
		Kind:          approval.NodeStart,
		Name:          "Start",
	}
	_, err := s.db.NewInsert().Model(startNode).Exec(s.ctx)
	s.Require().NoError(err, "Should create start node")

	return startNode
}

// TestRollbackDataClearWipesFormData pins the F1 fix: a node configured with
// RollbackDataStrategy=clear must wipe the instance form data on rollback.
// Before the fix the clear branch was unimplemented, so stale data survived.
func (s *RollbackTaskTestSuite) TestRollbackDataClearWipesFormData() {
	startNode := s.seedStartNode("clear-start")

	inst, task := s.setupData("rollback-clear-op")
	_, err := s.db.NewUpdate().
		Model((*approval.Instance)(nil)).
		Set("form_data", map[string]any{"amount": 100, "reason": "lunch"}).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(inst.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should seed instance form data")

	// s.rollbackNode is configured RollbackDataClear + RollbackAny.
	operator := approval.OperatorInfo{ID: "rollback-clear-op", Name: "Operator"}
	_, err = s.handler.Handle(s.ctx, command.RollbackTaskCmd{
		TaskID:       task.ID,
		Operator:     operator,
		TargetNodeID: startNode.ID,
		Opinion:      "rollback and clear",
		Caller:       approval.SystemCaller,
	})
	s.Require().NoError(err, "Rollback to start with clear strategy should succeed")

	var reloaded approval.Instance

	reloaded.ID = inst.ID
	s.Require().NoError(s.db.NewSelect().Model(&reloaded).WherePK().Scan(s.ctx), "Should reload instance after rollback")
	s.Assert().Equal(approval.InstanceReturned, reloaded.Status, "Instance should be returned to the applicant")
	s.Assert().Empty(reloaded.FormData, "RollbackDataClear must wipe instance form data, not retain it")
}

// TestRollbackDataKeepRestoresSnapshot guards the complementary branch: keep
// restores the snapshot captured at the target node.
func (s *RollbackTaskTestSuite) TestRollbackDataKeepRestoresSnapshot() {
	startNode := s.seedStartNode("keep-start")

	keepNode := &approval.FlowNode{
		FlowVersionID:        s.fixture.VersionID,
		Key:                  "rollback-keep-node",
		Kind:                 approval.NodeApproval,
		Name:                 "Rollback Keep Node",
		IsRollbackAllowed:    true,
		RollbackType:         approval.RollbackAny,
		RollbackDataStrategy: approval.RollbackDataKeep,
	}
	_, err := s.db.NewInsert().Model(keepNode).Exec(s.ctx)
	s.Require().NoError(err, "Should create keep node")

	s.instanceSeq++
	currentNodeID := keepNode.ID
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Rollback Keep Test",
		InstanceNo:    fmt.Sprintf("RB-KEEP-%03d", s.instanceSeq),
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &currentNodeID,
		FormData:      map[string]any{"current": "value"},
	}
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should create running instance")

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     keepNode.ID,
		AssigneeID: "rollback-keep-op",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should create pending task")

	snapshot := &approval.FormSnapshot{
		InstanceID: inst.ID,
		NodeID:     startNode.ID,
		FormData:   map[string]any{"restored": "from-snapshot"},
	}
	_, err = s.db.NewInsert().Model(snapshot).Exec(s.ctx)
	s.Require().NoError(err, "Should seed form snapshot for target node")

	operator := approval.OperatorInfo{ID: "rollback-keep-op", Name: "Operator"}
	_, err = s.handler.Handle(s.ctx, command.RollbackTaskCmd{
		TaskID:       task.ID,
		Operator:     operator,
		TargetNodeID: startNode.ID,
		Opinion:      "rollback and keep",
		Caller:       approval.SystemCaller,
	})
	s.Require().NoError(err, "Rollback to start with keep strategy should succeed")

	var reloaded approval.Instance

	reloaded.ID = inst.ID
	s.Require().NoError(s.db.NewSelect().Model(&reloaded).WherePK().Scan(s.ctx), "Should reload instance after rollback")
	s.Assert().Equal("from-snapshot", reloaded.FormData["restored"], "RollbackDataKeep must restore the target node's form snapshot")
}

// TestRollbackToIntermediateNodeClearsFormData exercises the intermediate-node
// persistence branch (a column-scoped UPDATE + ProcessNode, distinct from the
// NodeStart "return to applicant" path that the other clear test covers): the
// clear strategy must persist NULL form data there too, and the instance keeps
// running at the target node.
func (s *RollbackTaskTestSuite) TestRollbackToIntermediateNodeClearsFormData() {
	inst, task := s.setupData("rollback-clear-mid-op")
	_, err := s.db.NewUpdate().
		Model((*approval.Instance)(nil)).
		Set("form_data", map[string]any{"amount": 100, "reason": "lunch"}).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(inst.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should seed instance form data")

	// s.rollbackNode is RollbackDataClear; s.targetNode is an intermediate
	// approval node (not a start node), so rollback takes the else branch.
	operator := approval.OperatorInfo{ID: "rollback-clear-mid-op", Name: "Operator"}
	_, err = s.handler.Handle(s.ctx, command.RollbackTaskCmd{
		TaskID:       task.ID,
		Operator:     operator,
		TargetNodeID: s.targetNode.ID,
		Opinion:      "rollback to intermediate and clear",
		Caller:       approval.SystemCaller,
	})
	s.Require().NoError(err, "Rollback to an intermediate node with clear strategy should succeed")

	var reloaded approval.Instance

	reloaded.ID = inst.ID
	s.Require().NoError(s.db.NewSelect().Model(&reloaded).WherePK().Scan(s.ctx), "Should reload instance after rollback")
	s.Assert().Equal(approval.InstanceRunning, reloaded.Status, "Rollback to an intermediate node keeps the instance running")
	s.Require().NotNil(reloaded.CurrentNodeID, "Current node should be set after rollback")
	s.Assert().Equal(s.targetNode.ID, *reloaded.CurrentNodeID, "Instance should move to the rollback target node")
	s.Assert().Empty(reloaded.FormData, "RollbackDataClear must persist NULL form data on the intermediate-node path too")
}

// TestRollbackToIntermediateNodeKeepsSnapshot is the keep counterpart on the
// intermediate-node branch: the target node's snapshot must be restored and
// persisted while the instance keeps running at that node.
func (s *RollbackTaskTestSuite) TestRollbackToIntermediateNodeKeepsSnapshot() {
	keepNode := &approval.FlowNode{
		FlowVersionID:        s.fixture.VersionID,
		Key:                  "rollback-keep-mid-node",
		Kind:                 approval.NodeApproval,
		Name:                 "Rollback Keep Mid Node",
		IsRollbackAllowed:    true,
		RollbackType:         approval.RollbackAny,
		RollbackDataStrategy: approval.RollbackDataKeep,
	}
	_, err := s.db.NewInsert().Model(keepNode).Exec(s.ctx)
	s.Require().NoError(err, "Should create keep node")

	s.instanceSeq++
	currentNodeID := keepNode.ID
	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Rollback Keep Intermediate Test",
		InstanceNo:    fmt.Sprintf("RB-KEEP-MID-%03d", s.instanceSeq),
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &currentNodeID,
		FormData:      map[string]any{"current": "value"},
	}
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should create running instance")

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     keepNode.ID,
		AssigneeID: "rollback-keep-mid-op",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should create pending task")

	snapshot := &approval.FormSnapshot{
		InstanceID: inst.ID,
		NodeID:     s.targetNode.ID,
		FormData:   map[string]any{"restored": "from-snapshot"},
	}
	_, err = s.db.NewInsert().Model(snapshot).Exec(s.ctx)
	s.Require().NoError(err, "Should seed form snapshot for the intermediate target node")

	operator := approval.OperatorInfo{ID: "rollback-keep-mid-op", Name: "Operator"}
	_, err = s.handler.Handle(s.ctx, command.RollbackTaskCmd{
		TaskID:       task.ID,
		Operator:     operator,
		TargetNodeID: s.targetNode.ID,
		Opinion:      "rollback to intermediate and keep",
		Caller:       approval.SystemCaller,
	})
	s.Require().NoError(err, "Rollback to an intermediate node with keep strategy should succeed")

	var reloaded approval.Instance

	reloaded.ID = inst.ID
	s.Require().NoError(s.db.NewSelect().Model(&reloaded).WherePK().Scan(s.ctx), "Should reload instance after rollback")
	s.Assert().Equal(approval.InstanceRunning, reloaded.Status, "Rollback to an intermediate node keeps the instance running")
	s.Assert().Equal("from-snapshot", reloaded.FormData["restored"], "RollbackDataKeep must restore the snapshot on the intermediate-node path too")
}

// TestRollbackClearIsNotWedgedByOversizeFormData proves the size-cap delta and
// rollback-clear compose: an instance whose stored form data is already over the
// cap can still be rolled back (the no-op-merge passes PrepareOperation's growth
// check) and the clear then wipes it — it is not wedged.
func (s *RollbackTaskTestSuite) TestRollbackClearIsNotWedgedByOversizeFormData() {
	startNode := s.seedStartNode("oversize-clear-start")

	inst, task := s.setupData("rollback-oversize-op")
	_, err := s.db.NewUpdate().
		Model((*approval.Instance)(nil)).
		Set("form_data", map[string]any{"blob": strings.Repeat("x", 70*1024)}).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(inst.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should seed an over-cap instance form payload")

	operator := approval.OperatorInfo{ID: "rollback-oversize-op", Name: "Operator"}
	_, err = s.handler.Handle(s.ctx, command.RollbackTaskCmd{
		TaskID:       task.ID,
		Operator:     operator,
		TargetNodeID: startNode.ID,
		Opinion:      "rollback an oversize instance and clear",
		Caller:       approval.SystemCaller,
	})
	s.Require().NoError(err, "Rollback-clear must not be wedged by a pre-existing oversize instance")

	var reloaded approval.Instance

	reloaded.ID = inst.ID
	s.Require().NoError(s.db.NewSelect().Model(&reloaded).WherePK().Scan(s.ctx), "Should reload instance after rollback")
	s.Assert().Empty(reloaded.FormData, "RollbackDataClear must still wipe the oversize form data")
}
