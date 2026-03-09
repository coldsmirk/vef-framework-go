package query_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/page"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &FindMyCCRecordsTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// FindMyCCRecordsTestSuite tests the FindMyCCRecordsHandler.
type FindMyCCRecordsTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.FindMyCCRecordsHandler
}

func (s *FindMyCCRecordsTestSuite) SetupSuite() {
	s.handler = query.NewFindMyCCRecordsHandler(s.db)

	fix := setupQueryFixture(s.T(), s.ctx, s.db, "mcc-flow", 2)

	inst := &approval.Instance{
		TenantID: "t1", FlowID: fix.FlowID, FlowVersionID: fix.VersionID,
		Title: "CC Instance", InstanceNo: "MCC-001", ApplicantID: "user-x", Status: approval.InstanceRunning,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert instance")

	now := timex.Now()
	nodeID0 := fix.NodeIDs[0]
	nodeID1 := fix.NodeIDs[1]

	records := []approval.CCRecord{
		{InstanceID: inst.ID, NodeID: &nodeID0, CCUserID: "user-a", IsManual: false},
		{InstanceID: inst.ID, NodeID: &nodeID1, CCUserID: "user-a", IsManual: false, ReadAt: &now},
		{InstanceID: inst.ID, NodeID: &nodeID0, CCUserID: "user-b", IsManual: false},
	}
	for i := range records {
		_, err := s.db.NewInsert().Model(&records[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert CC record")
	}
}

func (s *FindMyCCRecordsTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *FindMyCCRecordsTestSuite) TestFindAllForUser() {
	result, err := s.handler.Handle(s.ctx, query.FindMyCCRecordsQuery{
		UserID:   "user-a",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(2), result.Total, "Should find 2 CC records for user-a")
}

func (s *FindMyCCRecordsTestSuite) TestFilterUnread() {
	isRead := false
	result, err := s.handler.Handle(s.ctx, query.FindMyCCRecordsQuery{
		UserID:   "user-a",
		IsRead:   &isRead,
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(1), result.Total, "Should find 1 unread CC record")
	s.Assert().False(result.Items[0].IsRead, "Should be unread")
}

func (s *FindMyCCRecordsTestSuite) TestFilterRead() {
	isRead := true
	result, err := s.handler.Handle(s.ctx, query.FindMyCCRecordsQuery{
		UserID:   "user-a",
		IsRead:   &isRead,
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(1), result.Total, "Should find 1 read CC record")
	s.Assert().True(result.Items[0].IsRead, "Should be read")
}

func (s *FindMyCCRecordsTestSuite) TestNoResults() {
	result, err := s.handler.Handle(s.ctx, query.FindMyCCRecordsQuery{
		UserID:   "non-existent-user",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(0), result.Total, "Should find 0 CC records")
	s.Assert().Empty(result.Items, "Should return empty slice")
}
