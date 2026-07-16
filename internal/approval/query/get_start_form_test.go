package query_test

import (
	"context"
	"encoding/json"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

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
	s.handler = query.NewGetStartFormHandler(s.db, service.NewValidationService(nil))

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
}

func (s *GetStartFormTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *GetStartFormTestSuite) TestGetStartForm() {
	s.Run("ReturnsPublishedForm", func() {
		form, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			TenantID: "default",
			FlowCode: "gsf-open",
			UserID:   "user-z",
		})
		s.Require().NoError(err, "All-allowed flow should be loadable by anyone")
		s.Assert().Equal("gsf-open", form.FlowCode, "Flow code should round-trip")
		s.Assert().Equal("Open Flow", form.FlowName, "Flow name should round-trip")
		s.Assert().Equal(3, form.Version, "Should surface the published version number")
		s.Assert().NotEmpty(form.VersionID, "Should surface the published version id")
		s.Assert().JSONEq(`{"fields": [{"key": "amount"}]}`, string(form.FormSchema), "Form schema should be returned verbatim")
	})

	s.Run("DefaultsTenant", func() {
		form, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			FlowCode: "gsf-open",
			UserID:   "user-z",
		})
		s.Require().NoError(err, "Empty tenant should fall back to the default tenant")
		s.Assert().Equal("gsf-open", form.FlowCode, "Should resolve the default-tenant flow")
	})

	s.Run("AllowsMatchedInitiator", func() {
		form, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			TenantID: "default",
			FlowCode: "gsf-restricted",
			UserID:   "user-a",
		})
		s.Require().NoError(err, "Configured initiator should pass the gate")
		s.Assert().Equal("gsf-restricted", form.FlowCode, "Flow code should round-trip")
	})

	s.Run("DeniesUnmatchedInitiator", func() {
		_, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			TenantID: "default",
			FlowCode: "gsf-restricted",
			UserID:   "user-z",
		})
		s.Require().ErrorIs(err, shared.ErrNotAllowedInitiate, "Non-initiator should be denied")
	})

	s.Run("RejectsInactiveFlow", func() {
		_, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			TenantID: "default",
			FlowCode: "gsf-inactive",
			UserID:   "user-z",
		})
		s.Require().ErrorIs(err, shared.ErrFlowNotActive, "Inactive flow should be rejected")
	})

	s.Run("RejectsUnpublishedFlow", func() {
		_, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			TenantID: "default",
			FlowCode: "gsf-unpublished",
			UserID:   "user-z",
		})
		s.Require().ErrorIs(err, shared.ErrNoPublishedVersion, "Flow without a published version should be rejected")
	})

	s.Run("RejectsUnknownFlow", func() {
		_, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			TenantID: "default",
			FlowCode: "gsf-missing",
			UserID:   "user-z",
		})
		s.Require().ErrorIs(err, shared.ErrFlowNotFound, "Unknown flow code should be rejected")
	})

	s.Run("RejectsCrossTenantCode", func() {
		_, err := s.handler.Handle(s.ctx, query.GetStartFormQuery{
			TenantID: "tenant-other",
			FlowCode: "gsf-open",
			UserID:   "user-z",
		})
		s.Require().ErrorIs(err, shared.ErrFlowNotFound, "Flow lookup should be tenant-scoped")
	})
}
