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
		return &FindAdminTasksTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// FindAdminTasksTestSuite tests the FindAdminTasksHandler.
type FindAdminTasksTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.FindAdminTasksHandler
}

func (s *FindAdminTasksTestSuite) SetupSuite() {
	s.handler = query.NewFindAdminTasksHandler(s.db)

	fix := setupQueryFixture(s.T(), s.ctx, s.db, "adt-flow", 1)

	inst := &approval.Instance{
		TenantID: "t1", FlowID: fix.FlowID, FlowVersionID: fix.VersionID,
		Title: "Admin Task Instance", InstanceNo: "ADT-001", ApplicantID: "user-x", Status: approval.InstanceRunning,
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

func (s *FindAdminTasksTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *FindAdminTasksTestSuite) TestFindAll() {
	result, err := s.handler.Handle(s.ctx, query.FindAdminTasksQuery{
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(3), result.Total, "Should find 3 tasks")
	s.Assert().Len(result.Items, 3, "Should return 3 items")
}

func (s *FindAdminTasksTestSuite) TestFilterByAssignee() {
	result, err := s.handler.Handle(s.ctx, query.FindAdminTasksQuery{
		AssigneeID: "user-a",
		Pageable:   page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(2), result.Total, "Should find 2 tasks for user-a")
}

func (s *FindAdminTasksTestSuite) TestFilterByStatus() {
	result, err := s.handler.Handle(s.ctx, query.FindAdminTasksQuery{
		Status:   string(approval.TaskPending),
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(2), result.Total, "Should find 2 pending tasks")
}

func (s *FindAdminTasksTestSuite) TestNoResults() {
	result, err := s.handler.Handle(s.ctx, query.FindAdminTasksQuery{
		AssigneeID: "non-existent-user",
		Pageable:   page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(0), result.Total, "Should find 0 tasks")
	s.Assert().Empty(result.Items, "Should return empty slice")
}
