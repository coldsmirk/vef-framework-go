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
	s.handler = command.NewCreateFlowHandler(s.db, newTestBindingValidator())
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
		Caller:                 approval.SystemCaller,
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

func (s *CreateFlowTestSuite) TestCreateFlowLabels() {
	s.Run("PersistsLabels", func() {
		cmd := command.CreateFlowCmd{
			IsAllInitiationAllowed: true,
			TenantID:               "tenant-1",
			Code:                   "labeled",
			Name:                   "Labeled Flow",
			CategoryID:             s.categoryID,
			BindingMode:            approval.BindingStandalone,
			InstanceTitleTemplate:  "Test",
			Labels:                 map[string]string{"app": "crm", "mobile": "true"},
			Caller:                 approval.SystemCaller,
		}

		created, err := s.handler.Handle(s.ctx, cmd)
		s.Require().NoError(err, "Should create labeled flow without error")

		// Reload to prove the labels survive the jsonb round trip, not just
		// the in-memory echo of the command.
		var stored approval.Flow

		stored.ID = created.ID
		err = s.db.NewSelect().Model(&stored).WherePK().Scan(s.ctx)
		s.Require().NoError(err, "Should reload created flow")
		s.Assert().Equal(map[string]string{"app": "crm", "mobile": "true"}, stored.Labels,
			"Labels should round-trip through storage")
	})

	s.Run("RejectsInvalidLabelKey", func() {
		cmd := command.CreateFlowCmd{
			IsAllInitiationAllowed: true,
			TenantID:               "tenant-1",
			Code:                   "bad-label",
			Name:                   "Bad Label Flow",
			CategoryID:             s.categoryID,
			BindingMode:            approval.BindingStandalone,
			InstanceTitleTemplate:  "Test",
			Labels:                 map[string]string{"app.id": "crm"},
			Caller:                 approval.SystemCaller,
		}

		_, err := s.handler.Handle(s.ctx, cmd)
		s.Require().ErrorIs(err, shared.ErrInvalidFlowLabel,
			"A dotted label key must be rejected at save time — it would silently escape the label filter")
	})
}

func (s *CreateFlowTestSuite) TestCreateFlowDefaultTenant() {
	cmd := command.CreateFlowCmd{
		IsAllInitiationAllowed: true,
		TenantID:               "", // empty → defaults to "default"
		Code:                   "reimbursement",
		Name:                   "Reimbursement Approval",
		CategoryID:             s.categoryID,
		BindingMode:            approval.BindingStandalone,
		InstanceTitleTemplate:  "Reimbursement request",
		Caller:                 approval.SystemCaller,
	}

	result, err := s.handler.Handle(s.ctx, cmd)
	s.Require().NoError(err, "Should create flow without error")
	s.Assert().Equal("default", result.TenantID, "Should default TenantID to 'default'")
}

func (s *CreateFlowTestSuite) TestCreateFlowDuplicateCode() {
	cmd := command.CreateFlowCmd{
		IsAllInitiationAllowed: true,
		TenantID:               "tenant-dup",
		Code:                   "unique-code",
		Name:                   "First Flow",
		CategoryID:             s.categoryID,
		BindingMode:            approval.BindingStandalone,
		InstanceTitleTemplate:  "Title Template",
		Caller:                 approval.SystemCaller,
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
		Caller: approval.SystemCaller,
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

func (s *CreateFlowTestSuite) TestCreateFlowBusinessBindingComplete() {
	table := "t_leave"
	pk := "id"
	status := "approval_status"
	instanceCol := "apv_instance_id"

	result, err := s.handler.Handle(s.ctx, command.CreateFlowCmd{
		IsAllInitiationAllowed: true,
		TenantID:               "tenant-binding",
		Code:                   "business-complete",
		Name:                   "Business Bound",
		CategoryID:             s.categoryID,
		BindingMode:            approval.BindingBusiness,
		BusinessBinding: &approval.BusinessBindingConfig{
			TableName:        table,
			KeyColumns:       []string{pk},
			StatusColumn:     status,
			InstanceIDColumn: &instanceCol,
		},
		InstanceTitleTemplate: "Title",
		Caller:                approval.SystemCaller,
	})
	s.Require().NoError(err, "A complete business binding should be accepted")
	s.Assert().Equal(approval.BindingBusiness, result.BindingMode)
	s.Require().NotNil(result.BusinessBinding)
	s.Assert().Equal("t_leave", result.BusinessBinding.TableName)
}

func (s *CreateFlowTestSuite) TestCreateFlowBusinessBindingIncomplete() {
	pk := "id"
	status := "approval_status"

	// Business mode with a missing table must be rejected, not silently saved
	// (it would no-op the status write-back on the first completed instance).
	_, err := s.handler.Handle(s.ctx, command.CreateFlowCmd{
		IsAllInitiationAllowed: true,
		TenantID:               "tenant-binding-bad",
		Code:                   "business-incomplete",
		Name:                   "Half Bound",
		CategoryID:             s.categoryID,
		BindingMode:            approval.BindingBusiness,
		BusinessBinding: &approval.BusinessBindingConfig{
			KeyColumns:   []string{pk},
			StatusColumn: status,
		},
		InstanceTitleTemplate: "Title",
		Caller:                approval.SystemCaller,
	})
	s.Require().Error(err, "An incomplete business binding should be rejected")
	s.Assert().ErrorIs(err, shared.ErrBindingIncomplete)
}

func (s *CreateFlowTestSuite) TestCreateFlowLinkageColumns() {
	table := "t_leave"
	pk := "id"
	status := "approval_status"
	instanceCol := "apv_instance_id"

	linkageCmd := func(code string) command.CreateFlowCmd {
		return command.CreateFlowCmd{
			IsAllInitiationAllowed: true,
			TenantID:               "tenant-linkage",
			Code:                   code,
			Name:                   "Linkage Bound",
			CategoryID:             s.categoryID,
			BindingMode:            approval.BindingBusiness,
			BusinessBinding: &approval.BusinessBindingConfig{
				TableName:        table,
				KeyColumns:       []string{pk},
				StatusColumn:     status,
				InstanceIDColumn: &instanceCol,
			},
			InstanceTitleTemplate: "Title",
			Caller:                approval.SystemCaller,
		}
	}

	s.Run("PersistsOptionalColumns", func() {
		instanceCol, startedCol, finishedCol := "apv_instance_id", "apv_started_at", "apv_finished_at"

		cmd := linkageCmd("linkage-full")
		cmd.BusinessBinding.InstanceIDColumn = &instanceCol
		cmd.BusinessBinding.StartedAtColumn = &startedCol
		cmd.BusinessBinding.FinishedAtColumn = &finishedCol

		result, err := s.handler.Handle(s.ctx, cmd)
		s.Require().NoError(err, "Optional linkage columns should be accepted")

		var flow approval.Flow

		flow.ID = result.ID
		s.Require().NoError(s.db.NewSelect().Model(&flow).WherePK().Scan(s.ctx), "Should reload the created flow")
		s.Require().NotNil(flow.BusinessBinding, "business_binding should persist")
		s.Require().NotNil(flow.BusinessBinding.InstanceIDColumn, "instanceIdColumn should persist")
		s.Assert().Equal("apv_instance_id", *flow.BusinessBinding.InstanceIDColumn, "Instance-id column should round-trip")
		s.Require().NotNil(flow.BusinessBinding.StartedAtColumn, "startedAtColumn should persist")
		s.Assert().Equal("apv_started_at", *flow.BusinessBinding.StartedAtColumn, "Started-at column should round-trip")
		s.Require().NotNil(flow.BusinessBinding.FinishedAtColumn, "finishedAtColumn should persist")
		s.Assert().Equal("apv_finished_at", *flow.BusinessBinding.FinishedAtColumn, "Finished-at column should round-trip")
	})

	s.Run("RejectsUnsafeOptionalIdentifier", func() {
		unsafe := "col; DROP TABLE x"

		cmd := linkageCmd("linkage-unsafe")
		cmd.BusinessBinding.InstanceIDColumn = &unsafe

		_, err := s.handler.Handle(s.ctx, cmd)
		s.Require().Error(err, "An unsafe optional identifier must be rejected")
		s.Assert().ErrorIs(err, shared.ErrInvalidBusinessIdentifier, "Optional columns share the SQL-identifier whitelist")
	})

	s.Run("RejectsDuplicateWriteColumns", func() {
		duplicate := status

		cmd := linkageCmd("linkage-duplicate")
		cmd.BusinessBinding.FinishedAtColumn = &duplicate

		_, err := s.handler.Handle(s.ctx, cmd)
		s.Require().Error(err, "Two binding fields naming the same column must be rejected")
		s.Assert().ErrorIs(err, shared.ErrBindingColumnsConflict, "Duplicate write columns would render SET col = ?, col = ?")
	})
}

// TestInitiatorPolicyIsExclusive pins the two initiation settings as mutually
// exclusive in both directions. Permission checking short-circuits on
// isAllInitiationAllowed and never reads the rules, so the pairing that used to
// be accepted — open to everyone WITH rules — displayed a restriction that did
// not hold; and a restricted flow with no rules could be started by nobody. The
// invariant also makes an empty initiator list mean exactly "open to everyone",
// which is what lets one query answer who may start a flow.
func (s *CreateFlowTestSuite) TestInitiatorPolicyIsExclusive() {
	baseCmd := func(code string) command.CreateFlowCmd {
		return command.CreateFlowCmd{
			TenantID:              "tenant-policy",
			Code:                  code,
			Name:                  "Initiator Policy Flow",
			CategoryID:            s.categoryID,
			BindingMode:           approval.BindingStandalone,
			InstanceTitleTemplate: "Template",
			Caller:                approval.SystemCaller,
		}
	}

	s.Run("RejectsInitiatorsOnAFlowOpenToEveryone", func() {
		cmd := baseCmd("policy-open-with-rules")
		cmd.IsAllInitiationAllowed = true
		cmd.Initiators = []shared.CreateFlowInitiatorCmd{
			{Kind: approval.InitiatorUser, IDs: []string{"user-1"}},
		}

		_, err := s.handler.Handle(s.ctx, cmd)
		s.Require().Error(err, "Rules on an open flow must be rejected")
		s.Assert().ErrorIs(err, shared.ErrInitiatorsNotAllowed,
			"The rejection must name the exclusivity rule")
	})

	s.Run("RejectsARestrictedFlowWithNoInitiators", func() {
		cmd := baseCmd("policy-restricted-empty")
		cmd.IsAllInitiationAllowed = false

		_, err := s.handler.Handle(s.ctx, cmd)
		s.Require().Error(err, "A restricted flow with no rules must be rejected")
		s.Assert().ErrorIs(err, shared.ErrInitiatorsRequired,
			"The rejection must name the missing rules")
	})

	// Rule count is not the invariant: a rule selecting nobody is matched
	// against no applicant, so it leaves the flow unstartable while making the
	// stored list look restricted. The wizard filters these out, so only a
	// direct API call can submit one.
	s.Run("RejectsARestrictedFlowWhoseRuleSelectsNobody", func() {
		cmd := baseCmd("policy-restricted-blank-rule")
		cmd.IsAllInitiationAllowed = false
		cmd.Initiators = []shared.CreateFlowInitiatorCmd{
			{Kind: approval.InitiatorUser, IDs: []string{}},
		}

		_, err := s.handler.Handle(s.ctx, cmd)
		s.Require().Error(err, "A rule selecting nobody must be rejected")
		s.Assert().ErrorIs(err, shared.ErrInitiatorsRequired,
			"An empty rule names nobody, so it fails the same requirement as no rules")
	})

	s.Run("AcceptsAFlowOpenToEveryoneWithoutInitiators", func() {
		cmd := baseCmd("policy-open-clean")
		cmd.IsAllInitiationAllowed = true

		_, err := s.handler.Handle(s.ctx, cmd)
		s.Require().NoError(err, "Open to everyone with no rules is the valid open shape")
	})

	s.Run("AcceptsARestrictedFlowWithInitiators", func() {
		cmd := baseCmd("policy-restricted-rules")
		cmd.IsAllInitiationAllowed = false
		cmd.Initiators = []shared.CreateFlowInitiatorCmd{
			{Kind: approval.InitiatorUser, IDs: []string{"user-1"}},
		}

		_, err := s.handler.Handle(s.ctx, cmd)
		s.Require().NoError(err, "Restricted with rules is the valid restricted shape")
	})
}
