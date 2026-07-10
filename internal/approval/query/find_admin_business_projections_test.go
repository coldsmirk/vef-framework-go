package query_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/page"
	"github.com/coldsmirk/vef-framework-go/timex"
)

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &FindAdminBusinessProjectionsTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

type FindAdminBusinessProjectionsTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.FindAdminBusinessProjectionsHandler
}

func (s *FindAdminBusinessProjectionsTestSuite) SetupSuite() {
	s.handler = query.NewFindAdminBusinessProjectionsHandler(s.db)

	instanceIDColumn := "apv_instance_id"
	now := timex.Now()

	projections := []approval.BusinessProjection{
		{
			TenantID: "t1", FlowID: "flow-1", FlowVersionID: "version-1", OwnerInstanceID: "instance-1",
			TargetHash: "admin-projection-1", Consistency: config.ApprovalBindingEventual,
			Binding: &approval.BusinessBindingConfig{
				TableName: "biz_order", KeyColumns: []string{"id"},
				StatusColumn: "approval_status", InstanceIDColumn: &instanceIDColumn,
			},
			RecordKey:     []byte(`[{"column":"id","kind":"string","value":"order-1"}]`),
			DesiredStatus: approval.InstanceRunning, DesiredStartedAt: now,
			DesiredRevision: 1, Status: approval.BindingProjectionPending,
		},
		{
			TenantID: "t1", FlowID: "flow-1", FlowVersionID: "version-1", OwnerInstanceID: "instance-2",
			TargetHash: "admin-projection-2", Consistency: config.ApprovalBindingEventual,
			Binding: &approval.BusinessBindingConfig{
				TableName: "biz_order", KeyColumns: []string{"id"},
				StatusColumn: "approval_status", InstanceIDColumn: &instanceIDColumn,
			},
			RecordKey:     []byte(`[{"column":"id","kind":"string","value":"order-2"}]`),
			DesiredStatus: approval.InstanceApproved, DesiredStartedAt: now, DesiredFinishedAt: &now,
			DesiredRevision: 2, AppliedRevision: 1, Status: approval.BindingProjectionFailed,
		},
		{
			TenantID: "t2", FlowID: "flow-2", FlowVersionID: "version-2", OwnerInstanceID: "instance-3",
			TargetHash: "admin-projection-3", Consistency: config.ApprovalBindingSynchronous,
			Binding: &approval.BusinessBindingConfig{
				TableName: "biz_invoice", KeyColumns: []string{"id"},
				StatusColumn: "approval_status", InstanceIDColumn: &instanceIDColumn,
			},
			RecordKey:     []byte(`[{"column":"id","kind":"string","value":"invoice-1"}]`),
			DesiredStatus: approval.InstanceApproved, DesiredStartedAt: now, DesiredFinishedAt: &now,
			DesiredRevision: 1, AppliedRevision: 1, Status: approval.BindingProjectionApplied,
		},
	}
	for i := range projections {
		_, err := s.db.NewInsert().Model(&projections[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert admin projection fixture")
	}
}

func (s *FindAdminBusinessProjectionsTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *FindAdminBusinessProjectionsTestSuite) TestFindAll() {
	result, err := s.handler.Handle(s.ctx, query.FindAdminBusinessProjectionsQuery{
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query business projections")
	s.Assert().Equal(int64(3), result.Total, "Cross-tenant admin query should return every projection")
	s.Assert().Len(result.Items, 3, "Cross-tenant admin query should project every row")

	for _, item := range result.Items {
		s.Assert().NotEmpty(item.BusinessTable, "Admin projection should expose its target table")
		s.Assert().NotEmpty(item.RecordKey, "Admin projection should expose its durable record key")
	}
}

func (s *FindAdminBusinessProjectionsTestSuite) TestFilterByTenantAndStatus() {
	status := approval.BindingProjectionFailed
	result, err := s.handler.Handle(s.ctx, query.FindAdminBusinessProjectionsQuery{
		TenantID: new("t1"),
		Status:   &status,
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should filter business projections by tenant and status")
	s.Require().Len(result.Items, 1, "Only one t1 projection should be failed")
	s.Assert().Equal(int64(1), result.Total, "Filtered projection total should match")
	s.Assert().Equal("instance-2", result.Items[0].OwnerInstanceID,
		"Filtered projection should be the failed t1 target")
	s.Assert().Equal(approval.BindingProjectionFailed, result.Items[0].Status,
		"Filtered projection should retain its convergence status")
	s.Assert().False(result.Items[0].DesiredStartedAt.IsZero(),
		"Admin projection should expose the desired start time")
	s.Assert().NotNil(result.Items[0].DesiredFinishedAt,
		"Admin projection should expose the desired finish time for final state")
}

func (s *FindAdminBusinessProjectionsTestSuite) TestPagination() {
	result, err := s.handler.Handle(s.ctx, query.FindAdminBusinessProjectionsQuery{
		Pageable: page.Pageable{Page: 1, Size: 2},
	})
	s.Require().NoError(err, "Should paginate business projections")
	s.Assert().Equal(int64(3), result.Total, "Pagination should retain the full projection total")
	s.Assert().Len(result.Items, 2, "Page size should limit returned projections")
}
