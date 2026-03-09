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
	})
	s.Require().NoError(err, "Should query without error")
	s.Require().Len(result, 3, "Should find 3 versions for flow 1")

	s.Assert().Equal(3, result[0].Version, "First version should be 3 (DESC order)")
	s.Assert().Equal(2, result[1].Version, "Second version should be 2")
	s.Assert().Equal(1, result[2].Version, "Third version should be 1")
}

func (s *FindFlowVersionsTestSuite) TestEmpty() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowVersionsQuery{
		FlowID: "non-existent-flow-id",
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Empty(result, "Should return empty slice for non-existent flow")
}
