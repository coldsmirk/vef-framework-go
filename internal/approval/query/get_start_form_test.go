package query_test

import (
	"context"
	"encoding/json"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// mustInitiatorComposite builds the framework's own initiator vocabulary.
// Registration is static, so a failure here is a programming error rather than
// a test condition.
func mustInitiatorComposite(svc approval.AssigneeService) *strategy.CompositeInitiatorResolver {
	composite, err := strategy.NewCompositeInitiatorResolver(strategy.BuiltinInitiatorResolvers(svc), nil)
	if err != nil {
		panic(err)
	}

	return composite
}

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &GetStartFormTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// GetStartFormTestSuite tests the GetStartFormHandler.
type GetStartFormTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.GetStartFormHandler
}

func (s *GetStartFormTestSuite) SetupSuite() {
	s.handler = query.NewGetStartFormHandler(s.db, service.NewValidationService(mustInitiatorComposite(nil)))

	category := &approval.FlowCategory{
		TenantID: "default",
		Code:     "gsf-cat",
		Name:     "Start Form Category",
	}
	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "Should insert category")

	// Flow 1: all initiation allowed, active, published with a form schema.
	flow1 := &approval.Flow{
		TenantID:               "default",
		CategoryID:             category.ID,
		Code:                   "gsf-open",
		Name:                   "Open Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Test",
		IsActive:               true,
	}
	_, err = s.db.NewInsert().Model(flow1).Exec(s.ctx)
	s.Require().NoError(err, "Should insert open flow")

	_, err = s.db.NewInsert().Model(&approval.FlowVersion{
		FlowID:     flow1.ID,
		Version:    3,
		Status:     approval.VersionPublished,
		FormSchema: json.RawMessage(`{"fields": [{"key": "amount"}]}`),
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should insert published version")

	// Flow 2: restricted to user-a, active, published.
	flow2 := &approval.Flow{
		TenantID:               "default",
		CategoryID:             category.ID,
		Code:                   "gsf-restricted",
		Name:                   "Restricted Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: false,
		InstanceTitleTemplate:  "Test",
		IsActive:               true,
	}
	_, err = s.db.NewInsert().Model(flow2).Exec(s.ctx)
	s.Require().NoError(err, "Should insert restricted flow")

	_, err = s.db.NewInsert().Model(&approval.FlowInitiator{
		FlowID: flow2.ID,
		Kind:   approval.InitiatorUser,
		IDs:    []string{"user-a"},
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should insert initiator")

	_, err = s.db.NewInsert().Model(&approval.FlowVersion{
		FlowID:  flow2.ID,
		Version: 1,
		Status:  approval.VersionPublished,
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should insert restricted published version")

	// Flow 3: inactive.
	flow3 := &approval.Flow{
		TenantID:               "default",
		CategoryID:             category.ID,
		Code:                   "gsf-inactive",
		Name:                   "Inactive Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Test",
		IsActive:               false,
	}
	_, err = s.db.NewInsert().Model(flow3).Exec(s.ctx)
	s.Require().NoError(err, "Should insert inactive flow")

	// Flow 4: active but never published.
	flow4 := &approval.Flow{
		TenantID:               "default",
		CategoryID:             category.ID,
		Code:                   "gsf-unpublished",
		Name:                   "Unpublished Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Test",
		IsActive:               true,
	}
	_, err = s.db.NewInsert().Model(flow4).Exec(s.ctx)
	s.Require().NoError(err, "Should insert unpublished flow")

	// Shared flows under a shared category: one open everywhere, and one the
	// default tenant replaces with an inactive flow of the same code.
	sharedCategory := &approval.FlowCategory{
		TenantID: approval.SharedTenantID,
		Code:     "gsf-shared-cat",
		Name:     "Shared Start Form Category",
	}
	_, err = s.db.NewInsert().Model(sharedCategory).Exec(s.ctx)
	s.Require().NoError(err, "Should insert shared category")

	for _, flow := range []*approval.Flow{
		{TenantID: approval.SharedTenantID, CategoryID: sharedCategory.ID, Code: "gsf-shared", Name: "Shared Flow", IsActive: true},
		{TenantID: approval.SharedTenantID, CategoryID: sharedCategory.ID, Code: "gsf-replaced", Name: "Replaced Shared Flow", IsActive: true},
		{TenantID: "default", CategoryID: category.ID, Code: "gsf-replaced", Name: "Tenant Replacement"},
	} {
		flow.BindingMode = approval.BindingStandalone
		flow.IsAllInitiationAllowed = true
		flow.InstanceTitleTemplate = "Test"

		_, err = s.db.NewInsert().Model(flow).Exec(s.ctx)
		s.Require().NoError(err, "Should insert flow %s of tenant %s", flow.Code, flow.TenantID)

		_, err = s.db.NewInsert().Model(&approval.FlowVersion{
			FlowID:  flow.ID,
			Version: 1,
			Status:  approval.VersionPublished,
		}).Exec(s.ctx)
		s.Require().NoError(err, "Should publish flow %s of tenant %s", flow.Code, flow.TenantID)
	}
}

func (s *GetStartFormTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *GetStartFormTestSuite) TestGetStartForm() {
	s.Run("ReturnsPublishedForm", func() {
		form, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			TenantID:  "default",
			FlowCode:  "gsf-open",
			Applicant: approval.UserInfo{ID: "user-z", Name: "User Z"},
			Caller:    defaultTenantCaller,
		})
		s.Require().NoError(err, "All-allowed flow should be loadable by anyone")
		s.Assert().Equal("gsf-open", form.FlowCode, "Flow code should round-trip")
		s.Assert().Equal("Open Flow", form.FlowName, "Flow name should round-trip")
		s.Assert().Equal(3, form.Version, "Should surface the published version number")
		s.Assert().NotEmpty(form.VersionID, "Should surface the published version id")
		s.Assert().JSONEq(`{"fields": [{"key": "amount"}]}`, string(form.FormSchema), "Form schema should be returned verbatim")
		s.Assert().Equal(approval.UserInfo{ID: "user-z", Name: "User Z"}, form.Applicant,
			"Should return the applicant snapshot the instance would record, the form's $applicant")
	})

	s.Run("DefaultsTenant", func() {
		form, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			FlowCode:  "gsf-open",
			Applicant: approval.UserInfo{ID: "user-z", Name: "User Z"},
			Caller:    approval.SystemCaller,
		})
		s.Require().NoError(err, "A privileged caller naming no tenant should fall back to the default tenant")
		s.Assert().Equal("gsf-open", form.FlowCode, "Should resolve the default-tenant flow")
	})

	s.Run("AllowsMatchedInitiator", func() {
		form, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			TenantID:  "default",
			FlowCode:  "gsf-restricted",
			Applicant: approval.UserInfo{ID: "user-a", Name: "User A"},
			Caller:    defaultTenantCaller,
		})
		s.Require().NoError(err, "Configured initiator should pass the gate")
		s.Assert().Equal("gsf-restricted", form.FlowCode, "Flow code should round-trip")
	})

	s.Run("DeniesUnmatchedInitiator", func() {
		_, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			TenantID:  "default",
			FlowCode:  "gsf-restricted",
			Applicant: approval.UserInfo{ID: "user-z", Name: "User Z"},
			Caller:    defaultTenantCaller,
		})
		s.Require().ErrorIs(err, approval.ErrNotAllowedInitiate, "Non-initiator should be denied")
	})

	s.Run("RejectsInactiveFlow", func() {
		_, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			TenantID:  "default",
			FlowCode:  "gsf-inactive",
			Applicant: approval.UserInfo{ID: "user-z", Name: "User Z"},
			Caller:    defaultTenantCaller,
		})
		s.Require().ErrorIs(err, approval.ErrFlowNotActive, "Inactive flow should be rejected")
	})

	s.Run("RejectsUnpublishedFlow", func() {
		_, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			TenantID:  "default",
			FlowCode:  "gsf-unpublished",
			Applicant: approval.UserInfo{ID: "user-z", Name: "User Z"},
			Caller:    defaultTenantCaller,
		})
		s.Require().ErrorIs(err, approval.ErrNoPublishedVersion, "Flow without a published version should be rejected")
	})

	s.Run("RejectsUnknownFlow", func() {
		_, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			TenantID:  "default",
			FlowCode:  "gsf-missing",
			Applicant: approval.UserInfo{ID: "user-z", Name: "User Z"},
			Caller:    defaultTenantCaller,
		})
		s.Require().ErrorIs(err, approval.ErrFlowNotFound, "Unknown flow code should be rejected")
	})

	s.Run("RejectsCrossTenantCode", func() {
		_, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			FlowCode:  "gsf-open",
			Applicant: approval.UserInfo{ID: "user-z", Name: "User Z"},
			Caller:    approval.CallerContext{TenantID: "tenant-other"},
		})
		s.Require().ErrorIs(err, approval.ErrFlowNotFound, "Flow lookup should be tenant-scoped")
	})

	s.Run("PinsOrdinaryCallerToOwnTenant", func() {
		_, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			TenantID:  "default",
			FlowCode:  "gsf-open",
			Applicant: approval.UserInfo{ID: "user-z", Name: "User Z"},
			Caller:    approval.CallerContext{TenantID: "tenant-other"},
		})
		s.Require().ErrorIs(err, approval.ErrFlowNotFound,
			"An ordinary caller naming another tenant must still be confined to its own")
	})

	s.Run("FailsClosedWithoutTenant", func() {
		_, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			TenantID:  "default",
			FlowCode:  "gsf-open",
			Applicant: approval.UserInfo{ID: "user-z", Name: "User Z"},
		})
		s.Require().ErrorIs(err, approval.ErrFlowNotFound, "A caller without a tenant must see no flow at all")
	})

	s.Run("OpensSharedFlowInAnyTenant", func() {
		form, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			FlowCode:  "gsf-shared",
			Applicant: approval.UserInfo{ID: "user-z", Name: "User Z"},
			Caller:    approval.CallerContext{TenantID: "tenant-other"},
		})
		s.Require().NoError(err, "A shared flow should open in a tenant without a flow of that code")
		s.Assert().Equal("Shared Flow", form.FlowName, "Should resolve the shared flow")
	})

	s.Run("TenantFlowReplacesSharedFlow", func() {
		_, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			FlowCode:  "gsf-replaced",
			Applicant: approval.UserInfo{ID: "user-z", Name: "User Z"},
			Caller:    defaultTenantCaller,
		})
		s.Require().ErrorIs(err, approval.ErrFlowNotActive,
			"The tenant's own inactive flow, not the shared one, is what its code names in the tenant")

		form, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			FlowCode:  "gsf-replaced",
			Applicant: approval.UserInfo{ID: "user-z", Name: "User Z"},
			Caller:    approval.CallerContext{TenantID: "tenant-other"},
		})
		s.Require().NoError(err, "A tenant without its own flow of the code still gets the shared one")
		s.Assert().Equal("Replaced Shared Flow", form.FlowName, "Should resolve the shared flow elsewhere")
	})

	s.Run("RejectsSharedTenantAsStartTenant", func() {
		_, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			TenantID:  approval.SharedTenantID,
			FlowCode:  "gsf-shared",
			Applicant: approval.UserInfo{ID: "user-z", Name: "User Z"},
			Caller:    approval.SystemCaller,
		})
		s.Require().ErrorIs(err, approval.ErrFlowNotFound, "Nothing starts in the shared tenant")
	})
}
