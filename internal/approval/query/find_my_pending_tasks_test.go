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
		return &FindMyPendingTasksTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// FindMyPendingTasksTestSuite tests the FindMyPendingTasksHandler.
type FindMyPendingTasksTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.FindMyPendingTasksHandler
}

func (s *FindMyPendingTasksTestSuite) SetupSuite() {
	s.handler = query.NewFindMyPendingTasksHandler(s.db)

	fix := setupQueryFixture(s.T(), s.ctx, s.db, "mpt-flow", 1)

	inst := &approval.Instance{
		TenantID: "t1", FlowID: fix.FlowID, FlowVersionID: fix.VersionID,
		Title: "Task Instance", InstanceNo: "MPT-001", ApplicantID: "user-x", Status: approval.InstanceRunning,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert instance")

	tasks := []approval.Task{
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-a", SortOrder: 1, Status: approval.TaskPending},
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-a", SortOrder: 2, Status: approval.TaskApproved},
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-b", SortOrder: 3, Status: approval.TaskPending},
	}
	for i := range tasks {
		_, err := s.db.NewInsert().Model(&tasks[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert task")
	}
}

func (s *FindMyPendingTasksTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *FindMyPendingTasksTestSuite) TestFindPendingForUser() {
	result, err := s.handler.Handle(s.ctx, query.FindMyPendingTasksQuery{
		UserID:   "user-a",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(1), result.Total, "Should find 1 pending task for user-a")
	s.Assert().Equal("Task Instance", result.Items[0].InstanceTitle, "Should have correct instance title")
}

func (s *FindMyPendingTasksTestSuite) TestFilterByTenant() {
	result, err := s.handler.Handle(s.ctx, query.FindMyPendingTasksQuery{
		UserID:   "user-a",
		TenantID: new("t1"),
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(1), result.Total, "Should find 1 pending task in tenant t1")
}

func (s *FindMyPendingTasksTestSuite) TestNoResults() {
	result, err := s.handler.Handle(s.ctx, query.FindMyPendingTasksQuery{
		UserID:   "non-existent-user",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(0), result.Total, "Should find 0 pending tasks")
	s.Assert().Empty(result.Items, "Should return empty slice")
}

func (s *FindMyPendingTasksTestSuite) TestExcludesNonPendingTasks() {
	result, err := s.handler.Handle(s.ctx, query.FindMyPendingTasksQuery{
		UserID:   "user-a",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(1), result.Total, "Should only count pending tasks, not approved ones")
}
