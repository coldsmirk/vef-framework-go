package command_test

import (
	"context"
	"strings"

	"github.com/coldsmirk/go-collections"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/cqrs"
	"github.com/coldsmirk/vef-framework-go/internal/eventtest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &ApproveTaskTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// ApproveTaskTestSuite tests the ApproveTaskHandler.
type ApproveTaskTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler cqrs.Handler[command.ApproveTaskCmd, cqrs.Unit]
	fixture *FlowFixture
}

func (s *ApproveTaskTestSuite) SetupSuite() {
	s.fixture = setupApprovalFlow(s.T(), s.ctx, s.db)

	eng := buildTestEngine(s.db)
	taskSvc, nodeSvc, validSvc := buildTestServices(eng)

	s.handler = wrapWithBusAndDB(s.db, eventtest.NewFakeBus(), command.NewApproveTaskHandler(s.db, taskSvc, nodeSvc, validSvc, nil))
}

func (s *ApproveTaskTestSuite) TearDownTest() {
	cleanRuntimeData(s.ctx, s.db)
}

func (s *ApproveTaskTestSuite) TearDownSuite() {
	cleanAllApprovalData(s.ctx, s.db)
}

func (s *ApproveTaskTestSuite) newRunningInstance(assigneeID string) (*approval.Instance, *approval.Task) {
	return setupRunningInstance(s.T(), s.ctx, s.db, s.fixture, assigneeID)
}

func (s *ApproveTaskTestSuite) TestApproveSuccess() {
	inst, task := s.newRunningInstance("approver-1")

	operator := approval.UserInfo{ID: "approver-1", Name: "Approver"}
	_, err := s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:      task.ID,
		Operator:    operator,
		Opinion:     "Approved",
		Attachments: []string{"signed.pdf"},
		Caller:      approval.SystemCaller,
	})
	s.Require().NoError(err, "Should approve task without error")

	// Verify task status
	var updated approval.Task

	updated.ID = task.ID
	s.Require().NoError(s.db.NewSelect().Model(&updated).WherePK().Scan(s.ctx), "TestApproveSuccess should complete without error")
	s.Assert().Equal(approval.TaskApproved, updated.Status, "Task should be approved")

	// Verify action log
	var logs []approval.ActionLog
	s.Require().NoError(s.db.NewSelect().Model(&logs).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
		Scan(s.ctx), "TestApproveSuccess should complete without error")
	s.Assert().GreaterOrEqual(len(logs), 1, "Should have at least 1 action log")

	found := false
	for _, log := range logs {
		if log.Action == approval.ActionApprove {
			found = true

			s.Assert().Equal("approver-1", log.OperatorID, "TestApproveSuccess should match expected value")
			s.Assert().Equal([]string{"signed.pdf"}, log.Attachments, "Approve log should persist the attachments")
		}
	}

	s.Assert().True(found, "Should have an approve action log")
}

func (s *ApproveTaskTestSuite) TestApproveTaskNotFound() {
	operator := approval.UserInfo{ID: "approver-1", Name: "Approver"}
	_, err := s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   "non-existent",
		Operator: operator,
		Caller:   approval.SystemCaller,
	})
	s.Require().Error(err, "TestApproveTaskNotFound should return an error")
	s.Assert().ErrorIs(err, approval.ErrTaskNotFound, "Should return expected error")
}

func (s *ApproveTaskTestSuite) TestApproveNotAssignee() {
	_, task := s.newRunningInstance("approver-1")

	operator := approval.UserInfo{ID: "wrong-user", Name: "Wrong"}
	_, err := s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
		Caller:   approval.SystemCaller,
	})
	s.Require().Error(err, "TestApproveNotAssignee should return an error")
	s.Assert().ErrorIs(err, approval.ErrNotAssignee, "Should return expected error")
}

func (s *ApproveTaskTestSuite) TestApproveAlreadyCompleted() {
	_, task := s.newRunningInstance("approver-1")

	// Set task to already approved
	_, err := s.db.NewUpdate().
		Model((*approval.Task)(nil)).
		Set("status", approval.TaskApproved).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "TestApproveAlreadyCompleted should complete without error")

	operator := approval.UserInfo{ID: "approver-1", Name: "Approver"}
	_, err = s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
		Caller:   approval.SystemCaller,
	})
	s.Require().Error(err, "TestApproveAlreadyCompleted should return an error")
	s.Assert().ErrorIs(err, approval.ErrTaskNotPending, "Should return expected error")
}

func (s *ApproveTaskTestSuite) TestApproveTaskNotCurrentNode() {
	inst, task := s.newRunningInstance("approver-current")

	otherNode := &approval.FlowNode{
		FlowVersionID: s.fixture.VersionID,
		Key:           "approve-other-current-node",
		Kind:          approval.NodeApproval,
		Name:          "Approve Other Current Node",
	}
	_, err := s.db.NewInsert().Model(otherNode).Exec(s.ctx)
	s.Require().NoError(err, "Should create another node")

	_, err = s.db.NewUpdate().
		Model((*approval.Instance)(nil)).
		Set("current_node_id", otherNode.ID).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(inst.ID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should move instance current node away from task node")

	operator := approval.UserInfo{ID: "approver-current", Name: "Approver"}
	_, err = s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
		Opinion:  "approved",
		Caller:   approval.SystemCaller,
	})
	s.Require().Error(err, "Should fail when approving a task not in current node")
	s.Assert().ErrorIs(err, approval.ErrTaskNotPending, "Should return task not pending for stale node task")
}

func (s *ApproveTaskTestSuite) TestApproveShouldResolveCCFromFormField() {
	inst, task := s.newRunningInstance("approver-cc-form")

	ccField := "ccUsers"
	// The approver submits ccUsers as an editable field, so it must be part of
	// the version's form schema or PrepareOperation rejects it as undefined.
	setPublishedFormFields(s.T(), s.ctx, s.db, s.fixture.VersionID, []approval.FormFieldDefinition{
		{Key: ccField, Kind: approval.FieldSelect, Label: "CC Users"},
	})

	_, err := s.db.NewInsert().Model(&approval.FlowNodeCC{
		NodeID:    task.NodeID,
		Kind:      approval.CCFormField,
		FormField: &ccField,
		Timing:    approval.CCTimingAlways,
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should insert form-field CC config")

	// Ensure the ccUsers form field is editable so MergeFormData passes it through.
	_, err = s.db.NewUpdate().
		Model((*approval.FlowNode)(nil)).
		Set("field_permissions", map[string]approval.Permission{
			ccField: approval.PermissionEditable,
		}).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(task.NodeID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should update node field permissions")

	operator := approval.UserInfo{ID: "approver-cc-form", Name: "Approver"}
	_, err = s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
		Opinion:  "Approved",
		FormData: map[string]any{
			ccField: []string{"cc-user-2", "cc-user-3"},
		},
		Caller: approval.SystemCaller,
	})
	s.Require().NoError(err, "Should approve task with form-field CC data")

	var records []approval.CCRecord
	s.Require().NoError(s.db.NewSelect().
		Model(&records).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("instance_id", inst.ID) }).
		Scan(s.ctx), "TestApproveShouldResolveCCFromFormField should complete without error")

	userIDs := make([]string, len(records))
	for i, record := range records {
		userIDs[i] = record.CCUserID
	}

	userSet := collections.NewHashSetFrom(userIDs...)

	s.Assert().True(userSet.Contains("cc-user-1"), "Should keep existing static CC recipient")
	s.Assert().True(userSet.Contains("cc-user-2"), "Should include CC recipient resolved from form field")
	s.Assert().True(userSet.Contains("cc-user-3"), "Should include CC recipient resolved from form field")
}

// TestApproveRejectsOversizedFormData pins the F4 fix: the 64 KiB form-data cap
// is re-checked on the task-action path, so an approver cannot grow the
// instance form past the limit one editable field at a time.
func (s *ApproveTaskTestSuite) TestApproveRejectsOversizedFormData() {
	// blob is an unconstrained text field, so value validation passes and the
	// size guard — not schema validation — is what rejects the oversized payload.
	setPublishedFormFields(s.T(), s.ctx, s.db, s.fixture.VersionID, []approval.FormFieldDefinition{
		{Key: "blob", Kind: approval.FieldTextarea, Label: "Blob"},
	})

	node := &approval.FlowNode{
		FlowVersionID:    s.fixture.VersionID,
		Key:              "approve-oversized-node",
		Kind:             approval.NodeApproval,
		Name:             "Oversized Form Node",
		PassRule:         approval.PassAll,
		FieldPermissions: map[string]approval.Permission{"blob": approval.PermissionEditable},
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should create node with an editable field")

	inst := &approval.Instance{
		TenantID:      "default",
		FlowID:        s.fixture.FlowID,
		FlowVersionID: s.fixture.VersionID,
		Title:         "Oversized Form Test",
		InstanceNo:    "APV-OVERSIZE-001",
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
		AssigneeID: "approver-oversize",
		SortOrder:  1,
		Status:     approval.TaskPending,
	}
	_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
	s.Require().NoError(err, "Should create pending task")

	// 70 KiB of editable-field content exceeds the 64 KiB default cap
	// (config.DefaultFormDataMaxBytes) these fixtures build the services with.
	oversized := strings.Repeat("x", 70*1024)

	operator := approval.UserInfo{ID: "approver-oversize", Name: "Approver"}
	_, err = s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   task.ID,
		Operator: operator,
		FormData: map[string]any{"blob": oversized},
		Caller:   approval.SystemCaller,
	})
	s.Require().Error(err, "Approving with oversized form data should fail")
	s.Assert().ErrorIs(err, approval.ErrFormDataTooLarge, "Should reject form data exceeding the size cap on the approve path")

	var reloaded approval.Task

	reloaded.ID = task.ID
	s.Require().NoError(s.db.NewSelect().Model(&reloaded).WherePK().Scan(s.ctx), "Should reload task after rejection")
	s.Assert().Equal(approval.TaskPending, reloaded.Status, "Task must stay pending — the size guard runs before any state change")
}

// TestApproveDoesNotWedgeAlreadyOversizeInstance pins the delta-based size cap:
// an instance whose stored form data already exceeds the cap stays actionable
// (a no-op approval succeeds), while an action that grows it further is still
// rejected. Before the delta fix every action on a pre-existing oversize
// instance hard-failed, wedging the instance.
func (s *ApproveTaskTestSuite) TestApproveDoesNotWedgeAlreadyOversizeInstance() {
	// blob is an unconstrained text field, so the delta size guard — not schema
	// validation — governs the drip-feed-growth subtest.
	setPublishedFormFields(s.T(), s.ctx, s.db, s.fixture.VersionID, []approval.FormFieldDefinition{
		{Key: "blob", Kind: approval.FieldTextarea, Label: "Blob"},
	})

	node := &approval.FlowNode{
		FlowVersionID:    s.fixture.VersionID,
		Key:              "approve-oversize-noop-node",
		Kind:             approval.NodeApproval,
		Name:             "Oversize No-op Node",
		ApprovalMethod:   approval.ApprovalParallel,
		PassRule:         approval.PassAll,
		FieldPermissions: map[string]approval.Permission{"blob": approval.PermissionEditable},
	}
	_, err := s.db.NewInsert().Model(node).Exec(s.ctx)
	s.Require().NoError(err, "Should create node with an editable field")

	oversized := strings.Repeat("x", 70*1024) // already over the 64 KiB cap

	// seedOversizeInstance creates a running instance whose stored form data is
	// already over the cap, plus the approver's task and a pending peer so a
	// single approval leaves the PassAll node running (no outgoing edge needed).
	seedOversizeInstance := func(no, approver string) *approval.Task {
		inst := &approval.Instance{
			TenantID:      "default",
			FlowID:        s.fixture.FlowID,
			FlowVersionID: s.fixture.VersionID,
			Title:         "Oversize Instance",
			InstanceNo:    no,
			ApplicantID:   "applicant-1",
			Status:        approval.InstanceRunning,
			CurrentNodeID: &node.ID,
			FormData:      map[string]any{"blob": oversized},
		}
		_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
		s.Require().NoError(err, "Should create oversize running instance")

		task := &approval.Task{
			TenantID:   "default",
			InstanceID: inst.ID,
			NodeID:     node.ID,
			VisitID:    ensureActiveVisit(s.T(), s.ctx, s.db, "default", inst.ID, node.ID).ID,
			AssigneeID: approver,
			SortOrder:  1,
			Status:     approval.TaskPending,
		}
		_, err = s.db.NewInsert().Model(task).Exec(s.ctx)
		s.Require().NoError(err, "Should create approver task")

		peer := &approval.Task{
			TenantID:   "default",
			InstanceID: inst.ID,
			NodeID:     node.ID,
			VisitID:    ensureActiveVisit(s.T(), s.ctx, s.db, "default", inst.ID, node.ID).ID,
			AssigneeID: approver + "-peer",
			SortOrder:  2,
			Status:     approval.TaskPending,
		}
		_, err = s.db.NewInsert().Model(peer).Exec(s.ctx)
		s.Require().NoError(err, "Should create peer task")

		return task
	}

	s.Run("No-op approval on an oversize instance is allowed", func() {
		task := seedOversizeInstance("APV-OVERSIZE-NOOP-1", "approver-noop")
		_, err := s.handler.Handle(s.ctx, command.ApproveTaskCmd{
			TaskID:   task.ID,
			Operator: approval.UserInfo{ID: "approver-noop", Name: "Approver"},
			Caller:   approval.SystemCaller,
		})
		s.Require().NoError(err, "A no-op approval must not be wedged by a pre-existing oversize instance")

		var reloaded approval.Task

		reloaded.ID = task.ID
		s.Require().NoError(s.db.NewSelect().Model(&reloaded).WherePK().Scan(s.ctx), "Should reload task")
		s.Assert().Equal(approval.TaskApproved, reloaded.Status, "The no-op approval should go through")
	})

	s.Run("Drip-feed growth past the cap is still rejected", func() {
		task := seedOversizeInstance("APV-OVERSIZE-NOOP-2", "approver-grow")
		_, err := s.handler.Handle(s.ctx, command.ApproveTaskCmd{
			TaskID:   task.ID,
			Operator: approval.UserInfo{ID: "approver-grow", Name: "Approver"},
			FormData: map[string]any{"blob": strings.Repeat("y", 80*1024)},
			Caller:   approval.SystemCaller,
		})
		s.Require().ErrorIs(err, approval.ErrFormDataTooLarge, "Growing an already-oversize instance further must still be rejected")

		var reloaded approval.Task

		reloaded.ID = task.ID
		s.Require().NoError(s.db.NewSelect().Model(&reloaded).WherePK().Scan(s.ctx), "Should reload task")
		s.Assert().Equal(approval.TaskPending, reloaded.Status, "Task must stay pending after a rejected oversize growth")
	})
}

// TestApproveEnforcesRequiredPermissionField pins the required-permission
// must-fill rule: approve / handle complete the decision, so every field the
// node marks required must hold a value by then — checked against the merged
// form data so an earlier participant's value counts.
func (s *ApproveTaskTestSuite) TestApproveEnforcesRequiredPermissionField() {
	fields := []approval.FormFieldDefinition{{Key: "note", Kind: approval.FieldInput, Label: "Note"}}
	required := map[string]approval.Permission{"note": approval.PermissionRequired}

	s.Run("BlocksWhenRequiredFieldEmpty", func() {
		_, task := setupFormFieldInstance(s.T(), s.ctx, s.db, s.fixture, approval.NodeApproval, required, fields, nil, "req-empty-approver")

		_, err := s.handler.Handle(s.ctx, command.ApproveTaskCmd{
			TaskID:   task.ID,
			Operator: approval.UserInfo{ID: "req-empty-approver", Name: "Approver"},
			Opinion:  "ok",
			Caller:   approval.SystemCaller,
		})

		var re result.Error
		s.Require().ErrorAs(err, &re, "approve must be blocked while a required-permission field is empty")
		s.Assert().Equal(approval.ErrCodeFormValidationFailed, re.Code, "should be a form validation error")

		var reloaded approval.Task

		reloaded.ID = task.ID
		s.Require().NoError(s.db.NewSelect().Model(&reloaded).WherePK().Scan(s.ctx), "Should reload task")
		s.Assert().Equal(approval.TaskPending, reloaded.Status, "task must stay pending — the required check runs before any state change")
	})

	s.Run("PassesWhenRequiredFieldFilledByEarlierStep", func() {
		// An earlier participant already filled the required field; the approver
		// submits nothing, and the merged value must satisfy the requirement.
		inst, task := setupFormFieldInstance(s.T(), s.ctx, s.db, s.fixture, approval.NodeApproval, required, fields, map[string]any{"note": "filled earlier"}, "req-filled-approver")

		// A still-pending peer keeps the PassAll node running after the approval,
		// so no node-completion / edge traversal is needed on this dedicated node.
		peer := &approval.Task{
			TenantID: "default", InstanceID: inst.ID, NodeID: task.NodeID,
			VisitID:    ensureActiveVisit(s.T(), s.ctx, s.db, "default", inst.ID, task.NodeID).ID,
			AssigneeID: "req-filled-peer", SortOrder: 2, Status: approval.TaskPending,
		}
		_, err := s.db.NewInsert().Model(peer).Exec(s.ctx)
		s.Require().NoError(err, "Should create pending peer")

		_, err = s.handler.Handle(s.ctx, command.ApproveTaskCmd{
			TaskID:   task.ID,
			Operator: approval.UserInfo{ID: "req-filled-approver", Name: "Approver"},
			Opinion:  "ok",
			Caller:   approval.SystemCaller,
		})
		s.Require().NoError(err, "approve must pass when the required field was filled by an earlier step")

		var reloaded approval.Task

		reloaded.ID = task.ID
		s.Require().NoError(s.db.NewSelect().Model(&reloaded).WherePK().Scan(s.ctx), "Should reload task")
		s.Assert().Equal(approval.TaskApproved, reloaded.Status, "task should be approved")
	})

	s.Run("HandleNodeBlocksWhenRequiredFieldEmpty", func() {
		_, task := setupFormFieldInstance(s.T(), s.ctx, s.db, s.fixture, approval.NodeHandle, required, fields, nil, "req-empty-handler")

		_, err := s.handler.Handle(s.ctx, command.ApproveTaskCmd{
			TaskID:   task.ID,
			Operator: approval.UserInfo{ID: "req-empty-handler", Name: "Handler"},
			Opinion:  "ok",
			Caller:   approval.SystemCaller,
		})

		var re result.Error
		s.Require().ErrorAs(err, &re, "handle must enforce required-permission fields like approve")
		s.Assert().Equal(approval.ErrCodeFormValidationFailed, re.Code, "should be a form validation error")
	})
}

// TestApproveRejectsInvalidEditableValue proves the editable-subset value
// validation runs on the approve path: a submitted editable value that violates
// its schema rule is rejected before any state change.
func (s *ApproveTaskTestSuite) TestApproveRejectsInvalidEditableValue() {
	fields := []approval.FormFieldDefinition{
		{Key: "amount", Kind: approval.FieldNumber, Label: "Amount", Validation: &approval.ValidationRule{Min: new(10.0)}},
	}
	editable := map[string]approval.Permission{"amount": approval.PermissionEditable}
	_, task := setupFormFieldInstance(s.T(), s.ctx, s.db, s.fixture, approval.NodeApproval, editable, fields, nil, "approve-invalid-value")

	_, err := s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   task.ID,
		Operator: approval.UserInfo{ID: "approve-invalid-value", Name: "Approver"},
		Opinion:  "ok",
		FormData: map[string]any{"amount": 5},
		Caller:   approval.SystemCaller,
	})

	var re result.Error
	s.Require().ErrorAs(err, &re, "an editable value below its schema minimum must fail approve")
	s.Assert().Equal(approval.ErrCodeFormValidationFailed, re.Code, "should be a form validation error")

	var reloaded approval.Task

	reloaded.ID = task.ID
	s.Require().NoError(s.db.NewSelect().Model(&reloaded).WherePK().Scan(s.ctx), "Should reload task")
	s.Assert().Equal(approval.TaskPending, reloaded.Status, "task must stay pending after a rejected invalid value")
}

// TestApprovePermissionlessNodeDropsSubmittedFormData pins the permission-less
// skip: a node with no field permissions loads no form_fields, so submitted form
// data is neither validated nor merged — the approver's edit is silently dropped
// and the approval still succeeds.
func (s *ApproveTaskTestSuite) TestApprovePermissionlessNodeDropsSubmittedFormData() {
	// amount carries a Min:10 rule that a below-min value would fail IF the node
	// exposed it for editing — proving the data is dropped by the filter, not
	// validated, since the node grants no field permissions.
	fields := []approval.FormFieldDefinition{
		{Key: "amount", Kind: approval.FieldNumber, Label: "Amount", Validation: &approval.ValidationRule{Min: new(10.0)}},
	}
	inst, task := setupFormFieldInstance(s.T(), s.ctx, s.db, s.fixture, approval.NodeApproval, nil, fields, nil, "permless-approver")

	// A still-pending peer keeps the PassAll node running after the approval, so
	// no node completion / edge traversal is needed on this dedicated node.
	peer := &approval.Task{
		TenantID: "default", InstanceID: inst.ID, NodeID: task.NodeID,
		VisitID:    ensureActiveVisit(s.T(), s.ctx, s.db, "default", inst.ID, task.NodeID).ID,
		AssigneeID: "permless-peer", SortOrder: 2, Status: approval.TaskPending,
	}
	_, err := s.db.NewInsert().Model(peer).Exec(s.ctx)
	s.Require().NoError(err, "Should create pending peer")

	_, err = s.handler.Handle(s.ctx, command.ApproveTaskCmd{
		TaskID:   task.ID,
		Operator: approval.UserInfo{ID: "permless-approver", Name: "Approver"},
		Opinion:  "ok",
		FormData: map[string]any{"amount": 5}, // below Min:10 — dropped, so never validated
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "a permission-less node must approve with submitted data dropped, not validated")

	var reloadedTask approval.Task

	reloadedTask.ID = task.ID
	s.Require().NoError(s.db.NewSelect().Model(&reloadedTask).WherePK().Scan(s.ctx), "Should reload task")
	s.Assert().Equal(approval.TaskApproved, reloadedTask.Status, "task should be approved")

	var reloadedInst approval.Instance

	reloadedInst.ID = inst.ID
	s.Require().NoError(s.db.NewSelect().Model(&reloadedInst).WherePK().Scan(s.ctx), "Should reload instance")
	s.Assert().NotContains(reloadedInst.FormData, "amount", "submitted data on a permission-less node must be dropped, not merged")
}
