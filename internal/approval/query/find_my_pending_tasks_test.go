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
		return &FindMyPendingTasksTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// FindMyPendingTasksTestSuite tests the FindMyPendingTasksHandler.
type FindMyPendingTasksTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.FindMyPendingTasksHandler
	// purchaseFlowID is the flow of the filter fixture's second instance.
	purchaseFlowID string
}

func (s *FindMyPendingTasksTestSuite) SetupSuite() {
	s.handler = query.NewFindMyPendingTasksHandler(s.db)

	fix := setupQueryFixture(s.T(), s.ctx, s.db, "mpt-flow", 1)

	inst := &approval.Instance{
		TenantID:      "t1",
		FlowID:        fix.FlowID,
		FlowVersionID: fix.VersionID,
		Title:         "Task Instance",
		InstanceNo:    "MPT-001",
		ApplicantID:   "user-x",
		Status:        approval.InstanceRunning,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert instance")

	tasks := []approval.Task{
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-a", SortOrder: 1, Status: approval.TaskPending},
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-a", SortOrder: 2, Status: approval.TaskApproved},
		{TenantID: "t1", InstanceID: inst.ID, NodeID: fix.NodeIDs[0], AssigneeID: "user-b", SortOrder: 3, Status: approval.TaskPending},
	}
	for i := range tasks {
		tasks[i].VisitID = ensureActiveVisit(s.T(), s.ctx, s.db, tasks[i].TenantID, tasks[i].InstanceID, tasks[i].NodeID).ID
		_, err := s.db.NewInsert().Model(&tasks[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert task")
	}

	s.seedFilterFixture(fix)
}

// seedFilterFixture gives user-f one pending task on each of two instances
// that differ in every filtered attribute, so each filter keeps exactly one.
// A separate assignee keeps the user-a assertions exact.
func (s *FindMyPendingTasksTestSuite) seedFilterFixture(travelFix *QueryFixture) {
	purchaseFix := setupQueryFixture(s.T(), s.ctx, s.db, "mpt-flow-b", 1)
	s.purchaseFlowID = purchaseFix.FlowID

	travel := &approval.Instance{
		TenantID:      "t1",
		FlowID:        travelFix.FlowID,
		FlowVersionID: travelFix.VersionID,
		Title:         "Travel reimbursement",
		InstanceNo:    "MPT-101",
		ApplicantID:   "user-p",
		ApplicantName: "Alice Wang",
		Status:        approval.InstanceRunning,
	}
	purchase := &approval.Instance{
		TenantID:      "t1",
		FlowID:        purchaseFix.FlowID,
		FlowVersionID: purchaseFix.VersionID,
		Title:         "Purchase request",
		InstanceNo:    "MPT-102",
		ApplicantID:   "user-q",
		ApplicantName: "Bob Li",
		Status:        approval.InstanceRunning,
	}

	for _, inst := range []*approval.Instance{travel, purchase} {
		_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
		s.Require().NoError(err, "Should insert filter fixture instance")
	}

	insertTask(s.T(), s.ctx, s.db, &approval.Task{
		TenantID: "t1", InstanceID: travel.ID, NodeID: travelFix.NodeIDs[0], AssigneeID: "user-f",
		SortOrder: 1, Status: approval.TaskPending, CreatedAt: septemberAt(1, 10),
	})
	insertTask(s.T(), s.ctx, s.db, &approval.Task{
		TenantID: "t1", InstanceID: purchase.ID, NodeID: purchaseFix.NodeIDs[0], AssigneeID: "user-f",
		SortOrder: 1, Status: approval.TaskPending, IsTimeout: true, CreatedAt: septemberAt(10, 10),
	})
}

func (s *FindMyPendingTasksTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *FindMyPendingTasksTestSuite) TestFindPendingForUser() {
	result, err := s.handler.Handle(s.ctx, query.FindMyPendingTasksQuery{
		UserID: "user-a",
		Page:   1,
		Size:   10,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(1), result.Total, "Should find 1 pending task for user-a")
	s.Assert().Equal("Task Instance", result.Items[0].InstanceTitle, "Should have correct instance title")
}

func (s *FindMyPendingTasksTestSuite) TestNoResults() {
	result, err := s.handler.Handle(s.ctx, query.FindMyPendingTasksQuery{
		UserID: "non-existent-user",
		Page:   1,
		Size:   10,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(0), result.Total, "Should find 0 pending tasks")
	s.Assert().Empty(result.Items, "Should return empty slice")
}

func (s *FindMyPendingTasksTestSuite) TestFilters() {
	const travel, purchase = "Travel reimbursement", "Purchase request"

	// titles runs the query for user-f and returns the matched instance titles,
	// checking that the count agrees with the rows the join returned.
	titles := func(q query.FindMyPendingTasksQuery) []string {
		q.UserID, q.Page, q.Size = "user-f", 1, 10

		result, err := s.handler.Handle(s.ctx, q)
		s.Require().NoError(err, "Should query without error")
		s.Require().Equal(int64(len(result.Items)), result.Total, "Total should count exactly the filtered rows")

		matched := make([]string, len(result.Items))
		for i, item := range result.Items {
			matched[i] = item.InstanceTitle
		}

		return matched
	}

	s.Run("NoFilter", func() {
		s.ElementsMatch([]string{travel, purchase}, titles(query.FindMyPendingTasksQuery{}),
			"Joining the instance should keep every pending task")
	})

	s.Run("Keyword", func() {
		s.Equal([]string{travel}, titles(query.FindMyPendingTasksQuery{Keyword: new("reimburse")}),
			"Keyword should match the instance title by substring")
	})

	s.Run("ApplicantID", func() {
		s.Equal([]string{purchase}, titles(query.FindMyPendingTasksQuery{ApplicantID: new("user-q")}),
			"ApplicantID should match the applicant exactly")
	})

	s.Run("ApplicantName", func() {
		s.Equal([]string{travel}, titles(query.FindMyPendingTasksQuery{ApplicantName: new("Alice")}),
			"ApplicantName should match the applicant name by substring")
	})

	s.Run("FlowID", func() {
		s.Equal([]string{purchase}, titles(query.FindMyPendingTasksQuery{FlowID: new(s.purchaseFlowID)}),
			"FlowID should match the instance's flow")
	})

	s.Run("IsTimeout", func() {
		s.Equal([]string{purchase}, titles(query.FindMyPendingTasksQuery{IsTimeout: new(true)}),
			"IsTimeout true should keep only timed-out tasks")
		s.Equal([]string{travel}, titles(query.FindMyPendingTasksQuery{IsTimeout: new(false)}),
			"IsTimeout false should keep only tasks within their deadline")
	})

	s.Run("CreatedAtRange", func() {
		s.Equal([]string{purchase}, titles(query.FindMyPendingTasksQuery{CreatedAtFrom: new(septemberAt(5, 0))}),
			"CreatedAtFrom should drop tasks that arrived earlier")
		s.Equal([]string{travel}, titles(query.FindMyPendingTasksQuery{CreatedAtTo: new(septemberAt(5, 0))}),
			"CreatedAtTo should drop tasks that arrived later")
	})

	s.Run("CreatedAtBoundsAreInclusive", func() {
		arrivedAt := septemberAt(10, 10)

		s.Equal([]string{purchase}, titles(query.FindMyPendingTasksQuery{CreatedAtFrom: &arrivedAt, CreatedAtTo: &arrivedAt}),
			"A task arriving exactly on both bounds should be kept")
	})

	s.Run("FiltersCombineWithAnd", func() {
		s.Empty(titles(query.FindMyPendingTasksQuery{Keyword: new("reimburse"), ApplicantID: new("user-q")}),
			"Filters matching different tasks should together match none")
	})
}

func (s *FindMyPendingTasksTestSuite) TestExcludesNonPendingTasks() {
	result, err := s.handler.Handle(s.ctx, query.FindMyPendingTasksQuery{
		UserID: "user-a",
		Page:   1,
		Size:   10,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(1), result.Total, "Should only count pending tasks, not approved ones")
}
