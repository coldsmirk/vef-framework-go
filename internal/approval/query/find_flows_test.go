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
		{
			TenantID:    "t1",
			CategoryID:  s.categoryID1,
			Code:        "flow1",
			Name:        "Leave Flow",
			IsActive:    true,
			BindingMode: approval.BindingStandalone,
			Labels:      map[string]string{"app": "crm", "mobile": "true"},
		},
		{TenantID: "t1", CategoryID: s.categoryID1, Code: "flow2", Name: "Expense Flow", IsActive: false, BindingMode: approval.BindingStandalone, Labels: map[string]string{"beta": ""}},
		{TenantID: "t1", CategoryID: s.categoryID2, Code: "flow3", Name: "Travel Flow", IsActive: true, BindingMode: approval.BindingBusiness, Labels: map[string]string{"app": "erp"}},
		{TenantID: "t2", CategoryID: s.categoryID2, Code: "flow4", Name: "Purchase Flow", IsActive: true, BindingMode: approval.BindingBusiness, Labels: map[string]string{"app": "crm"}},
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
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(4), result.Total, "Should find 4 flows")
	s.Assert().Len(result.Items, 4, "Should return 4 items")
}

func (s *FindFlowsTestSuite) TestFilterByTenant() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		TenantID: new("t1"),
		Pageable: page.Pageable{Page: 1, Size: 10},
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(3), result.Total, "Should find 3 flows in tenant t1")
}

func (s *FindFlowsTestSuite) TestFilterByCategory() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		CategoryID: new(s.categoryID1),
		Pageable:   page.Pageable{Page: 1, Size: 10},
		Caller:     approval.SystemCaller,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(2), result.Total, "Should find 2 flows in category 1")
}

func (s *FindFlowsTestSuite) TestFilterByIsActive() {
	isActive := true
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		IsActive: &isActive,
		Pageable: page.Pageable{Page: 1, Size: 10},
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(3), result.Total, "Should find 3 active flows")
}

func (s *FindFlowsTestSuite) TestFilterByBindingMode() {
	s.Run("Standalone", func() {
		result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
			BindingMode: new(approval.BindingStandalone),
			Pageable:    page.Pageable{Page: 1, Size: 10},
			Caller:      approval.SystemCaller,
		})
		s.Require().NoError(err, "Should query without error")
		s.Assert().Equal(int64(2), result.Total, "Should find the 2 standalone flows")
	})

	s.Run("Business", func() {
		result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
			BindingMode: new(approval.BindingBusiness),
			Pageable:    page.Pageable{Page: 1, Size: 10},
			Caller:      approval.SystemCaller,
		})
		s.Require().NoError(err, "Should query without error")
		s.Require().Equal(int64(2), result.Total, "Should find the 2 business-bound flows")
		codes := []string{result.Items[0].Code, result.Items[1].Code}
		s.Assert().ElementsMatch([]string{"flow3", "flow4"}, codes, "Only the business-bound flows should match")
	})

	s.Run("CombinesWithOtherFilters", func() {
		result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
			TenantID:    new("t1"),
			BindingMode: new(approval.BindingBusiness),
			Pageable:    page.Pageable{Page: 1, Size: 10},
			Caller:      approval.SystemCaller,
		})
		s.Require().NoError(err, "Should query without error")
		s.Require().Equal(int64(1), result.Total, "Tenant scope should exclude flow4 from the business match")
		s.Assert().Equal("flow3", result.Items[0].Code, "Only flow3 is business-bound within tenant t1")
	})

	s.Run("NoMatch", func() {
		result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
			BindingMode: new(approval.BindingMode("unknown")),
			Pageable:    page.Pageable{Page: 1, Size: 10},
			Caller:      approval.SystemCaller,
		})
		s.Require().NoError(err, "Should query without error")
		s.Assert().Equal(int64(0), result.Total, "An out-of-enum mode should match no flow")
	})
}

func (s *FindFlowsTestSuite) TestKeywordSearch() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		Keyword:  new("Flow"),
		Pageable: page.Pageable{Page: 1, Size: 10},
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(4), result.Total, "Should find 4 flows with 'Flow' in name")
}

func (s *FindFlowsTestSuite) TestFilterByLabels() {
	s.Run("SingleLabel", func() {
		result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
			Labels:   map[string]string{"app": "crm"},
			Pageable: page.Pageable{Page: 1, Size: 10},
			Caller:   approval.SystemCaller,
		})
		s.Require().NoError(err, "Should query without error")
		s.Assert().Equal(int64(2), result.Total, "app=crm should match flow1 and flow4")
	})

	s.Run("MultipleLabelsAnd", func() {
		result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
			Labels:   map[string]string{"app": "crm", "mobile": "true"},
			Pageable: page.Pageable{Page: 1, Size: 10},
			Caller:   approval.SystemCaller,
		})
		s.Require().NoError(err, "Should query without error")
		s.Require().Equal(int64(1), result.Total, "Label pairs must AND-combine, leaving only flow1")
		s.Assert().Equal("flow1", result.Items[0].Code, "Only flow1 carries both labels")
	})

	s.Run("CombinesWithOtherFilters", func() {
		result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
			TenantID: new("t1"),
			Labels:   map[string]string{"app": "crm"},
			Pageable: page.Pageable{Page: 1, Size: 10},
			Caller:   approval.SystemCaller,
		})
		s.Require().NoError(err, "Should query without error")
		s.Require().Equal(int64(1), result.Total, "Tenant scope should exclude flow4 from the app=crm match")
		s.Assert().Equal("flow1", result.Items[0].Code, "Only flow1 is app=crm within tenant t1")
	})

	s.Run("EmptyValueMatches", func() {
		result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
			Labels:   map[string]string{"beta": ""},
			Pageable: page.Pageable{Page: 1, Size: 10},
			Caller:   approval.SystemCaller,
		})
		s.Require().NoError(err, "Should query without error")
		s.Require().Equal(int64(1), result.Total, "Presence-style empty-value label should match flow2")
		s.Assert().Equal("flow2", result.Items[0].Code, "Only flow2 carries the beta flag")
	})

	s.Run("NoMatch", func() {
		result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
			Labels:   map[string]string{"app": "nonexistent"},
			Pageable: page.Pageable{Page: 1, Size: 10},
			Caller:   approval.SystemCaller,
		})
		s.Require().NoError(err, "Should query without error")
		s.Assert().Equal(int64(0), result.Total, "Unmatched label value should exclude every flow, labeled or not")
	})

	s.Run("LabelsReturnedInItems", func() {
		result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
			Labels:   map[string]string{"mobile": "true"},
			Pageable: page.Pageable{Page: 1, Size: 10},
			Caller:   approval.SystemCaller,
		})
		s.Require().NoError(err, "Should query without error")
		s.Require().Equal(int64(1), result.Total, "mobile=true should match only flow1")
		s.Assert().Equal(map[string]string{"app": "crm", "mobile": "true"}, result.Items[0].Labels,
			"Stored labels should round-trip through the list projection")
	})
}

func (s *FindFlowsTestSuite) TestPagination() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		Pageable: page.Pageable{Page: 1, Size: 2},
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(4), result.Total, "Total should be 4")
	s.Assert().Len(result.Items, 2, "Page 1 should return 2 items")
}

func (s *FindFlowsTestSuite) TestEmpty() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		TenantID: new("non-existent-tenant"),
		Pageable: page.Pageable{Page: 1, Size: 10},
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(0), result.Total, "Should find 0 flows")
	s.Assert().Empty(result.Items, "Should return empty slice")
}

func (s *FindFlowsTestSuite) TestNonSuperAdminIgnoresForeignTenantOverride() {
	// A non-super-admin caller in t1 supplying a foreign override (t2) must be
	// pinned to its own tenant via TenantScopeFilter, so it sees only t1 rows
	// (3 flows) and never the t2 flow — the override carries no authority.
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		TenantID: new("t2"),
		Pageable: page.Pageable{Page: 1, Size: 10},
		Caller:   approval.CallerContext{TenantID: "t1"},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(3), result.Total, "Non-super-admin caller is confined to its own tenant regardless of override")

	for _, flow := range result.Items {
		s.Assert().Equal("t1", flow.TenantID, "Only own-tenant rows must be returned")
	}
}
