package query_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &FindFlowVersionsTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// FindFlowVersionsTestSuite tests the FindFlowVersionsHandler.
type FindFlowVersionsTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.FindFlowVersionsHandler

	flowID1 string
	flowID2 string
}

func (s *FindFlowVersionsTestSuite) SetupSuite() {
	s.handler = query.NewFindFlowVersionsHandler(s.db)

	category := approval.FlowCategory{TenantID: "t1", Code: "cat-ver", Name: "Version Category", IsActive: true}
	_, err := s.db.NewInsert().Model(&category).Exec(s.ctx)
	s.Require().NoError(err, "Should insert category")

	flow1 := approval.Flow{TenantID: "t1", CategoryID: category.ID, Code: "flow-ver-1", Name: "Flow 1", IsActive: true}
	flow2 := approval.Flow{TenantID: "t1", CategoryID: category.ID, Code: "flow-ver-2", Name: "Flow 2", IsActive: true}
	_, err = s.db.NewInsert().Model(&flow1).Exec(s.ctx)
	s.Require().NoError(err, "Should insert flow 1")
	_, err = s.db.NewInsert().Model(&flow2).Exec(s.ctx)
	s.Require().NoError(err, "Should insert flow 2")
	s.flowID1 = flow1.ID
	s.flowID2 = flow2.ID

	versions := []approval.FlowVersion{
		{FlowID: s.flowID1, Version: 1, Status: approval.VersionDraft, StorageMode: approval.StorageJSON},
		{FlowID: s.flowID1, Version: 2, Status: approval.VersionPublished, StorageMode: approval.StorageJSON},
		{FlowID: s.flowID1, Version: 3, Status: approval.VersionDraft, StorageMode: approval.StorageJSON},
		{FlowID: s.flowID2, Version: 1, Status: approval.VersionPublished, StorageMode: approval.StorageJSON},
	}
	for i := range versions {
		_, err := s.db.NewInsert().Model(&versions[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert test version")
	}
}

func (s *FindFlowVersionsTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *FindFlowVersionsTestSuite) TestSuccessWithMultipleVersions() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowVersionsQuery{
		FlowID: s.flowID1,
		Caller: approval.SystemCaller,
	})
	s.Require().NoError(err, "Should query without error")
	s.Require().Len(result, 3, "Should find 3 versions for flow 1")

	s.Assert().Equal(3, result[0].Version, "First version should be 3 (DESC order)")
	s.Assert().Equal(2, result[1].Version, "Second version should be 2")
	s.Assert().Equal(1, result[2].Version, "Third version should be 1")

	// The summary projection must carry the metadata columns the version list
	// renders — a dropped Select column would surface here as a zero value.
	s.Assert().NotEmpty(result[0].ID, "Summary should carry the version id")
	s.Assert().Equal(s.flowID1, result[0].FlowID, "Summary should carry the flow id")
	s.Assert().Equal(approval.VersionDraft, result[0].Status, "Summary should carry the status")
	s.Assert().Equal(approval.StorageJSON, result[0].StorageMode, "Summary should carry the storage mode")
	s.Assert().False(result[0].CreatedAt.Unwrap().IsZero(), "Summary should carry the deploy time")
}

func (s *FindFlowVersionsTestSuite) TestEmpty() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowVersionsQuery{
		FlowID: "non-existent-flow-id",
		Caller: approval.SystemCaller,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Empty(result, "Should return empty slice for non-existent flow")
}

func (s *FindFlowVersionsTestSuite) TestForeignTenantReturnsEmptyNonNil() {
	// Fixture flows belong to t1. A non-super-admin caller in t2 must get an
	// empty (non-nil) slice — opaque, indistinguishable from "no such flow".
	result, err := s.handler.Handle(s.ctx, query.FindFlowVersionsQuery{
		FlowID: s.flowID1,
		Caller: approval.CallerContext{TenantID: "t2"},
	})
	s.Require().NoError(err, "Cross-tenant access must be an opaque empty result, not an error")
	s.Require().NotNil(result, "Result must be a non-nil empty slice for response-shape uniformity")
	s.Assert().Empty(result, "A foreign-tenant caller must see no versions")
}

func (s *FindFlowVersionsTestSuite) TestMismatchedQueryTenantReturnsEmpty() {
	// Even an authorized caller (super-admin) gets empty when the explicit
	// query.TenantID does not match the flow's tenant.
	mismatch := "t2"
	result, err := s.handler.Handle(s.ctx, query.FindFlowVersionsQuery{
		FlowID:   s.flowID1,
		TenantID: &mismatch,
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "Mismatched tenant filter must not error")
	s.Require().NotNil(result, "Result must be a non-nil empty slice")
	s.Assert().Empty(result, "A query.TenantID that does not match the flow's tenant returns no versions")
}
