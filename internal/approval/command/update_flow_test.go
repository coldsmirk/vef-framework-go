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
		FlowID:                 "non-existent-flow-id",
		Name:                   "Updated Flow",
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Template",
	}

	_, err := s.handler.Handle(s.ctx, cmd)
	s.Require().Error(err, "Should return error for non-existent flow")
	s.Assert().ErrorIs(err, shared.ErrFlowNotFound, "Should return ErrFlowNotFound")
}

func (s *UpdateFlowTestSuite) TestUpdateAllFields() {
	icon := "all-fields-icon"
	desc := "All fields description"

	cmd := command.UpdateFlowCmd{
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
