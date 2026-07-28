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

	s.handler = command.NewUpdateFlowHandler(s.db, newTestBindingValidator())
}

func (s *UpdateFlowTestSuite) TearDownTest() {
	// Reset flow to original state and remove test initiators.
	_, _ = s.db.NewUpdate().
		Model((*approval.Flow)(nil)).
		Set("name", "Original Flow").
		Set("icon", nil).
		Set("description", nil).
		Set("labels", nil).
		Set("admin_user_ids", nil).
		Set("is_all_initiation_allowed", false).
		Set("instance_title_template", "Original Template").
		Set("binding_mode", approval.BindingStandalone).
		Set("business_binding", nil).
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
		IsAllInitiationAllowed: false,
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
	s.Assert().False(result.IsAllInitiationAllowed,
		"A flow carrying initiator rules must stay restricted — the two are mutually exclusive")
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

func (s *UpdateFlowTestSuite) TestUpdateFlowLabels() {
	reload := func() approval.Flow {
		var stored approval.Flow

		stored.ID = s.flowID
		err := s.db.NewSelect().Model(&stored).WherePK().Scan(s.ctx)
		s.Require().NoError(err, "Should reload flow")

		return stored
	}

	baseCmd := func() command.UpdateFlowCmd {
		return command.UpdateFlowCmd{
			IsAllInitiationAllowed: true,
			FlowID:                 s.flowID,
			Name:                   "Original Flow",
			BindingMode:            approval.BindingStandalone,
			InstanceTitleTemplate:  "Original Template",
			Caller:                 approval.SystemCaller,
		}
	}

	s.Run("ReplacesLabels", func() {
		cmd := baseCmd()
		cmd.Labels = map[string]string{"app": "crm"}
		_, err := s.handler.Handle(s.ctx, cmd)
		s.Require().NoError(err, "Should update flow with labels")
		s.Assert().Equal(map[string]string{"app": "crm"}, reload().Labels, "Labels should be persisted")

		cmd = baseCmd()
		cmd.Labels = map[string]string{"app": "erp", "mobile": "true"}
		_, err = s.handler.Handle(s.ctx, cmd)
		s.Require().NoError(err, "Should update flow again")
		s.Assert().Equal(map[string]string{"app": "erp", "mobile": "true"}, reload().Labels,
			"Update should fully replace the previous label set, not merge into it")
	})

	s.Run("OmittedLabelsClear", func() {
		_, err := s.handler.Handle(s.ctx, baseCmd())
		s.Require().NoError(err, "Should update flow without labels")
		s.Assert().Empty(reload().Labels, "Update is full-replace: omitting labels clears the stored set")
	})

	s.Run("RejectsInvalidLabelKey", func() {
		cmd := baseCmd()
		cmd.Labels = map[string]string{"app.id": "crm"}
		_, err := s.handler.Handle(s.ctx, cmd)
		s.Require().ErrorIs(err, shared.ErrInvalidFlowLabel,
			"A dotted label key must be rejected at save time — it would silently escape the label filter")
	})
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

func (s *UpdateFlowTestSuite) TestUpdateFlowBindingDoesNotMutateRunningVersion() {
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

	table, pk, status := "biz_orders", "id", "approval_status"
	instanceCol := "apv_instance_id"
	updated, err := s.handler.Handle(s.ctx, command.UpdateFlowCmd{
		IsAllInitiationAllowed: true,
		FlowID:                 s.flowID,
		Name:                   "Renamed While Running",
		BindingMode:            approval.BindingBusiness,
		BusinessBinding: &approval.BusinessBindingConfig{
			TableName:        table,
			KeyColumns:       []string{pk},
			StatusColumn:     status,
			InstanceIDColumn: &instanceCol,
		},
		InstanceTitleTemplate: "Original Template",
		Caller:                approval.SystemCaller,
	})
	s.Require().NoError(err, "Flow binding edits should be allowed while old versions are in use")
	s.Assert().Equal("Renamed While Running", updated.Name, "Should apply the non-binding edit")

	var reloadedVersion approval.FlowVersion

	reloadedVersion.ID = version.ID
	s.Require().NoError(s.db.NewSelect().Model(&reloadedVersion).WherePK().Scan(s.ctx), "Should reload old flow version")
	s.Assert().Nil(reloadedVersion.BusinessBinding, "Updating Flow must not rewrite an existing version snapshot")
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
		IsAllInitiationAllowed: false,
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
	s.Assert().False(result.IsAllInitiationAllowed,
		"A flow carrying initiator rules must stay restricted — the two are mutually exclusive")
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
	instanceCol := "apv_instance_id"

	result, err := s.handler.Handle(s.ctx, command.UpdateFlowCmd{
		FlowID:      s.flowID,
		Name:        "Now Business Bound",
		BindingMode: approval.BindingBusiness,
		BusinessBinding: &approval.BusinessBindingConfig{
			TableName:        table,
			KeyColumns:       []string{pk},
			StatusColumn:     status,
			InstanceIDColumn: &instanceCol,
		},
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
	s.Require().NotNil(flow.BusinessBinding)
	s.Assert().Equal("approval_status", flow.BusinessBinding.StatusColumn, "statusColumn should persist")
}

func (s *UpdateFlowTestSuite) TestUpdateFlowBusinessBindingIncomplete() {
	status := "approval_status"

	_, err := s.handler.Handle(s.ctx, command.UpdateFlowCmd{
		FlowID:      s.flowID,
		Name:        "Half Bound",
		BindingMode: approval.BindingBusiness,
		BusinessBinding: &approval.BusinessBindingConfig{
			StatusColumn: status,
		},
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
	instanceCol := "apv_instance_id"
	startedCol := "apv_started_at"
	finishedCol := "apv_finished_at"

	_, err := s.handler.Handle(s.ctx, command.UpdateFlowCmd{
		FlowID:      s.flowID,
		Name:        "Business",
		BindingMode: approval.BindingBusiness,
		BusinessBinding: &approval.BusinessBindingConfig{
			TableName:        table,
			KeyColumns:       []string{pk},
			StatusColumn:     status,
			InstanceIDColumn: &instanceCol,
			StartedAtColumn:  &startedCol,
			FinishedAtColumn: &finishedCol,
		},
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
	s.Assert().Nil(flow.BusinessBinding, "business_binding should clear to NULL")
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
