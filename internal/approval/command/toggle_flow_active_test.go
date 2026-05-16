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
		return &ToggleFlowActiveTestSuite{
			ctx: env.Ctx,
			db:  env.DB,
		}
	})
}

// ToggleFlowActiveTestSuite tests the ToggleFlowActiveHandler.
type ToggleFlowActiveTestSuite struct {
	suite.Suite

	ctx        context.Context
	db         orm.DB
	handler    *command.ToggleFlowActiveHandler
	categoryID string
	flowID     string
}

func (s *ToggleFlowActiveTestSuite) SetupSuite() {
	category := &approval.FlowCategory{
		TenantID: "default",
		Code:     "test-toggle",
		Name:     "Test Toggle Category",
	}
	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test category")
	s.categoryID = category.ID

	flow := &approval.Flow{
		TenantID:               "tenant-1",
		CategoryID:             s.categoryID,
		Code:                   "toggle-flow",
		Name:                   "Toggle Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Template",
		IsActive:               true,
	}
	_, err = s.db.NewInsert().Model(flow).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test flow")
	s.flowID = flow.ID

	s.handler = command.NewToggleFlowActiveHandler(s.db)
}

func (s *ToggleFlowActiveTestSuite) TearDownTest() {
	// Reset to initial state (is_active = true).
	_, _ = s.db.NewUpdate().
		Model((*approval.Flow)(nil)).
		Set("is_active", true).
		Where(func(cb orm.ConditionBuilder) { cb.PKEquals(s.flowID) }).
		Exec(s.ctx)
}

func (s *ToggleFlowActiveTestSuite) TearDownSuite() {
	deleteAll(s.ctx, s.db, (*approval.Flow)(nil), (*approval.FlowCategory)(nil))
}

func (s *ToggleFlowActiveTestSuite) TestActivate() {
	_, err := s.db.NewUpdate().
		Model((*approval.Flow)(nil)).
		Set("is_active", false).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("id", s.flowID)
		}).
		Exec(s.ctx)
	s.Require().NoError(err, "Should deactivate flow for test")

	cmd := command.ToggleFlowActiveCmd{
		FlowID:   s.flowID,
		IsActive: true,
	}

	_, err = s.handler.Handle(s.ctx, cmd)
	s.Require().NoError(err, "Should activate flow without error")

	var flow approval.Flow

	flow.ID = s.flowID
	err = s.db.NewSelect().
		Model(&flow).
		WherePK().
		Scan(s.ctx)
	s.Require().NoError(err, "Should query flow")
	s.Assert().True(flow.IsActive, "Flow should be active")
}

func (s *ToggleFlowActiveTestSuite) TestDeactivate() {
	cmd := command.ToggleFlowActiveCmd{
		FlowID:   s.flowID,
		IsActive: false,
	}

	_, err := s.handler.Handle(s.ctx, cmd)
	s.Require().NoError(err, "Should deactivate flow without error")

	var flow approval.Flow

	flow.ID = s.flowID
	err = s.db.NewSelect().
		Model(&flow).
		WherePK().
		Scan(s.ctx)
	s.Require().NoError(err, "Should query flow")
	s.Assert().False(flow.IsActive, "Flow should be inactive")
}

func (s *ToggleFlowActiveTestSuite) TestNotFound() {
	cmd := command.ToggleFlowActiveCmd{
		FlowID:   "non-existent-flow-id",
		IsActive: true,
	}

	_, err := s.handler.Handle(s.ctx, cmd)
	s.Require().Error(err, "Should return error for non-existent flow")
	s.Assert().ErrorIs(err, shared.ErrFlowNotFound, "Should return ErrFlowNotFound")
}
