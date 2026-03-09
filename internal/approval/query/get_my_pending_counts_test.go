package query_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &GetMyPendingCountsTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// GetMyPendingCountsTestSuite tests the GetMyPendingCountsHandler.
type GetMyPendingCountsTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.GetMyPendingCountsHandler
}

func (s *GetMyPendingCountsTestSuite) SetupSuite() {
	s.handler = query.NewGetMyPendingCountsHandler(s.db)

	fix := setupQueryFixture(s.T(), s.ctx, s.db, "mpc-flow", 3)

	inst := &approval.Instance{
		TenantID: "t1", FlowID: fix.FlowID, FlowVersionID: fix.VersionID,
		Title: "Counts Instance", InstanceNo: "MPC-001", ApplicantID: "user-x", Status: approval.InstanceRunning,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert instance")

	// 2 pending tasks (different nodes) + 1 approved task for user-a.
	tasks := []approval.Task{
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-a", SortOrder: 1, Status: approval.TaskPending},
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[1], AssigneeID: "user-a", SortOrder: 2, Status: approval.TaskPending},
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[2], AssigneeID: "user-a", SortOrder: 3, Status: approval.TaskApproved},
	}
	for i := range tasks {
		_, err := s.db.NewInsert().Model(&tasks[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert task")
	}

	// 2 unread + 1 read CC for user-a (different nodes to avoid unique constraint).
	now := timex.Now()
	nodeID0 := fix.NodeIDs[0]
	nodeID1 := fix.NodeIDs[1]
	nodeID2 := fix.NodeIDs[2]

	ccRecords := []approval.CCRecord{
		{InstanceID: inst.ID, NodeID: &nodeID0, CCUserID: "user-a", IsManual: false},
		{InstanceID: inst.ID, NodeID: &nodeID1, CCUserID: "user-a", IsManual: false},
		{InstanceID: inst.ID, NodeID: &nodeID2, CCUserID: "user-a", IsManual: false, ReadAt: &now},
	}
	for i := range ccRecords {
		_, err := s.db.NewInsert().Model(&ccRecords[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert CC record")
	}
}

func (s *GetMyPendingCountsTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *GetMyPendingCountsTestSuite) TestCountsForUser() {
	counts, err := s.handler.Handle(s.ctx, query.GetMyPendingCountsQuery{UserID: "user-a"})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(2, counts.PendingTaskCount, "Should have 2 pending tasks")
	s.Assert().Equal(2, counts.UnreadCCCount, "Should have 2 unread CC records")
}

func (s *GetMyPendingCountsTestSuite) TestZeroCountsForUnknownUser() {
	counts, err := s.handler.Handle(s.ctx, query.GetMyPendingCountsQuery{UserID: "non-existent-user"})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(0, counts.PendingTaskCount, "Should have 0 pending tasks")
	s.Assert().Equal(0, counts.UnreadCCCount, "Should have 0 unread CC records")
}
