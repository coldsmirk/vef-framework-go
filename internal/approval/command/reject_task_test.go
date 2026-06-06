package command_test

import (
	"context"
	"strings"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &RejectTaskTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// RejectTaskTestSuite tests the RejectTaskHandler.
type RejectTaskTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler cqrs.Handler[command.RejectTaskCmd, cqrs.Unit]
	fixture *FlowFixture
}

func (s *RejectTaskTestSuite) SetupSuite() {
	s.fixture = setupApprovalFlow(s.T(), s.ctx, s.db)

	eng := buildTestEngine()
	taskSvc, nodeSvc, validSvc := buildTestServices(eng)

	s.handler = wrapWithBusAndDB(s.db, eventtest.NewFakeBus(), command.NewRejectTaskHandler(s.db, taskSvc, nodeSvc, validSvc))
}

func (s *RejectTaskTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *RejectTaskTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *RejectTaskTestSuite) newRunningInstance(assigneeID string) (*approval.Instance, *approval.Task) {
	return setupRunningInstance(s.T(), s.ctx, s.db, s.fixture, assigneeID)
}

func (s *RejectTaskTestSuite) TestRejectSuccess() {
	inst, task := s.newRunningInstance("rejector-1")

	operator := approval.OperatorInfo{ID: "rejector-1", Name: "Rejector"}
	_, err := s.handler.Handle(s.ctx, command.RejectTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
		Opinion:  "Not acceptable",
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "Should reject task without error")

	// Verify task status
	var updated approval.Task

	updated.ID = task.ID
	s.Require().NoError(s.db.NewSelect().Model(&updated).WherePK().Scan(s.ctx), "TestRejectSuccess should complete without error")
	s.Assert().Equal(approval.TaskRejected, updated.Status, "Task should be rejected")

	// Verify instance status (with PassAll rule, one rejection rejects instance)
	var updatedInst approval.Instance

	updatedInst.ID = inst.ID
	s.Require().NoError(s.db.NewSelect().Model(&updatedInst).WherePK().Scan(s.ctx), "TestRejectSuccess should complete without error")
	s.Assert().Equal(approval.InstanceRejected, updatedInst.Status, "Instance should be rejected")

	// Verify action log
	var logs []approval.ActionLog
	s.Require().NoError(s.db.NewSelect().Model(&logs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
		Scan(s.ctx), "TestRejectSuccess should complete without error")

	found := false
	for _, log := range logs {
		if log.Action == approval.ActionReject {
			found = true

			s.Assert().Equal("rejector-1", log.OperatorID, "TestRejectSuccess should match expected value")
		}
	}

	s.Assert().True(found, "Should have a reject action log")
}

func (s *RejectTaskTestSuite) TestRejectTaskNotFound() {
	operator := approval.OperatorInfo{ID: "rejector-1", Name: "Rejector"}
	_, err := s.handler.Handle(s.ctx, command.RejectTaskCmd{
		TaskID:   "non-existent",
		Operator: operator,
		Caller:   approval.SystemCaller,
	})
	s.Require().Error(err, "TestRejectTaskNotFound should return an error")
	s.Assert().ErrorIs(err, shared.ErrTaskNotFound, "Should return expected error")
}

func (s *RejectTaskTestSuite) TestRejectNotAssignee() {
	_, task := s.newRunningInstance("rejector-1")

	operator := approval.OperatorInfo{ID: "wrong-user", Name: "Wrong"}
	_, err := s.handler.Handle(s.ctx, command.RejectTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
		Caller:   approval.SystemCaller,
	})
	s.Require().Error(err, "TestRejectNotAssignee should return an error")
	s.Assert().ErrorIs(err, shared.ErrNotAssignee, "Should return expected error")
}

func (s *RejectTaskTestSuite) TestRejectTaskNotCurrentNode() {
	inst, task := s.newRunningInstance("rejector-current")

	otherNode := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "reject-other-current-node",
		Kind:          approval.NodeApproval,
		Name:          "Reject Other Current Node",
	}
	_, err := s.db.NewInsert().Model(otherNode).Exec(s.ctx)
	s.Require().NoError(err, "Should create another node")

	_, err = s.db.NewUpdate().
		Model((*approval.Instance)(nil)).
		Set("current_node_id", otherNode.ID).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(inst.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should move instance current node away from task node")

	operator := approval.OperatorInfo{ID: "rejector-current", Name: "Rejector"}
	_, err = s.handler.Handle(s.ctx, command.RejectTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
		Opinion:  "rejected",
		Caller:   approval.SystemCaller,
	})
	s.Require().Error(err, "Should fail when rejecting a task not in current node")
	s.Assert().ErrorIs(err, shared.ErrTaskNotPending, "Should return task not pending for stale node task")
}

// TestRejectEnforcesFormDataSizeCap proves the form-data size cap is enforced on
// the reject path too, not only approve: every task action shares the
// PrepareOperation chokepoint, so growing the instance past the cap via reject's
// form data is rejected before any state change.
func (s *RejectTaskTestSuite) TestRejectEnforcesFormDataSizeCap() {
	node := &approval.FlowNode{
		FlowVersionID:    s.fixture.VersionID,
		Key:              "reject-oversize-node",
		Kind:             approval.NodeApproval,
		Name:             "Reject Oversize Node",
		PassRule:         approval.PassAll,
		FieldPermissions: map[string]approval.Permission{"blob": approval.PermissionEditable},
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should create node with an editable field")

	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Reject Oversize Test",
		InstanceNo:    "RJ-OVERSIZE-001",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
		CurrentNodeID: &node.ID,
	}
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should create running instance")

	task := &approval.Task{
		TenantID:   "default",
		InstanceID: inst.ID,
		NodeID:     node.ID,
		AssigneeID: "rejector-oversize",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should create pending task")

	_, err = s.handler.Handle(s.ctx, command.RejectTaskCmd{
		TaskID:   task.ID,
		Operator: approval.OperatorInfo{ID: "rejector-oversize", Name: "Rejector"},
		Opinion:  "reject with oversized form",
		FormData: map[string]any{"blob": strings.Repeat("x", 70*1024)},
		Caller:   approval.SystemCaller,
	})
	s.Require().ErrorIs(err, shared.ErrFormDataTooLarge, "Reject must enforce the form-data size cap via PrepareOperation")

	var reloaded approval.Task

	reloaded.ID = task.ID
	s.Require().NoError(s.db.NewSelect().Model(&reloaded).WherePK().Scan(s.ctx), "Should reload task")
	s.Assert().Equal(approval.TaskPending, reloaded.Status, "Task must stay pending — the size guard runs before the reject")
}
