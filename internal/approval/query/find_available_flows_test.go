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

type FindAvailableFlowsMockAssigneeService struct {
	roleUsers map[string][]string
}

func (*FindAvailableFlowsMockAssigneeService) GetSuperior(context.Context, string) (*approval.UserInfo, error) {
	return nil, nil
}

func (*FindAvailableFlowsMockAssigneeService) GetDeptLeaders(context.Context, string) ([]approval.UserInfo, error) {
	return nil, nil
}

func (m *FindAvailableFlowsMockAssigneeService) GetRoleUsers(_ context.Context, roleID string) ([]approval.UserInfo, error) {
	users := m.roleUsers[roleID]

	result := make([]approval.UserInfo, len(users))
	for i, u := range users {
		result[i] = approval.UserInfo{ID: u, Name: ""}
	}

	return result, nil
}

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &FindAvailableFlowsTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// FindAvailableFlowsTestSuite tests the FindAvailableFlowsHandler.
type FindAvailableFlowsTestSuite struct {
	suite.Suite

	ctx     context.Context
	db      orm.DB
	handler *query.FindAvailableFlowsHandler

	allAllowedFlowID  string
	restrictedFlowID  string
	deptFlowID        string
	roleFlowID        string
	inactiveFlowID    string
	unpublishedFlowID string
}

func (s *FindAvailableFlowsTestSuite) SetupSuite() {
	s.handler = query.NewFindAvailableFlowsHandler(s.db, &FindAvailableFlowsMockAssigneeService{
		roleUsers: map[string][]string{
			"role-reviewer": {"user-role"},
		},
	})

	// Create category.
	category := &approval.FlowCategory{
		TenantID: "default",
		Code:     "maf-cat",
		Name:     "Available Category",
	}
	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "Should insert category")

	// Flow 1: all initiation allowed, active.
	flow1 := &approval.Flow{
		TenantID:               "default",
		CategoryID:             category.ID,
		Code:                   "maf-all",
		Name:                   "All Allowed Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Test",
		IsActive:               true,
	}
	_, err = s.db.NewInsert().Model(flow1).Exec(s.ctx)
	s.Require().NoError(err, "Should insert flow1")
	s.allAllowedFlowID = flow1.ID
	s.insertPublishedVersion(flow1.ID, 1)

	// Flow 2: restricted initiation, active.
	flow2 := &approval.Flow{
		TenantID:               "default",
		CategoryID:             category.ID,
		Code:                   "maf-restricted",
		Name:                   "Restricted Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: false,
		InstanceTitleTemplate:  "Test",
		IsActive:               true,
	}
	_, err = s.db.NewInsert().Model(flow2).Exec(s.ctx)
	s.Require().NoError(err, "Should insert flow2")
	s.restrictedFlowID = flow2.ID
	s.insertPublishedVersion(flow2.ID, 1)

	// Add initiator rule for flow2: user-a is allowed.
	initiator := &approval.FlowInitiator{
		FlowID: flow2.ID,
		Kind:   approval.InitiatorUser,
		IDs:    []string{"user-a"},
	}
	_, err = s.db.NewInsert().Model(initiator).Exec(s.ctx)
	s.Require().NoError(err, "Should insert initiator")

	// Flow 3: inactive flow.
	flow3 := &approval.Flow{
		TenantID:               "default",
		CategoryID:             category.ID,
		Code:                   "maf-inactive",
		Name:                   "Inactive Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Test",
		IsActive:               false,
	}
	_, err = s.db.NewInsert().Model(flow3).Exec(s.ctx)
	s.Require().NoError(err, "Should insert flow3")
	s.inactiveFlowID = flow3.ID
	s.insertPublishedVersion(flow3.ID, 1)

	flow4 := &approval.Flow{
		TenantID:               "default",
		CategoryID:             category.ID,
		Code:                   "maf-dept",
		Name:                   "Department Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: false,
		InstanceTitleTemplate:  "Test",
		IsActive:               true,
	}
	_, err = s.db.NewInsert().Model(flow4).Exec(s.ctx)
	s.Require().NoError(err, "Should insert department-restricted flow")
	s.deptFlowID = flow4.ID
	s.insertPublishedVersion(flow4.ID, 1)
	_, err = s.db.NewInsert().Model(&approval.FlowInitiator{
		FlowID: flow4.ID,
		Kind:   approval.InitiatorDept,
		IDs:    []string{"dept-a"},
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should insert department initiator")

	flow5 := &approval.Flow{
		TenantID:               "default",
		CategoryID:             category.ID,
		Code:                   "maf-role",
		Name:                   "Role Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: false,
		InstanceTitleTemplate:  "Test",
		IsActive:               true,
	}
	_, err = s.db.NewInsert().Model(flow5).Exec(s.ctx)
	s.Require().NoError(err, "Should insert role-restricted flow")
	s.roleFlowID = flow5.ID
	s.insertPublishedVersion(flow5.ID, 1)
	_, err = s.db.NewInsert().Model(&approval.FlowInitiator{
		FlowID: flow5.ID,
		Kind:   approval.InitiatorRole,
		IDs:    []string{"role-reviewer"},
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should insert role initiator")

	flow6 := &approval.Flow{
		TenantID:               "default",
		CategoryID:             category.ID,
		Code:                   "maf-unpublished",
		Name:                   "Unpublished Flow",
		BindingMode:            approval.BindingStandalone,
		IsAllInitiationAllowed: true,
		InstanceTitleTemplate:  "Test",
		IsActive:               true,
	}
	_, err = s.db.NewInsert().Model(flow6).Exec(s.ctx)
	s.Require().NoError(err, "Should insert unpublished flow")
	s.unpublishedFlowID = flow6.ID
}

func (s *FindAvailableFlowsTestSuite) insertPublishedVersion(flowID string, version int) {
	s.T().Helper()
	_, err := s.db.NewInsert().Model(&approval.FlowVersion{
		FlowID:  flowID,
		Version: version,
		Status:  approval.VersionPublished,
	}).Exec(s.ctx)
	s.Require().NoError(err, "Should insert published version")
}

func (s *FindAvailableFlowsTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *FindAvailableFlowsTestSuite) TestAllAllowedFlows() {
	result, err := s.handler.Handle(s.ctx, query.FindAvailableFlowsQuery{
		UserID:   "user-z",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	// user-z should only see flow1 (all allowed) since flow2 is restricted and flow3 is inactive.
	s.Assert().GreaterOrEqual(result.Total, int64(1), "Should find at least the all-allowed flow")

	hasAllAllowed := false
	for _, item := range result.Items {
		if item.FlowID == s.allAllowedFlowID {
			hasAllAllowed = true

			s.Assert().Equal("All Allowed Flow", item.FlowName, "Should have correct name")
			s.Assert().Equal("Available Category", item.CategoryName, "Should have correct category")
		}
	}

	s.Assert().True(hasAllAllowed, "Should include the all-allowed flow")
}

func (s *FindAvailableFlowsTestSuite) TestUserWithInitiatorAccess() {
	result, err := s.handler.Handle(s.ctx, query.FindAvailableFlowsQuery{
		UserID:   "user-a",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")

	foundRestricted := false
	for _, item := range result.Items {
		if item.FlowID == s.restrictedFlowID {
			foundRestricted = true
		}
	}

	s.Assert().True(foundRestricted, "user-a should see the restricted flow via initiator rule")
}

func (s *FindAvailableFlowsTestSuite) TestUserWithDeptInitiatorAccess() {
	result, err := s.handler.Handle(s.ctx, query.FindAvailableFlowsQuery{
		UserID:          "user-dept",
		ApplicantDeptID: new("dept-a"),
		Pageable:        page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")

	foundDeptFlow := false
	for _, item := range result.Items {
		if item.FlowID == s.deptFlowID {
			foundDeptFlow = true
		}
	}

	s.Assert().True(foundDeptFlow, "User in matching department should see department-restricted flow")
}

func (s *FindAvailableFlowsTestSuite) TestUserWithRoleInitiatorAccess() {
	result, err := s.handler.Handle(s.ctx, query.FindAvailableFlowsQuery{
		UserID:   "user-role",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")

	foundRoleFlow := false
	for _, item := range result.Items {
		if item.FlowID == s.roleFlowID {
			foundRoleFlow = true
		}
	}

	s.Assert().True(foundRoleFlow, "User in matching role should see role-restricted flow")
}

func (s *FindAvailableFlowsTestSuite) TestExcludesInactiveFlows() {
	result, err := s.handler.Handle(s.ctx, query.FindAvailableFlowsQuery{
		UserID:   "user-z",
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")

	for _, item := range result.Items {
		s.Assert().NotEqual(s.inactiveFlowID, item.FlowID, "Should not include inactive flow")
	}
}

func (s *FindAvailableFlowsTestSuite) TestExcludesFlowsWithoutPublishedVersion() {
	result, err := s.handler.Handle(s.ctx, query.FindAvailableFlowsQuery{
		UserID:   "user-z",
		Pageable: page.Pageable{Page: 1, Size: 20},
	})
	s.Require().NoError(err, "Should query without error")

	for _, item := range result.Items {
		s.Assert().NotEqual(s.unpublishedFlowID, item.FlowID, "Should not include active flow without published version")
	}
}

func (s *FindAvailableFlowsTestSuite) TestFilterByKeyword() {
	result, err := s.handler.Handle(s.ctx, query.FindAvailableFlowsQuery{
		UserID:   "user-z",
		Keyword:  new("All Allowed"),
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().GreaterOrEqual(result.Total, int64(1), "Should find at least 1 flow matching keyword")
}

func (s *FindAvailableFlowsTestSuite) TestNoResults() {
	result, err := s.handler.Handle(s.ctx, query.FindAvailableFlowsQuery{
		UserID:   "user-z",
		Keyword:  new("NonExistentFlow"),
		Pageable: page.Pageable{Page: 1, Size: 10},
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(0), result.Total, "Should find 0 flows")
	s.Assert().Empty(result.Items, "Should return empty slice")
}
