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
		Page: 1,
		Size: 10,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(4), result.Total, "Should find 4 flows")
	s.Assert().Len(result.Items, 4, "Should return 4 items")
}

func (s *FindFlowsTestSuite) TestFilterByTenant() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		TenantID: new("t1"),
		Page:     1,
		Size:     10,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(3), result.Total, "Should find 3 flows in tenant t1")
}

func (s *FindFlowsTestSuite) TestFilterByCategory() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		CategoryID: new(s.categoryID1),
		Page:       1,
		Size:       10,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(2), result.Total, "Should find 2 flows in category 1")
}

func (s *FindFlowsTestSuite) TestFilterByIsActive() {
	isActive := true
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		IsActive: &isActive,
		Page:     1,
		Size:     10,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(3), result.Total, "Should find 3 active flows")
}

func (s *FindFlowsTestSuite) TestFilterByBindingMode() {
	s.Run("Standalone", func() {
		result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
			BindingMode: new(approval.BindingStandalone),
			Page:        1,
			Size:        10,
		})
		s.Require().NoError(err, "Should query without error")
		s.Assert().Equal(int64(2), result.Total, "Should find the 2 standalone flows")
	})

	s.Run("Business", func() {
		result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
			BindingMode: new(approval.BindingBusiness),
			Page:        1,
			Size:        10,
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
			Page:        1,
			Size:        10,
		})
		s.Require().NoError(err, "Should query without error")
		s.Require().Equal(int64(1), result.Total, "Tenant scope should exclude flow4 from the business match")
		s.Assert().Equal("flow3", result.Items[0].Code, "Only flow3 is business-bound within tenant t1")
	})

	s.Run("NoMatch", func() {
		result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
			BindingMode: new(approval.BindingMode("unknown")),
			Page:        1,
			Size:        10,
		})
		s.Require().NoError(err, "Should query without error")
		s.Assert().Equal(int64(0), result.Total, "An out-of-enum mode should match no flow")
	})
}

func (s *FindFlowsTestSuite) TestKeywordSearch() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		Keyword: new("Flow"),
		Page:    1,
		Size:    10,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(4), result.Total, "Should find 4 flows with 'Flow' in name")
}

func (s *FindFlowsTestSuite) TestFilterByLabels() {
	s.Run("SingleLabel", func() {
		result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
			Labels: map[string]string{"app": "crm"},
			Page:   1,
			Size:   10,
		})
		s.Require().NoError(err, "Should query without error")
		s.Assert().Equal(int64(2), result.Total, "app=crm should match flow1 and flow4")
	})

	s.Run("MultipleLabelsAnd", func() {
		result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
			Labels: map[string]string{"app": "crm", "mobile": "true"},
			Page:   1,
			Size:   10,
		})
		s.Require().NoError(err, "Should query without error")
		s.Require().Equal(int64(1), result.Total, "Label pairs must AND-combine, leaving only flow1")
		s.Assert().Equal("flow1", result.Items[0].Code, "Only flow1 carries both labels")
	})

	s.Run("CombinesWithOtherFilters", func() {
		result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
			TenantID: new("t1"),
			Labels:   map[string]string{"app": "crm"},
			Page:     1,
			Size:     10,
		})
		s.Require().NoError(err, "Should query without error")
		s.Require().Equal(int64(1), result.Total, "Tenant scope should exclude flow4 from the app=crm match")
		s.Assert().Equal("flow1", result.Items[0].Code, "Only flow1 is app=crm within tenant t1")
	})

	s.Run("EmptyValueMatches", func() {
		result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
			Labels: map[string]string{"beta": ""},
			Page:   1,
			Size:   10,
		})
		s.Require().NoError(err, "Should query without error")
		s.Require().Equal(int64(1), result.Total, "Presence-style empty-value label should match flow2")
		s.Assert().Equal("flow2", result.Items[0].Code, "Only flow2 carries the beta flag")
	})

	s.Run("NoMatch", func() {
		result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
			Labels: map[string]string{"app": "nonexistent"},
			Page:   1,
			Size:   10,
		})
		s.Require().NoError(err, "Should query without error")
		s.Assert().Equal(int64(0), result.Total, "Unmatched label value should exclude every flow, labeled or not")
	})

	s.Run("LabelsReturnedInItems", func() {
		result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
			Labels: map[string]string{"mobile": "true"},
			Page:   1,
			Size:   10,
		})
		s.Require().NoError(err, "Should query without error")
		s.Require().Equal(int64(1), result.Total, "mobile=true should match only flow1")
		s.Assert().Equal(map[string]string{"app": "crm", "mobile": "true"}, result.Items[0].Labels,
			"Stored labels should round-trip through the list projection")
	})
}

func (s *FindFlowsTestSuite) TestPagination() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		Page: 1,
		Size: 2,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(4), result.Total, "Total should be 4")
	s.Assert().Len(result.Items, 2, "Page 1 should return 2 items")
}

func (s *FindFlowsTestSuite) TestEmpty() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		TenantID: new("non-existent-tenant"),
		Page:     1,
		Size:     10,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(0), result.Total, "Should find 0 flows")
	s.Assert().Empty(result.Items, "Should return empty slice")
}

func (s *FindFlowsTestSuite) TestListsTheTenantTheRequestNames() {
	// Flow definitions are managed across tenants: the list reads the tenant
	// the request names, not the caller's own.
	result, err := s.handler.Handle(s.ctx, query.FindFlowsQuery{
		TenantID: new("t2"),
		Page:     1,
		Size:     10,
	})
	s.Require().NoError(err, "Should query without error")
	s.Require().Equal(int64(1), result.Total, "Should list the one flow of the requested tenant")
	s.Assert().Equal("t2", result.Items[0].TenantID, "Only the requested tenant's rows should be returned")
}

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &FindFlowsSharedTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// FindFlowsSharedTestSuite pins IncludeShared: a tenant's list gains the
// flows other tenants share, minus the ones whose code the tenant has taken
// for a flow of its own — the flows a start by code would find there.
type FindFlowsSharedTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.FindFlowsHandler
}

func (s *FindFlowsSharedTestSuite) SetupSuite() {
	s.handler = query.NewFindFlowsHandler(s.db)

	categories := map[string]string{}

	for _, tenantID := range []string{"s1", "s2"} {
		category := approval.FlowCategory{TenantID: tenantID, Code: tenantID + "-cat", Name: tenantID + " Category", IsActive: true}
		_, err := s.db.NewInsert().Model(&category).Exec(s.ctx)
		s.Require().NoError(err, "Should insert the category of %s", tenantID)

		categories[tenantID] = category.ID
	}

	for _, flow := range []approval.Flow{
		{TenantID: "s1", Code: "s1-own", Name: "S1 Own", IsActive: true},
		{TenantID: "s1", IsShared: true, Code: "s1-shared", Name: "S1 Shared", IsActive: true},
		// s1 replaces s2's shared flow of this code with an inactive one.
		{TenantID: "s1", Code: "s2-replaced", Name: "S1 Replacement"},
		{TenantID: "s2", IsShared: true, Code: "s2-shared", Name: "S2 Shared", IsActive: true},
		{TenantID: "s2", IsShared: true, Code: "s2-replaced", Name: "S2 Replaced", IsActive: true},
		{TenantID: "s2", Code: "s2-private", Name: "S2 Private", IsActive: true},
	} {
		flow.CategoryID = categories[flow.TenantID]
		flow.BindingMode = approval.BindingStandalone
		_, err := s.db.NewInsert().Model(&flow).Exec(s.ctx)
		s.Require().NoError(err, "Should insert flow %s of %s", flow.Code, flow.TenantID)
	}
}

func (s *FindFlowsSharedTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *FindFlowsSharedTestSuite) list(q query.FindFlowsQuery) []string {
	q.Page, q.Size = 1, 50

	result, err := s.handler.Handle(s.ctx, q)
	s.Require().NoError(err, "Should query without error")

	labels := make([]string, len(result.Items))
	for i, flow := range result.Items {
		labels[i] = flow.TenantID + "/" + flow.Code
	}

	return labels
}

func (s *FindFlowsSharedTestSuite) TestTenantListsItsOwnFlows() {
	s.Assert().ElementsMatch([]string{"s1/s1-own", "s1/s1-shared", "s1/s2-replaced"},
		s.list(query.FindFlowsQuery{TenantID: new("s1")}),
		"Without IncludeShared a tenant lists the flows it owns, its shared ones included")
}

func (s *FindFlowsSharedTestSuite) TestIncludeSharedAddsOtherTenantsSharedFlows() {
	s.Assert().ElementsMatch([]string{"s1/s1-own", "s1/s1-shared", "s1/s2-replaced", "s2/s2-shared"},
		s.list(query.FindFlowsQuery{TenantID: new("s1"), IncludeShared: true}),
		"IncludeShared adds the flows other tenants share, except the ones the tenant replaced and the unshared ones")
}

func (s *FindFlowsSharedTestSuite) TestIncludeSharedCombinesWithFilters() {
	s.Assert().ElementsMatch([]string{"s1/s1-own", "s1/s1-shared", "s2/s2-shared"},
		s.list(query.FindFlowsQuery{TenantID: new("s1"), IncludeShared: true, IsActive: new(true)}),
		"The other filters apply to the shared flows too, and an inactive replacement still hides the shared flow")
}

func (s *FindFlowsSharedTestSuite) TestIncludeSharedNeedsATenant() {
	s.Assert().Len(s.list(query.FindFlowsQuery{IncludeShared: true}), 6,
		"Without TenantID every tenant's flows are listed already")
}
