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
		return &FindMyInitiatedTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// FindMyInitiatedTestSuite tests the FindMyInitiatedHandler.
type FindMyInitiatedTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.FindMyInitiatedHandler
}

func (s *FindMyInitiatedTestSuite) SetupSuite() {
	s.handler = query.NewFindMyInitiatedHandler(s.db)

	fix1 := setupQueryFixture(s.T(), s.ctx, s.db, "mi-flow1", 1)
	fix2 := setupQueryFixture(s.T(), s.ctx, s.db, "mi-flow2", 0)

	instances := []approval.Instance{
		{
			TenantID:      "t1",
			FlowID:        fix1.FlowID,
			FlowVersionID: fix1.VersionID,
			Title:         "My Leave",
			InstanceNo:    "MI-001",
			ApplicantID:   "user-a",
			Status:        approval.InstanceRunning,
			CurrentNodeID: &fix1.NodeIDs[0],
		},
		{TenantID: "t1", FlowID: fix1.FlowID, FlowVersionID: fix1.VersionID, Title: "My Expense", InstanceNo: "MI-002", ApplicantID: "user-a", Status: approval.InstanceApproved},
		{TenantID: "t1", FlowID: fix2.FlowID, FlowVersionID: fix2.VersionID, Title: "My Travel", InstanceNo: "MI-003", ApplicantID: "user-a", Status: approval.InstanceRejected},
		{TenantID: "t2", FlowID: fix2.FlowID, FlowVersionID: fix2.VersionID, Title: "Other User", InstanceNo: "MI-004", ApplicantID: "user-b", Status: approval.InstanceRunning},
	}
	for i := range instances {
		_, err := s.db.NewInsert().Model(&instances[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert test instance")
	}
}

func (s *FindMyInitiatedTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *FindMyInitiatedTestSuite) TestFindAllForUser() {
	result, err := s.handler.Handle(s.ctx, query.FindMyInitiatedQuery{
		UserID:   "user-a",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(3), result.Total, "Should find 3 instances for user-a")
	s.Assert().Len(result.Items, 3, "Should return 3 items")
}

func (s *FindMyInitiatedTestSuite) TestFilterByStatus() {
	result, err := s.handler.Handle(s.ctx, query.FindMyInitiatedQuery{
		UserID:   "user-a",
		Status:   string(approval.InstanceRunning),
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(1), result.Total, "Should find 1 running instance")
}

func (s *FindMyInitiatedTestSuite) TestFilterByKeyword() {
	result, err := s.handler.Handle(s.ctx, query.FindMyInitiatedQuery{
		UserID:   "user-a",
		Keyword:  "Expense",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(1), result.Total, "Should find 1 instance matching keyword")
	s.Assert().Equal("My Expense", result.Items[0].Title, "Should match the correct instance")
}

func (s *FindMyInitiatedTestSuite) TestCurrentNodeName() {
	result, err := s.handler.Handle(s.ctx, query.FindMyInitiatedQuery{
		UserID:   "user-a",
		Status:   string(approval.InstanceRunning),
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Require().Len(result.Items, 1, "Should have 1 running instance")
	s.Assert().NotNil(result.Items[0].CurrentNodeName, "Should have current node name")
}

func (s *FindMyInitiatedTestSuite) TestPagination() {
	result, err := s.handler.Handle(s.ctx, query.FindMyInitiatedQuery{
		UserID:   "user-a",
		Pageable: page.Pageable{Page: 1, Size: 2},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(3), result.Total, "Total should be 3")
	s.Assert().Len(result.Items, 2, "Page 1 should return 2 items")
}

func (s *FindMyInitiatedTestSuite) TestNoResults() {
	result, err := s.handler.Handle(s.ctx, query.FindMyInitiatedQuery{
		UserID:   "non-existent-user",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(0), result.Total, "Should find 0 instances")
	s.Assert().Empty(result.Items, "Should return empty slice")
}
