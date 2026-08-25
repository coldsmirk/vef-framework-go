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
		return &FindMyCompletedTasksTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// FindMyCompletedTasksTestSuite tests the FindMyCompletedTasksHandler.
type FindMyCompletedTasksTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.FindMyCompletedTasksHandler
}

func (s *FindMyCompletedTasksTestSuite) SetupSuite() {
	s.handler = query.NewFindMyCompletedTasksHandler(s.db)

	fix := setupQueryFixture(s.T(), s.ctx, s.db, "mct-flow", 1)

	inst := &approval.Instance{
		TenantID:      "t1",
		FlowID:        fix.FlowID,
		FlowVersionID: fix.VersionID,
		Title:         "Completed Instance",
		InstanceNo:    "MCT-001",
		ApplicantID:   "user-x",
		Status:        approval.InstanceApproved,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert instance")

	_, err = s.db.NewUpdate().Model(new(approval.Flow)).
		Set("labels", map[string]string{"app": "smp"}).
		Where(func(cb orm.ConditionBuilder) {
			cb.Equals("id", fix.FlowID)
		}).
		Exec(s.ctx)
	s.Require().NoError(err, "Should set flow labels")

	now := timex.Now()

	tasks := []approval.Task{
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-a", SortOrder: 1, Status: approval.TaskApproved, FinishedAt: &now},
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-a", SortOrder: 2, Status: approval.TaskRejected, FinishedAt: &now},
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-a", SortOrder: 3, Status: approval.TaskHandled, FinishedAt: &now},
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-a", SortOrder: 4, Status: approval.TaskRolledBack, FinishedAt: &now},
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-a", SortOrder: 5, Status: approval.TaskPending},
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-b", SortOrder: 6, Status: approval.TaskApproved, FinishedAt: &now},
	}
	for i := range tasks {
		tasks[i].VisitID = ensureActiveVisit(s.T(), s.ctx, s.db, tasks[i].TenantID, tasks[i].InstanceID, tasks[i].NodeID).ID
		_, err := s.db.NewInsert().Model(&tasks[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert task")
	}
}

func (s *FindMyCompletedTasksTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *FindMyCompletedTasksTestSuite) TestFindCompletedForUser() {
	result, err := s.handler.Handle(s.ctx, query.FindMyCompletedTasksQuery{
		UserID: "user-a",
		Page:   1,
		Size:   10,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(4), result.Total, "Should include approved, rejected, handled, and rolled-back tasks for user-a")
}

func (s *FindMyCompletedTasksTestSuite) TestExcludesPendingTasks() {
	result, err := s.handler.Handle(s.ctx, query.FindMyCompletedTasksQuery{
		UserID: "user-a",
		Page:   1,
		Size:   10,
	})
	s.Require().NoError(err, "Should query without error")

	for _, item := range result.Items {
		s.Assert().NotEqual("pending", item.Status, "Should not include pending tasks")
	}
}

func (s *FindMyCompletedTasksTestSuite) TestProjectsInstanceStatus() {
	result, err := s.handler.Handle(s.ctx, query.FindMyCompletedTasksQuery{
		UserID: "user-a",
		Page:   1,
		Size:   10,
	})
	s.Require().NoError(err, "Should query without error")
	s.Require().Len(result.Items, 4, "All four completed tasks should be projected")

	for _, item := range result.Items {
		s.Assert().Equal(approval.InstanceApproved, item.InstanceStatus,
			"Each row should carry the instance's current status")
	}
}

func (s *FindMyCompletedTasksTestSuite) TestProjectsFlowLabels() {
	result, err := s.handler.Handle(s.ctx, query.FindMyCompletedTasksQuery{
		UserID: "user-a",
		Page:   1,
		Size:   10,
	})
	s.Require().NoError(err, "Should query without error")
	s.Require().NotEmpty(result.Items, "Completed tasks should be projected")

	for _, item := range result.Items {
		s.Assert().Equal(map[string]string{"app": "smp"}, item.Labels,
			"Each row should carry the flow's labels")
	}
}

func (s *FindMyCompletedTasksTestSuite) TestNoResults() {
	result, err := s.handler.Handle(s.ctx, query.FindMyCompletedTasksQuery{
		UserID: "non-existent-user",
		Page:   1,
		Size:   10,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(0), result.Total, "Should find 0 completed tasks")
	s.Assert().Empty(result.Items, "Should return empty slice")
}
