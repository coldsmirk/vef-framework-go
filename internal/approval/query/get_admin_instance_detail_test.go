package query_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/approval/shared"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &GetAdminInstanceDetailTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// GetAdminInstanceDetailTestSuite tests the GetAdminInstanceDetailHandler.
type GetAdminInstanceDetailTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.GetAdminInstanceDetailHandler

	instanceID string
}

func (s *GetAdminInstanceDetailTestSuite) SetupSuite() {
	s.handler = query.NewGetAdminInstanceDetailHandler(s.db)

	fix := setupQueryFixture(s.T(), s.ctx, s.db, "adid", 0)

	// Create flow nodes.
	nodes := []approval.FlowNode{
		{FlowVersionID: fix.VersionID, Key: "start-1", Kind: approval.NodeStart, Name: "Start"},
		{FlowVersionID: fix.VersionID, Key: "approval-1", Kind: approval.NodeApproval, Name: "Approval"},
		{FlowVersionID: fix.VersionID, Key: "end-1", Kind: approval.NodeEnd, Name: "End"},
	}
	for i := range nodes {
		_, err := s.db.NewInsert().Model(&nodes[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert node")
	}

	// Create instance.
	instance := &approval.Instance{
		TenantID:      "default",
		FlowID:        fix.FlowID,
		FlowVersionID: fix.VersionID,
		Title:         "Admin Detail Test",
		InstanceNo:    "ADID-001",
		ApplicantID:   "user-1",
		Status:        approval.InstanceRunning,
	}
	_, err := s.db.NewInsert().Model(instance).Exec(s.ctx)
	s.Require().NoError(err, "Should insert instance")
	s.instanceID = instance.ID

	// Create tasks.
	tasks := []approval.Task{
		{TenantID: "default", InstanceID: instance.ID, NodeID: nodes[1].ID, AssigneeID: "user-2", SortOrder: 1, Status: approval.TaskPending},
	}
	for i := range tasks {
		_, err := s.db.NewInsert().Model(&tasks[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert task")
	}

	// Create action logs.
	logs := []approval.ActionLog{
		{InstanceID: instance.ID, Action: approval.ActionSubmit, OperatorID: "user-1", OperatorName: "Applicant"},
	}
	for i := range logs {
		_, err := s.db.NewInsert().Model(&logs[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert action log")
	}
}

func (s *GetAdminInstanceDetailTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *GetAdminInstanceDetailTestSuite) TestGetDetailSuccess() {
	detail, err := s.handler.Handle(s.ctx, query.GetAdminInstanceDetailQuery{
		InstanceID: s.instanceID,
	})
	s.Require().NoError(err, "Should get admin instance detail without error")
	s.Require().NotNil(detail, "Should not be nil")

	s.Assert().Equal(s.instanceID, detail.Instance.InstanceID, "Should return correct instance")
	s.Assert().Equal("Admin Detail Test", detail.Instance.Title, "Should return correct title")
	s.Assert().Equal("default", detail.Instance.TenantID, "Should include tenant ID")
	s.Assert().Len(detail.Tasks, 1, "Should return 1 task")
	s.Assert().Len(detail.ActionLogs, 1, "Should return 1 action log")
	s.Assert().Len(detail.FlowNodes, 3, "Should return 3 flow nodes")
}

func (s *GetAdminInstanceDetailTestSuite) TestNotFound() {
	_, err := s.handler.Handle(s.ctx, query.GetAdminInstanceDetailQuery{
		InstanceID: "non-existent-instance",
	})
	s.Require().ErrorIs(err, shared.ErrInstanceNotFound, "Should return instance-not-found error")
}
