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
		return &FindAdminActionLogsTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// FindAdminActionLogsTestSuite tests the FindAdminActionLogsHandler.
type FindAdminActionLogsTestSuite struct {
	suite.Suite

	ctx        context.Context
	db         orm.DB
	handler    *query.FindAdminActionLogsHandler
	instanceID string
}

func (s *FindAdminActionLogsTestSuite) SetupSuite() {
	s.handler = query.NewFindAdminActionLogsHandler(s.db)

	fix := setupQueryFixture(s.T(), s.ctx, s.db, "adal", 0)

	inst := &approval.Instance{
		TenantID: "default", FlowID: fix.FlowID, FlowVersionID: fix.VersionID,
		Title: "Action Log Test", InstanceNo: "ADAL-001", ApplicantID: "user-1", Status: approval.InstanceRunning,
	}
	_, err := s.db.NewInsert().Model(inst).Exec(s.ctx)
	s.Require().NoError(err, "Should insert instance")
	s.instanceID = inst.ID

	logs := []approval.ActionLog{
		{InstanceID: inst.ID, Action: approval.ActionSubmit, OperatorID: "user-1", OperatorName: "Alice"},
		{InstanceID: inst.ID, Action: approval.ActionApprove, OperatorID: "user-2", OperatorName: "Bob"},
		{InstanceID: inst.ID, Action: approval.ActionReject, OperatorID: "user-3", OperatorName: "Charlie"},
	}
	for i := range logs {
		_, err := s.db.NewInsert().Model(&logs[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert action log")
	}
}

func (s *FindAdminActionLogsTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *FindAdminActionLogsTestSuite) TestSuccessWithPagination() {
	result, err := s.handler.Handle(s.ctx, query.FindAdminActionLogsQuery{
		InstanceID: s.instanceID,
		Pageable:   page.Pageable{Page: 1, Size: 2},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(3), result.Total, "Total should be 3")
	s.Assert().Len(result.Items, 2, "Page 1 with size 2 should return 2 items")
	s.Assert().Equal("Alice", result.Items[0].OperatorName, "First log should be Alice (ordered by created_at ASC)")
}

func (s *FindAdminActionLogsTestSuite) TestEmpty() {
	result, err := s.handler.Handle(s.ctx, query.FindAdminActionLogsQuery{
		InstanceID: "non-existent-instance",
		Pageable:   page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(0), result.Total, "Should find 0 logs")
	s.Assert().Empty(result.Items, "Should return empty slice")
}
