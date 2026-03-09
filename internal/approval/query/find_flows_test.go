package query_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/page"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &FindFlowsTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// FindFlowsTestSuite tests the FindFlowsHandler.
type FindFlowsTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.FindFlowsHandler

	categoryID1 string
	categoryID2 string
}

func (s *FindFlowsTestSuite) SetupSuite() {
	s.handler = query.NewFindFlowsHandler(s.db)

	// Create categories
	cat1 := approval.FlowCategory{TenantID: "t1", Code: "cat1", Name: "Category 1", IsActive: true}
	cat2 := approval.FlowCategory{TenantID: "t1", Code: "cat2", Name: "Category 2", IsActive: true}
	_, err := s.db.NewInsert().Model(&cat1).Exec(s.ctx)
	s.Require().NoError(err, "Should insert category 1")
	_, err = s.db.NewInsert().Model(&cat2).Exec(s.ctx)
	s.Require().NoError(err, "Should insert category 2")
	s.categoryID1 = cat1.ID
	s.categoryID2 = cat2.ID

	// Create flows
	flows := []approval.Flow{
		{TenantID: "t1", CategoryID: s.categoryID1, Code: "flow1", Name: "Leave Flow", IsActive: true},
		{TenantID: "t1", CategoryID: s.categoryID1, Code: "flow2", Name: "Expense Flow", IsActive: false},
		{TenantID: "t1", CategoryID: s.categoryID2, Code: "flow3", Name: "Travel Flow", IsActive: true},
		{TenantID: "t2", CategoryID: s.categoryID2, Code: "flow4", Name: "Purchase Flow", IsActive: true},
	}
	for i := range flows {
		_, err := s.db.NewInsert().Model(&flows[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert test flow")
	}
}

func (s *FindFlowsTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *FindFlowsTestSuite) TestFindAll() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(4), result.Total, "Should find 4 flows")
	s.Assert().Len(result.Items, 4, "Should return 4 items")
}

func (s *FindFlowsTestSuite) TestFilterByTenant() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		TenantID: "t1",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(3), result.Total, "Should find 3 flows in tenant t1")
}

func (s *FindFlowsTestSuite) TestFilterByCategory() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		CategoryID: s.categoryID1,
		Pageable:   page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(2), result.Total, "Should find 2 flows in category 1")
}

func (s *FindFlowsTestSuite) TestFilterByIsActive() {
	isActive := true
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		IsActive: &isActive,
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(3), result.Total, "Should find 3 active flows")
}

func (s *FindFlowsTestSuite) TestKeywordSearch() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		Keyword:  "Flow",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(4), result.Total, "Should find 4 flows with 'Flow' in name")
}

func (s *FindFlowsTestSuite) TestPagination() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		Pageable: page.Pageable{Page: 1, Size: 2},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(4), result.Total, "Total should be 4")
	s.Assert().Len(result.Items, 2, "Page 1 should return 2 items")
}

func (s *FindFlowsTestSuite) TestEmpty() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		TenantID: "non-existent-tenant",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(0), result.Total, "Should find 0 flows")
	s.Assert().Empty(result.Items, "Should return empty slice")
}
