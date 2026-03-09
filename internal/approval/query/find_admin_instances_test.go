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
		return &FindAdminInstancesTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// FindAdminInstancesTestSuite tests the FindAdminInstancesHandler.
type FindAdminInstancesTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.FindAdminInstancesHandler

	flowID1 string
	flowID2 string
}

func (s *FindAdminInstancesTestSuite) SetupSuite() {
	s.handler = query.NewFindAdminInstancesHandler(s.db)

	fix1 := setupQueryFixture(s.T(), s.ctx, s.db, "adi-flow1", 1)
	fix2 := setupQueryFixture(s.T(), s.ctx, s.db, "adi-flow2", 0)
	s.flowID1 = fix1.FlowID
	s.flowID2 = fix2.FlowID

	instances := []approval.Instance{
		{
			TenantID:      "t1",
			FlowID:        fix1.FlowID,
			FlowVersionID: fix1.VersionID,
			Title:         "Admin Leave",
			InstanceNo:    "ADI-001",
			ApplicantID:   "user-a",
			Status:        approval.InstanceRunning,
			CurrentNodeID: &fix1.NodeIDs[0],
		},
		{TenantID: "t1", FlowID: fix1.FlowID, FlowVersionID: fix1.VersionID, Title: "Admin Expense", InstanceNo: "ADI-002", ApplicantID: "user-a", Status: approval.InstanceApproved},
		{TenantID: "t2", FlowID: fix2.FlowID, FlowVersionID: fix2.VersionID, Title: "Admin Travel", InstanceNo: "ADI-003", ApplicantID: "user-b", Status: approval.InstanceRejected},
	}
	for i := range instances {
		_, err := s.db.NewInsert().Model(&instances[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert test instance")
	}
}

func (s *FindAdminInstancesTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *FindAdminInstancesTestSuite) TestFindAll() {
	result, err := s.handler.Handle(s.ctx, query.FindAdminInstancesQuery{
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(3), result.Total, "Should find 3 instances")
	s.Assert().Len(result.Items, 3, "Should return 3 items")
}

func (s *FindAdminInstancesTestSuite) TestFilterByTenant() {
	result, err := s.handler.Handle(s.ctx, query.FindAdminInstancesQuery{
		TenantID: "t1",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(2), result.Total, "Should find 2 instances in tenant t1")
}

func (s *FindAdminInstancesTestSuite) TestFilterByStatus() {
	result, err := s.handler.Handle(s.ctx, query.FindAdminInstancesQuery{
		Status:   string(approval.InstanceRunning),
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(1), result.Total, "Should find 1 running instance")
}

func (s *FindAdminInstancesTestSuite) TestPagination() {
	result, err := s.handler.Handle(s.ctx, query.FindAdminInstancesQuery{
		Pageable: page.Pageable{Page: 1, Size: 2},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(3), result.Total, "Total should be 3")
	s.Assert().Len(result.Items, 2, "Page 1 should return 2 items")
}

func (s *FindAdminInstancesTestSuite) TestNoResults() {
	result, err := s.handler.Handle(s.ctx, query.FindAdminInstancesQuery{
		TenantID: "non-existent-tenant",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(0), result.Total, "Should find 0 instances")
	s.Assert().Empty(result.Items, "Should return empty slice")
}
