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
		return &CreateFlowTestSuite{
			ctx: env.Ctx,
			db:  env.DB,
		}
	})
}

// CreateFlowTestSuite tests the CreateFlowHandler.
type CreateFlowTestSuite struct {
	suite.Suite

	ctx        context.Context
	db         orm.DB
	handler    *command.CreateFlowHandler
	categoryID string
}

func (s *CreateFlowTestSuite) SetupSuite() {
	category := &approval.FlowCategory{
		TenantID: "default",
		Code:     "test",
		Name:     "Test Category",
	}
	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "Should insert test category")

	s.categoryID = category.ID
	s.handler = command.NewCreateFlowHandler(s.db)
}

func (s *CreateFlowTestSuite) TearDownSuite() {
	_, err := s.db.NewDelete().
		Model((*approval.FlowCategory)(nil)).
		Where(func(cb orm.ConditionBuilder) {
			cb.IsNotNull("id")
		}).
		Exec(s.ctx)
	s.Require().NoError(err, "Should clean flow categories")
}

func (s *CreateFlowTestSuite) TearDownTest() {
	deleteAll(s.ctx, s.db, (*approval.FlowInitiator)(nil), (*approval.Flow)(nil))
}

func (s *CreateFlowTestSuite) TestCreateFlowSuccess() {
	cmd := command.CreateFlowCmd{
		TenantID:               "tenant-1",
		Code:                   "leave",
		Name:                   "Leave Approval",
		CategoryID:             s.categoryID,
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "{{.applicantName}}'s leave request",
	}

	result, err := s.handler.Handle(s.ctx, cmd)
	s.Require().NoError(err, "Should create flow without error")
	s.Require().NotNil(result, "Should return created flow")

	s.Assert().NotEmpty(result.ID, "Should generate flow ID")
	s.Assert().Equal("tenant-1", result.TenantID, "Should set TenantID")
	s.Assert().Equal("leave", result.Code, "Should set Code")
	s.Assert().Equal("Leave Approval", result.Name, "Should set Name")
	s.Assert().Equal(s.categoryID, result.CategoryID, "Should set CategoryID")
	s.Assert().Equal(approval.BindingStandalone, result.BindingMode, "Should set BindingMode")
	s.Assert().True(result.IsAllInitiationAllowed, "Should set IsAllInitiationAllowed")
	s.Assert().True(result.IsActive, "Should default IsActive to true")
	s.Assert().Equal(0, result.CurrentVersion, "Should default CurrentVersion to 0")

	// Verify DB record
	var flow approval.Flow

	flow.ID = result.ID

	err = s.db.NewSelect().
		Model(&flow).
		WherePK().
		Scan(s.ctx)
	s.Require().NoError(err, "Should find flow in DB")
	s.Assert().Equal("leave", flow.Code, "DB record should have correct Code")
}

func (s *CreateFlowTestSuite) TestCreateFlowDefaultTenant() {
	cmd := command.CreateFlowCmd{
		TenantID:              "", // empty → defaults to "default"
		Code:                  "reimbursement",
		Name:                  "Reimbursement Approval",
		CategoryID:            s.categoryID,
		BindingMode:           approval.BindingStandalone,
		InstanceTitleTemplate: "Reimbursement request",
	}

	result, err := s.handler.Handle(s.ctx, cmd)
	s.Require().NoError(err, "Should create flow without error")
	s.Assert().Equal("default", result.TenantID, "Should default TenantID to 'default'")
}

func (s *CreateFlowTestSuite) TestCreateFlowDuplicateCode() {
	cmd := command.CreateFlowCmd{
		TenantID:              "tenant-dup",
		Code:                  "unique-code",
		Name:                  "First Flow",
		CategoryID:            s.categoryID,
		BindingMode:           approval.BindingStandalone,
		InstanceTitleTemplate: "Title Template",
	}

	_, err := s.handler.Handle(s.ctx, cmd)
	s.Require().NoError(err, "Should create first flow")

	// Same tenant + code → should fail
	cmd.Name = "Second Flow"
	_, err = s.handler.Handle(s.ctx, cmd)
	s.Require().Error(err, "Should reject duplicate code")
	s.Assert().ErrorIs(err, shared.ErrFlowCodeExists, "Should return ErrFlowCodeExists")
}

func (s *CreateFlowTestSuite) TestCreateFlowWithInitiators() {
	cmd := command.CreateFlowCmd{
		TenantID:              "tenant-init",
		Code:                  "with-initiators",
		Name:                  "Flow With Initiators",
		CategoryID:            s.categoryID,
		BindingMode:           approval.BindingStandalone,
		InstanceTitleTemplate: "Template",
		Initiators: []shared.CreateFlowInitiatorCmd{
			{Kind: approval.InitiatorUser, IDs: []string{"user-1", "user-2"}},
			{Kind: approval.InitiatorRole, IDs: []string{"role-admin"}},
		},
	}

	result, err := s.handler.Handle(s.ctx, cmd)
	s.Require().NoError(err, "Should create flow with initiators")

	// Verify initiators in DB
	var initiators []approval.FlowInitiator

	err = s.db.NewSelect().
		Model(&initiators).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("flow_id", result.ID)
		}).
		OrderBy("kind").
		Scan(s.ctx)
	s.Require().NoError(err, "Should query initiators")
	s.Require().Len(initiators, 2, "Should insert two initiators")

	s.Assert().Equal(approval.InitiatorRole, initiators[0].Kind, "Should set first initiator kind")
	s.Assert().Equal([]string{"role-admin"}, initiators[0].IDs, "Should set first initiator IDs")
	s.Assert().Equal(approval.InitiatorUser, initiators[1].Kind, "Should set second initiator kind")
	s.Assert().Equal([]string{"user-1", "user-2"}, initiators[1].IDs, "Should set second initiator IDs")
}
