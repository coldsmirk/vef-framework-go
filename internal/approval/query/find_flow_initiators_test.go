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
		return &FindFlowInitiatorsTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// FindFlowInitiatorsTestSuite tests the FindFlowInitiatorsHandler.
type FindFlowInitiatorsTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.FindFlowInitiatorsHandler

	flowID1 string
	flowID2 string
}

func (s *FindFlowInitiatorsTestSuite) SetupSuite() {
	s.handler = query.NewFindFlowInitiatorsHandler(s.db)

	category := approval.FlowCategory{TenantID: "t1", Code: "cat-init", Name: "Initiator Category", IsActive: true}
	_, err := s.db.NewInsert().Model(&category).Exec(s.ctx)
	s.Require().NoError(err, "Should insert category")

	flow1 := approval.Flow{TenantID: "t1", CategoryID: category.ID, Code: "flow-init-1", Name: "Flow 1", IsActive: true}
	flow2 := approval.Flow{TenantID: "t1", CategoryID: category.ID, Code: "flow-init-2", Name: "Flow 2", IsActive: true}
	_, err = s.db.NewInsert().Model(&flow1).Exec(s.ctx)
	s.Require().NoError(err, "Should insert flow 1")
	_, err = s.db.NewInsert().Model(&flow2).Exec(s.ctx)
	s.Require().NoError(err, "Should insert flow 2")
	s.flowID1 = flow1.ID
	s.flowID2 = flow2.ID

	initiators := []approval.FlowInitiator{
		{FlowID: s.flowID1, Kind: approval.InitiatorUser, IDs: []string{"u1", "u2"}},
		{FlowID: s.flowID1, Kind: approval.InitiatorRole, IDs: []string{"r1"}},
		{FlowID: s.flowID2, Kind: approval.InitiatorDepartment, IDs: []string{"d1"}},
	}
	for i := range initiators {
		_, err := s.db.NewInsert().Model(&initiators[i]).Exec(s.ctx)
		s.Require().NoError(err, "Should insert test initiator")
	}
}

func (s *FindFlowInitiatorsTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *FindFlowInitiatorsTestSuite) TestSuccessWithMultipleInitiators() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowInitiatorsQuery{
		FlowID: s.flowID1,
		Caller: approval.SystemCaller,
	})
	s.Require().NoError(err, "Should query without error")
	s.Require().Len(result, 2, "Should find 2 initiators for flow 1")

	kinds := map[approval.InitiatorKind][]string{}
	for _, initiator := range result {
		kinds[initiator.Kind] = initiator.IDs
	}

	s.Assert().Equal([]string{"u1", "u2"}, kinds[approval.InitiatorUser], "User initiator ids should round-trip")
	s.Assert().Equal([]string{"r1"}, kinds[approval.InitiatorRole], "Role initiator ids should round-trip")
}

func (s *FindFlowInitiatorsTestSuite) TestEmpty() {
	result, err := s.handler.Handle(s.ctx, query.FindFlowInitiatorsQuery{
		FlowID: "non-existent-flow-id",
		Caller: approval.SystemCaller,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Empty(result, "Should return empty slice for non-existent flow")
}

func (s *FindFlowInitiatorsTestSuite) TestForeignTenantReturnsEmptyNonNil() {
	// Fixture flows belong to t1. A non-super-admin caller in t2 must get an
	// empty (non-nil) slice — opaque, indistinguishable from "no such flow".
	result, err := s.handler.Handle(s.ctx, query.FindFlowInitiatorsQuery{
		FlowID: s.flowID1,
		Caller: approval.CallerContext{TenantID: "t2"},
	})
	s.Require().NoError(err, "Cross-tenant access must be an opaque empty result, not an error")
	s.Require().NotNil(result, "Result must be a non-nil empty slice for response-shape uniformity")
	s.Assert().Empty(result, "A foreign-tenant caller must see no initiators")
}

func (s *FindFlowInitiatorsTestSuite) TestMismatchedQueryTenantReturnsEmpty() {
	// Even an authorized caller (super-admin) gets empty when the explicit
	// query.TenantID does not match the flow's tenant.
	mismatch := "t2"
	result, err := s.handler.Handle(s.ctx, query.FindFlowInitiatorsQuery{
		FlowID:   s.flowID1,
		TenantID: &mismatch,
		Caller:   approval.SystemCaller,
	})
	s.Require().NoError(err, "Mismatched tenant filter must not error")
	s.Require().NotNil(result, "Result must be a non-nil empty slice")
	s.Assert().Empty(result, "A query.TenantID that does not match the flow's tenant returns no initiators")
}
