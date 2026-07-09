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
	"github.com/coldsmirk/vef-framework-go/result"
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

	eng := buildTestEngine(s.db)
	taskSvc, nodeSvc, validSvc := buildTestServices(eng)

	s.handler = wrapWithBusAndDB(s.db, eventtest.NewFakeBus(), command.NewRejectTaskHandler(s.db, taskSvc, nodeSvc, validSvc, nil))
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

	operator := approval.UserInfo{ID: "rejector-1", Name: "Rejector"}
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
	operator := approval.UserInfo{ID: "rejector-1", Name: "Rejector"}
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

	operator := approval.UserInfo{ID: "wrong-user", Name: "Wrong"}
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

	operator := approval.UserInfo{ID: "rejector-current", Name: "Rejector"}
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
	// blob is an unconstrained text field, so value validation passes and the
	// size guard is what rejects the oversized payload.
	setPublishedFormFields(s.T(), s.ctx, s.db, s.fixture.VersionID, []approval.FormFieldDefinition{
		{Key: "blob", Kind: approval.FieldTextarea, Label: "Blob"},
	})

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
		VisitID:    ensureActiveVisit(s.T(), s.ctx, s.db, "default", inst.ID, node.ID).ID,
		AssigneeID: "rejector-oversize",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should create pending task")

	_, err = s.handler.Handle(s.ctx, command.RejectTaskCmd{
		TaskID:   task.ID,
		Operator: approval.UserInfo{ID: "rejector-oversize", Name: "Rejector"},
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

// TestRejectAllowsEmptyRequiredField proves reject is exempt from the required-
// permission must-fill rule: a rejection must never be blocked by an unfilled
// field, even one the node marks required.
func (s *RejectTaskTestSuite) TestRejectAllowsEmptyRequiredField() {
	fields := []approval.FormFieldDefinition{{Key: "note", Kind: approval.FieldInput, Label: "Note"}}
	required := map[string]approval.Permission{"note": approval.PermissionRequired}
	inst, task := setupFormFieldInstance(s.T(), s.ctx, s.db, s.fixture, approval.NodeApproval, required, fields, nil, "reject-req-empty")

	_, err := s.handler.Handle(s.ctx, command.RejectTaskCmd{
		TaskID:   task.ID,
		Operator: approval.UserInfo{ID: "reject-req-empty", Name: "Rejector"},
		Opinion:  "not acceptable",
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "reject must succeed even when a required-permission field is empty")

	var reloadedTask approval.Task

	reloadedTask.ID = task.ID
	s.Require().NoError(s.db.NewSelect().Model(&reloadedTask).WherePK().Scan(s.ctx), "Should reload task")
	s.Assert().Equal(approval.TaskRejected, reloadedTask.Status, "task should be rejected")

	var reloadedInst approval.Instance

	reloadedInst.ID = inst.ID
	s.Require().NoError(s.db.NewSelect().Model(&reloadedInst).WherePK().Scan(s.ctx), "Should reload instance")
	s.Assert().Equal(approval.InstanceRejected, reloadedInst.Status, "PassAll node with one rejection rejects the instance")
}

// TestRejectRejectsInvalidEditableValue proves the editable-subset value
// validation runs on the reject path too: a submitted editable value that
// violates its schema rule is rejected before any state change.
func (s *RejectTaskTestSuite) TestRejectRejectsInvalidEditableValue() {
	fields := []approval.FormFieldDefinition{
		{Key: "amount", Kind: approval.FieldNumber, Label: "Amount", Validation: &approval.ValidationRule{Min: new(10.0)}},
	}
	editable := map[string]approval.Permission{"amount": approval.PermissionEditable}
	_, task := setupFormFieldInstance(s.T(), s.ctx, s.db, s.fixture, approval.NodeApproval, editable, fields, nil, "reject-invalid-value")

	_, err := s.handler.Handle(s.ctx, command.RejectTaskCmd{
		TaskID:   task.ID,
		Operator: approval.UserInfo{ID: "reject-invalid-value", Name: "Rejector"},
		Opinion:  "reject with a bad edit",
		FormData: map[string]any{"amount": 5},
		Caller:   approval.SystemCaller,
	})

	var re result.Error
	s.Require().ErrorAs(err, &re, "an editable value below its schema minimum must fail reject")
	s.Assert().Equal(shared.ErrCodeFormValidationFailed, re.Code, "should be a form validation error")

	var reloaded approval.Task

	reloaded.ID = task.ID
	s.Require().NoError(s.db.NewSelect().Model(&reloaded).WherePK().Scan(s.ctx), "Should reload task")
	s.Assert().Equal(approval.TaskPending, reloaded.Status, "task must stay pending after a rejected invalid value")
}
