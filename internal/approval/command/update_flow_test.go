package command_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/command"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &UpdateFlowTestSuite{
			ctx: env.Ctx,
			db:  env.DB,
		}
	})
}

// UpdateFlowTestSuite tests the UpdateFlowHandler.
type UpdateFlowTestSuite struct {
	suite.Suite

	ctx        context.Context
	db         orm.DB
	handler    *command.UpdateFlowHandler
	categoryID string
	flowID     string
}

func (s *UpdateFlowTestSuite) SetupSuite() {
	category := &approval.FlowCategory{
		TenantID: "default",
		Code:     "test-update",
		Name:     "Test Update Category",
	}
	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test category")
	s.categoryID = category.ID

	flow := &approval.Flow{
		TenantID:               "tenant-1",
		CategoryID:             s.categoryID,
		Code:                   "original-flow",
		Name:                   "Original Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: false,
		InstanceTitleTemplate:  "Original Template",
		IsActive:               true,
	}
	_, err = s.db.NewInsert().Model(flow).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test flow")
	s.flowID = flow.ID

	initiator := &approval.FlowInitiator{
		FlowID: s.flowID,
		Kind:   approval.InitiatorUser,
		IDs:    []string{"user-old"},
	}
	_, err = s.db.NewInsert().Model(initiator).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test initiator")

	s.handler = command.NewUpdateFlowHandler(s.db)
}

func (s *UpdateFlowTestSuite) TearDownTest() {
	// Reset flow to original state and remove test initiators.
	_, _ = s.db.NewUpdate().
		Model((*approval.Flow)(nil)).
		Set("name", "Original Flow").
		Set("icon", nil).
		Set("description", nil).
		Set("admin_user_ids", nil).
		Set("is_all_initiation_allowed", false).
		Set("instance_title_template", "Original Template").
		Set("binding_mode", approval.BindingStandalone).
		Set("business_table", nil).
		Set("business_pk_field", nil).
		Set("business_status_field", nil).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(s.flowID) }).
		Exec(s.ctx)
	_, _ = s.db.NewDelete().
		Model((*approval.FlowInitiator)(nil)).
		Where(func(cb orm.ConditionBuilder) { cb.Equals("flow_id", s.flowID) }).
		Exec(s.ctx)
	// Re-insert original initiator.
	_, _ = s.db.NewInsert().Model(&approval.FlowInitiator{
		FlowID: s.flowID,
		Kind:   approval.InitiatorUser,
		IDs:    []string{"user-old"},
	}).Exec(s.ctx)
}

func (s *UpdateFlowTestSuite) TearDownSuite() {
	deleteAll(s.ctx, s.db, (*approval.FlowInitiator)(nil), (*approval.Flow)(nil), (*approval.FlowCategory)(nil))
}

func (s *UpdateFlowTestSuite) TestUpdateFlowSuccess() {
	icon := "new-icon"
	desc := "Updated description"

	cmd := command.UpdateFlowCmd{
		BindingMode:            approval.BindingStandalone,
		FlowID:                 s.flowID,
		Name:                   "Updated Flow",
		Icon:                   &icon,
		Description:            &desc,
		AdminUserIDs:           []string{"admin-1", "admin-2"},
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Updated Template",
		Initiators: []shared.CreateFlowInitiatorCmd{
			{Kind: approval.InitiatorRole, IDs: []string{"role-new"}},
		},
		Caller: approval.SystemCaller,
	}

	result, err := s.handler.Handle(s.ctx, cmd)
	s.Require().NoError(err, "Should update flow without error")
	s.Require().NotNil(result, "Should return updated flow")

	s.Assert().Equal("Updated Flow", result.Name, "Should update Name")
	s.Assert().Equal(&icon, result.Icon, "Should update Icon")
	s.Assert().Equal(&desc, result.Description, "Should update Description")
	s.Assert().Equal([]string{"admin-1", "admin-2"}, result.AdminUserIDs, "Should update AdminUserIDs")
	s.Assert().True(result.IsAllInitiationAllowed, "Should update IsAllInitiationAllowed")
	s.Assert().Equal("Updated Template", result.InstanceTitleTemplate, "Should update InstanceTitleTemplate")

	var initiators []approval.FlowInitiator

	err = s.db.NewSelect().
		Model(&initiators).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_id", s.flowID)
		}).
		Scan(s.ctx)
	s.Require().NoError(err, "Should query initiators")
	s.Require().Len(initiators, 1, "Should have one initiator after update")
	s.Assert().Equal(approval.InitiatorRole, initiators[0].Kind, "Should update initiator kind")
	s.Assert().Equal([]string{"role-new"}, initiators[0].IDs, "Should update initiator IDs")
}

func (s *UpdateFlowTestSuite) TestUpdateFlowNotFound() {
	cmd := command.UpdateFlowCmd{
		BindingMode:            approval.BindingStandalone,
		FlowID:                 "non-existent-flow-id",
		Name:                   "Updated Flow",
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Template",
		Caller:                 approval.SystemCaller,
	}

	_, err := s.handler.Handle(s.ctx, cmd)
	s.Require().Error(err, "Should return error for non-existent flow")
	s.Assert().ErrorIs(err, shared.ErrFlowNotFound, "Should return ErrFlowNotFound")
}

// TestUpdateFlowBindingGuardWhileRunning verifies that the business-binding
// configuration is frozen while an instance of the flow is still running — a
// binding change is rejected, but a non-binding edit (name only) is still
// allowed.
func (s *UpdateFlowTestSuite) TestUpdateFlowBindingGuardWhileRunning() {
	version := &approval.FlowVersion{FlowID: s.flowID, Version: 99, Status: approval.VersionDraft}
	_, err := s.db.NewInsert().Model(version).Exec(s.ctx)
	s.Require().NoError(err, "Should insert flow version for the running instance")

	inst := &approval.Instance{
		TenantID:      "tenant-1",
		FlowID:        s.flowID,
		FlowVersionID: version.ID,
		Title:         "Running",
		InstanceNo:    "BIND-GUARD-001",
		ApplicantID:   "applicant-1",
		Status:        approval.InstanceRunning,
	}
	_, err = s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert running instance")

	// Clean up in FK-RESTRICT order (instance → version); flow/category are
	// removed by TearDownSuite. Without this, deleting the flow later would fail.
	defer func() {
		_, _ = s.db.NewDelete().Model((*approval.Instance)(nil)).
			Where(func(cb orm.ConditionBuilder) { cb.PKEquals(inst.ID) }).Exec(s.ctx)
		_, _ = s.db.NewDelete().Model((*approval.FlowVersion)(nil)).
			Where(func(cb orm.ConditionBuilder) { cb.PKEquals(version.ID) }).Exec(s.ctx)
	}()

	// Reset to a deterministic standalone baseline: sibling tests in this suite may
	// leave the flow in another binding state (the command layer does not enforce
	// the resource layer's "bindingMode required"), and this test asserts on the
	// binding-change transition specifically.
	_, err = s.db.NewUpdate().
		Model((*approval.Flow)(nil)).
		Set("binding_mode", approval.BindingStandalone).
		Set("business_table", nil).
		Set("business_pk_field", nil).
		Set("business_status_field", nil).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(s.flowID) }).
		Exec(s.ctx)
	s.Require().NoError(err, "Should reset flow to a standalone baseline")

	table, pk, status := "biz_orders", "id", "approval_status"

	// Switching the binding mode (standalone → business) while the instance runs
	// is rejected.
	_, err = s.handler.Handle(s.ctx, command.UpdateFlowCmd{
		FlowID:                s.flowID,
		Name:                  "Original Flow",
		BindingMode:           approval.BindingBusiness,
		BusinessTable:         &table,
		BusinessPkField:       &pk,
		BusinessStatusField:   &status,
		InstanceTitleTemplate: "Original Template",
		Caller:                approval.SystemCaller,
	})
	s.Require().Error(err, "Binding change must be blocked while an instance runs")
	s.Assert().ErrorIs(err, shared.ErrFlowBindingLocked, "Should return ErrFlowBindingLocked")

	// A non-binding edit (name only, binding unchanged from the baseline) is still
	// allowed even though an instance is running.
	updated, err := s.handler.Handle(s.ctx, command.UpdateFlowCmd{
		FlowID:                s.flowID,
		Name:                  "Renamed While Running",
		BindingMode:           approval.BindingStandalone,
		InstanceTitleTemplate: "Original Template",
		Caller:                approval.SystemCaller,
	})
	s.Require().NoError(err, "Non-binding edit must be allowed while an instance runs")
	s.Assert().Equal("Renamed While Running", updated.Name, "Should apply the non-binding edit")
}

func (s *UpdateFlowTestSuite) TestUpdateAllFields() {
	icon := "all-fields-icon"
	desc := "All fields description"

	cmd := command.UpdateFlowCmd{
		BindingMode:            approval.BindingStandalone,
		FlowID:                 s.flowID,
		Name:                   "All Fields Updated",
		Icon:                   &icon,
		Description:            &desc,
		AdminUserIDs:           []string{"admin-all"},
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "All Fields Template",
		Initiators: []shared.CreateFlowInitiatorCmd{
			{Kind: approval.InitiatorUser, IDs: []string{"user-all-1"}},
			{Kind: approval.InitiatorRole, IDs: []string{"role-all-1"}},
		},
		Caller: approval.SystemCaller,
	}

	result, err := s.handler.Handle(s.ctx, cmd)
	s.Require().NoError(err, "Should update all fields without error")

	s.Assert().Equal("All Fields Updated", result.Name, "Name should be updated")
	s.Assert().Equal(&icon, result.Icon, "Icon should be updated")
	s.Assert().Equal(&desc, result.Description, "Description should be updated")
	s.Assert().Equal([]string{"admin-all"}, result.AdminUserIDs, "AdminUserIDs should be updated")
	s.Assert().True(result.IsAllInitiationAllowed, "IsAllInitiationAllowed should be true")
	s.Assert().Equal("All Fields Template", result.InstanceTitleTemplate, "InstanceTitleTemplate should be updated")

	var initiators []approval.FlowInitiator

	err = s.db.NewSelect().
		Model(&initiators).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_id", s.flowID)
		}).
		OrderBy("kind").
		Scan(s.ctx)
	s.Require().NoError(err, "Should query initiators")
	s.Require().Len(initiators, 2, "Should have two initiators")
}

func (s *UpdateFlowTestSuite) TestUpdateFlowToBusinessBinding() {
	table := "t_orders"
	pk := "id"
	status := "approval_status"

	result, err := s.handler.Handle(s.ctx, command.UpdateFlowCmd{
		FlowID:                 s.flowID,
		Name:                   "Now Business Bound",
		BindingMode:            approval.BindingBusiness,
		BusinessTable:          &table,
		BusinessPkField:        &pk,
		BusinessStatusField:    &status,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Template",
		Caller:                 approval.SystemCaller,
	})
	s.Require().NoError(err, "Should switch to a complete business binding")
	s.Assert().Equal(approval.BindingBusiness, result.BindingMode)

	var flow approval.Flow

	flow.ID = s.flowID
	s.Require().NoError(s.db.NewSelect().Model(&flow).WherePK().Scan(s.ctx))
	s.Assert().Equal(approval.BindingBusiness, flow.BindingMode, "binding_mode should persist")
	s.Require().NotNil(flow.BusinessStatusField)
	s.Assert().Equal("approval_status", *flow.BusinessStatusField, "business_status_field should persist")
}

func (s *UpdateFlowTestSuite) TestUpdateFlowBusinessBindingIncomplete() {
	status := "approval_status"

	_, err := s.handler.Handle(s.ctx, command.UpdateFlowCmd{
		FlowID:                 s.flowID,
		Name:                   "Half Bound",
		BindingMode:            approval.BindingBusiness,
		BusinessStatusField:    &status,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Template",
		Caller:                 approval.SystemCaller,
	})
	s.Require().Error(err, "An incomplete business binding on update should be rejected")
	s.Assert().ErrorIs(err, shared.ErrBindingIncomplete)
}

func (s *UpdateFlowTestSuite) TestUpdateFlowBusinessToStandaloneClearsFields() {
	table := "t_orders"
	pk := "id"
	status := "approval_status"

	_, err := s.handler.Handle(s.ctx, command.UpdateFlowCmd{
		FlowID:                 s.flowID,
		Name:                   "Business",
		BindingMode:            approval.BindingBusiness,
		BusinessTable:          &table,
		BusinessPkField:        &pk,
		BusinessStatusField:    &status,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Template",
		Caller:                 approval.SystemCaller,
	})
	s.Require().NoError(err, "Should set the business binding first")

	// Switch back to standalone with the business fields omitted: the nullzero
	// columns plus the Select list must clear them to NULL, not leave them set.
	_, err = s.handler.Handle(s.ctx, command.UpdateFlowCmd{
		FlowID:                 s.flowID,
		Name:                   "Back To Standalone",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Template",
		Caller:                 approval.SystemCaller,
	})
	s.Require().NoError(err, "Should switch back to standalone")

	var flow approval.Flow

	flow.ID = s.flowID
	s.Require().NoError(s.db.NewSelect().Model(&flow).WherePK().Scan(s.ctx))
	s.Assert().Equal(approval.BindingStandalone, flow.BindingMode)
	s.Assert().Nil(flow.BusinessTable, "business_table should clear to NULL")
	s.Assert().Nil(flow.BusinessPkField, "business_pk_field should clear to NULL")
	s.Assert().Nil(flow.BusinessStatusField, "business_status_field should clear to NULL")
}

func (s *UpdateFlowTestSuite) TestUpdateFlowRejectsInvalidEnums() {
	s.Run("BindingMode", func() {
		_, err := s.handler.Handle(s.ctx, command.UpdateFlowCmd{
			FlowID:                 s.flowID,
			BindingMode:            "bogus",
			Name:                   "Enum Guard Flow",
			IsAllInitiationAllowed: true,
			InstanceTitleTemplate:  "t",
			Caller:                 approval.SystemCaller,
		})
		s.Require().ErrorIs(err, shared.ErrInvalidBindingMode,
			"An out-of-enum binding mode would silently disable the business write-back and must be rejected")
	})

	s.Run("InitiatorKind", func() {
		_, err := s.handler.Handle(s.ctx, command.UpdateFlowCmd{
			FlowID:                 s.flowID,
			BindingMode:            approval.BindingStandalone,
			Name:                   "Enum Guard Flow",
			IsAllInitiationAllowed: true,
			InstanceTitleTemplate:  "t",
			Initiators: []shared.CreateFlowInitiatorCmd{
				{Kind: "sideways", IDs: []string{"u1"}},
			},
			Caller: approval.SystemCaller,
		})
		s.Require().ErrorIs(err, shared.ErrInvalidInitiatorKind,
			"An out-of-enum initiator kind would silently never match and must be rejected")
	})
}
