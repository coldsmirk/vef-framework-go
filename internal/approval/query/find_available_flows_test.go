package query_test

import (
	"context"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/internal/approval/query"
	"github.com/coldsmirk/vef-framework-go/internal/approval/service"
	"github.com/coldsmirk/vef-framework-go/internal/approval/strategy"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
)

type FindAvailableFlowsMockAssigneeService struct {
	roleUsers map[string][]string
}

func (*FindAvailableFlowsMockAssigneeService) GetSuperior(context.Context, string) (*approval.UserInfo, error) {
	return nil, nil
}

func (*FindAvailableFlowsMockAssigneeService) GetDepartmentLeaders(context.Context, string) ([]approval.UserInfo, error) {
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

// availableHostInitiator is a host initiator kind admitting one user, and only
// for the default tenant, so the list proves it runs the same gate a start
// does — host kinds and the starting tenant included.
type availableHostInitiator struct{}

func (availableHostInitiator) Describe() approval.KindDescriptor[approval.InitiatorKind] {
	return approval.KindDescriptor[approval.InitiatorKind]{
		Kind:      "maf_host",
		Label:     "Host rule",
		Selection: approval.SelectionNone,
	}
}

func (availableHostInitiator) Permits(_ context.Context, rc *approval.InitiatorResolveContext) (bool, error) {
	return rc.Applicant.ID == "user-host" && rc.TenantID == approval.DefaultTenantID, nil
}

func init() {
	registry.Add(func(env *testx.DBEnv) suite.TestingSuite {
		return &FindAvailableFlowsTestSuite{ctx: env.Ctx, db: env.DB}
	})
}

// defaultTenantCaller is an ordinary caller of the default tenant, the shape
// every self-service request carries.
var defaultTenantCaller = approval.CallerContext{TenantID: approval.DefaultTenantID}

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
	hostRuleFlowID    string
	inactiveFlowID    string
	unpublishedFlowID string

	sharedFlowID       string
	shadowedFlowID     string
	otherTenantFlowID  string
	sharedCategoryName string
}

func (s *FindAvailableFlowsTestSuite) SetupSuite() {
	composite, err := strategy.NewCompositeInitiatorResolver(
		strategy.BuiltinInitiatorResolvers(&FindAvailableFlowsMockAssigneeService{
			roleUsers: map[string][]string{
				"role-reviewer": {"user-role"},
			},
		}),
		[]approval.InitiatorResolver{availableHostInitiator{}},
	)
	s.Require().NoError(err, "Should register the initiator kinds")

	s.handler = query.NewFindAvailableFlowsHandler(s.db, service.NewValidationService(composite))

	category := s.insertCategory(approval.DefaultTenantID, "maf-cat", "Available Category")

	// Flow 1: all initiation allowed, active.
	s.allAllowedFlowID = s.insertFlow(&approval.Flow{
		TenantID:               approval.DefaultTenantID,
		CategoryID:             category,
		Code:                   "maf-all",
		Name:                   "All Allowed Flow",
		Labels:                 map[string]string{"mobile": "true"},
		IsAllInitiationAllowed: true,
		IsActive:               true,
	}, true)

	// Flow 2: restricted initiation, active; user-a is allowed.
	s.restrictedFlowID = s.insertFlow(&approval.Flow{
		TenantID:   approval.DefaultTenantID,
		CategoryID: category,
		Code:       "maf-restricted",
		Name:       "Restricted Flow",
		Labels:     map[string]string{"app": "crm"},
		IsActive:   true,
	}, true)
	s.insertInitiator(s.restrictedFlowID, approval.InitiatorUser, "user-a")

	// Flow 3: inactive flow.
	s.inactiveFlowID = s.insertFlow(&approval.Flow{
		TenantID:               approval.DefaultTenantID,
		CategoryID:             category,
		Code:                   "maf-inactive",
		Name:                   "Inactive Flow",
		IsAllInitiationAllowed: true,
	}, true)

	s.deptFlowID = s.insertFlow(&approval.Flow{
		TenantID:   approval.DefaultTenantID,
		CategoryID: category,
		Code:       "maf-dept",
		Name:       "Department Flow",
		IsActive:   true,
	}, true)
	s.insertInitiator(s.deptFlowID, approval.InitiatorDepartment, "dept-a")

	s.roleFlowID = s.insertFlow(&approval.Flow{
		TenantID:   approval.DefaultTenantID,
		CategoryID: category,
		Code:       "maf-role",
		Name:       "Role Flow",
		IsActive:   true,
	}, true)
	s.insertInitiator(s.roleFlowID, approval.InitiatorRole, "role-reviewer")

	s.hostRuleFlowID = s.insertFlow(&approval.Flow{
		TenantID:   approval.DefaultTenantID,
		CategoryID: category,
		Code:       "maf-host",
		Name:       "Host Rule Flow",
		IsActive:   true,
	}, true)
	s.insertInitiator(s.hostRuleFlowID, "maf_host")

	s.unpublishedFlowID = s.insertFlow(&approval.Flow{
		TenantID:               approval.DefaultTenantID,
		CategoryID:             category,
		Code:                   "maf-unpublished",
		Name:                   "Unpublished Flow",
		IsAllInitiationAllowed: true,
		IsActive:               true,
	}, false)

	// Shared flows, under a shared category: one every tenant is offered, and
	// one whose code the default tenant has taken for an inactive flow of its
	// own — which therefore hides the shared one there, and only there.
	s.sharedCategoryName = "Shared Category"
	sharedCategory := s.insertCategory(approval.SharedTenantID, "maf-shared-cat", s.sharedCategoryName)

	s.sharedFlowID = s.insertFlow(&approval.Flow{
		TenantID:               approval.SharedTenantID,
		CategoryID:             sharedCategory,
		Code:                   "maf-shared",
		Name:                   "Shared Flow",
		IsAllInitiationAllowed: true,
		IsActive:               true,
	}, true)

	s.shadowedFlowID = s.insertFlow(&approval.Flow{
		TenantID:               approval.SharedTenantID,
		CategoryID:             sharedCategory,
		Code:                   "maf-shadowed",
		Name:                   "Shadowed Shared Flow",
		IsAllInitiationAllowed: true,
		IsActive:               true,
	}, true)
	s.insertFlow(&approval.Flow{
		TenantID:               approval.DefaultTenantID,
		CategoryID:             category,
		Code:                   "maf-shadowed",
		Name:                   "Tenant Replacement",
		IsAllInitiationAllowed: true,
	}, true)

	// Another tenant's open flow, never offered to the default tenant.
	otherCategory := s.insertCategory("tenant-b", "maf-other-cat", "Other Category")
	s.otherTenantFlowID = s.insertFlow(&approval.Flow{
		TenantID:               "tenant-b",
		CategoryID:             otherCategory,
		Code:                   "maf-other",
		Name:                   "Other Tenant Flow",
		IsAllInitiationAllowed: true,
		IsActive:               true,
	}, true)
}

func (s *FindAvailableFlowsTestSuite) insertCategory(tenantID, code, name string) string {
	s.T().Helper()

	category := &approval.FlowCategory{TenantID: tenantID, Code: code, Name: name}
	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "Should insert category %s", code)

	return category.ID
}

// insertFlow inserts the flow — with its published version when published —
// and returns its ID.
func (s *FindAvailableFlowsTestSuite) insertFlow(flow *approval.Flow, published bool) string {
	s.T().Helper()

	flow.BindingMode = approval.BindingStandalone
	flow.InstanceTitleTemplate = "Test"

	_, err := s.db.NewInsert().Model(flow).Exec(s.ctx)
	s.Require().NoError(err, "Should insert flow %s", flow.Code)

	if published {
		_, err = s.db.NewInsert().Model(&approval.FlowVersion{
			FlowID:  flow.ID,
			Version: 1,
			Status:  approval.VersionPublished,
		}).Exec(s.ctx)
		s.Require().NoError(err, "Should insert published version of %s", flow.Code)
	}

	return flow.ID
}

func (s *FindAvailableFlowsTestSuite) insertInitiator(flowID string, kind approval.InitiatorKind, ids ...string) {
	s.T().Helper()

	_, err := s.db.NewInsert().Model(&approval.FlowInitiator{FlowID: flowID, Kind: kind, IDs: ids}).Exec(s.ctx)
	s.Require().NoError(err, "Should insert initiator")
}

// listFor returns the IDs of the flows offered by the query, which pages wide
// enough to hold every fixture.
func (s *FindAvailableFlowsTestSuite) listFor(q query.FindAvailableFlowsQuery) []string {
	s.T().Helper()

	q.Page = 1
	q.Size = 50

	result, err := s.handler.Handle(s.ctx, q)
	s.Require().NoError(err, "Should query without error")

	ids := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		ids = append(ids, item.FlowID)
	}

	return ids
}

func (s *FindAvailableFlowsTestSuite) TearDownSuite() {
	cleanAllQueryData(s.ctx, s.db)
}

func (s *FindAvailableFlowsTestSuite) TestAllAllowedFlows() {
	result, err := s.handler.Handle(s.ctx, query.FindAvailableFlowsQuery{
		UserID: "user-z",
		Caller: defaultTenantCaller,
		Page:   1,
		Size:   10,
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

func (s *FindAvailableFlowsTestSuite) TestFilterByLabels() {
	s.Run("MatchesLabeledFlow", func() {
		result, err := s.handler.Handle(s.ctx, query.FindAvailableFlowsQuery{
			UserID: "user-z",
			Labels: map[string]string{"mobile": "true"},
			Caller: defaultTenantCaller,
			Page:   1,
			Size:   10,
		})
		s.Require().NoError(err, "Should query without error")
		s.Require().Equal(int64(1), result.Total, "mobile=true should keep only the labeled all-allowed flow")
		s.Assert().Equal(s.allAllowedFlowID, result.Items[0].FlowID, "Should return the mobile-labeled flow")
		s.Assert().Equal(map[string]string{"mobile": "true"}, result.Items[0].Labels,
			"AvailableFlow should carry the flow's labels")
	})

	s.Run("ExcludesUnlabeledFlows", func() {
		result, err := s.handler.Handle(s.ctx, query.FindAvailableFlowsQuery{
			UserID: "user-z",
			Labels: map[string]string{"app": "crm"},
			Caller: defaultTenantCaller,
			Page:   1,
			Size:   10,
		})
		s.Require().NoError(err, "Should query without error")
		s.Assert().Equal(int64(0), result.Total,
			"user-z cannot initiate the crm-labeled flow, and unlabeled flows never match a label filter")
	})

	s.Run("FiltersInitiatorMatchedFlows", func() {
		// user-a is allowed both flow1 (all-allowed) and flow2 (initiator
		// rule); the label filter must also apply to the initiator-matched
		// path, not just the all-allowed branch.
		result, err := s.handler.Handle(s.ctx, query.FindAvailableFlowsQuery{
			UserID: "user-a",
			Labels: map[string]string{"app": "crm"},
			Caller: defaultTenantCaller,
			Page:   1,
			Size:   10,
		})
		s.Require().NoError(err, "Should query without error")
		s.Require().Equal(int64(1), result.Total, "app=crm should keep only the restricted flow")
		s.Assert().Equal(s.restrictedFlowID, result.Items[0].FlowID, "Should return the crm-labeled restricted flow")
	})
}

func (s *FindAvailableFlowsTestSuite) TestUserWithInitiatorAccess() {
	ids := s.listFor(query.FindAvailableFlowsQuery{UserID: "user-a", Caller: defaultTenantCaller})
	s.Assert().Contains(ids, s.restrictedFlowID, "User-a should see the restricted flow via initiator rule")
}

func (s *FindAvailableFlowsTestSuite) TestUserWithDepartmentInitiatorAccess() {
	ids := s.listFor(query.FindAvailableFlowsQuery{
		UserID:                "user-dept",
		ApplicantDepartmentID: new("dept-a"),
		Caller:                defaultTenantCaller,
	})
	s.Assert().Contains(ids, s.deptFlowID, "User in matching department should see department-restricted flow")
}

func (s *FindAvailableFlowsTestSuite) TestUserWithRoleInitiatorAccess() {
	ids := s.listFor(query.FindAvailableFlowsQuery{UserID: "user-role", Caller: defaultTenantCaller})
	s.Assert().Contains(ids, s.roleFlowID, "User in matching role should see role-restricted flow")
}

// TestHostInitiatorKindGatesTheList pins the list to the gate a start applies:
// a flow restricted by a host-registered kind is offered exactly when that
// kind admits the user for the tenant the start would run in.
func (s *FindAvailableFlowsTestSuite) TestHostInitiatorKindGatesTheList() {
	s.Assert().Contains(s.listFor(query.FindAvailableFlowsQuery{UserID: "user-host", Caller: defaultTenantCaller}),
		s.hostRuleFlowID, "A user the host kind admits should be offered the flow")
	s.Assert().NotContains(s.listFor(query.FindAvailableFlowsQuery{UserID: "user-z", Caller: defaultTenantCaller}),
		s.hostRuleFlowID, "A user the host kind does not admit must not be offered the flow")
}

func (s *FindAvailableFlowsTestSuite) TestExcludesInactiveFlows() {
	s.Assert().NotContains(s.listFor(query.FindAvailableFlowsQuery{UserID: "user-z", Caller: defaultTenantCaller}),
		s.inactiveFlowID, "Should not include inactive flow")
}

func (s *FindAvailableFlowsTestSuite) TestExcludesFlowsWithoutPublishedVersion() {
	s.Assert().NotContains(s.listFor(query.FindAvailableFlowsQuery{UserID: "user-z", Caller: defaultTenantCaller}),
		s.unpublishedFlowID, "Should not include active flow without published version")
}

func (s *FindAvailableFlowsTestSuite) TestFilterByKeyword() {
	result, err := s.handler.Handle(s.ctx, query.FindAvailableFlowsQuery{
		UserID:  "user-z",
		Keyword: new("All Allowed"),
		Caller:  defaultTenantCaller,
		Page:    1,
		Size:    10,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().GreaterOrEqual(result.Total, int64(1), "Should find at least 1 flow matching keyword")
}

func (s *FindAvailableFlowsTestSuite) TestNoResults() {
	result, err := s.handler.Handle(s.ctx, query.FindAvailableFlowsQuery{
		UserID:  "user-z",
		Keyword: new("NonExistentFlow"),
		Caller:  defaultTenantCaller,
		Page:    1,
		Size:    10,
	})
	s.Require().NoError(err, "Should query without error")
	s.Assert().Equal(int64(0), result.Total, "Should find 0 flows")
	s.Assert().Empty(result.Items, "Should return empty slice")
}

func (s *FindAvailableFlowsTestSuite) TestTenancy() {
	s.Run("OffersSharedFlowsBesideOwn", func() {
		result, err := s.handler.Handle(s.ctx, query.FindAvailableFlowsQuery{
			UserID: "user-z",
			Caller: defaultTenantCaller,
			Page:   1,
			Size:   50,
		})
		s.Require().NoError(err, "Should query without error")

		var shared *string
		for _, item := range result.Items {
			if item.FlowID == s.sharedFlowID {
				shared = &item.CategoryName
			}
		}

		s.Require().NotNil(shared, "A shared flow should be offered to the tenant")
		s.Assert().Equal(s.sharedCategoryName, *shared, "A shared flow should carry its shared category")
	})

	s.Run("OffersSharedFlowsToEveryTenant", func() {
		ids := s.listFor(query.FindAvailableFlowsQuery{
			UserID: "user-b",
			Caller: approval.CallerContext{TenantID: "tenant-b"},
		})
		s.Assert().ElementsMatch([]string{s.otherTenantFlowID, s.sharedFlowID, s.shadowedFlowID}, ids,
			"Tenant B should be offered its own flow plus every shared flow it has not replaced")
	})

	s.Run("TenantFlowHidesSharedFlowOfSameCode", func() {
		ids := s.listFor(query.FindAvailableFlowsQuery{UserID: "user-z", Caller: defaultTenantCaller})
		s.Assert().NotContains(ids, s.shadowedFlowID,
			"A tenant flow — even an inactive one — replaces the shared flow of its code within the tenant")
	})

	s.Run("HidesOtherTenantsFlows", func() {
		ids := s.listFor(query.FindAvailableFlowsQuery{UserID: "user-z", Caller: defaultTenantCaller})
		s.Assert().NotContains(ids, s.otherTenantFlowID, "Another tenant's flow must never be offered")
	})

	s.Run("PinsOrdinaryCallerToOwnTenant", func() {
		ids := s.listFor(query.FindAvailableFlowsQuery{
			UserID:   "user-z",
			TenantID: new("tenant-b"),
			Caller:   defaultTenantCaller,
		})
		s.Assert().NotContains(ids, s.otherTenantFlowID,
			"An ordinary caller naming another tenant is still offered only its own tenant's flows")
		s.Assert().Contains(ids, s.allAllowedFlowID, "An ordinary caller keeps its own tenant's flows")
	})

	s.Run("SuperAdminListsTheRequestedTenant", func() {
		ids := s.listFor(query.FindAvailableFlowsQuery{
			UserID:   "user-z",
			TenantID: new("tenant-b"),
			Caller:   approval.CallerContext{IsSuperAdmin: true},
		})
		s.Assert().ElementsMatch([]string{s.otherTenantFlowID, s.sharedFlowID, s.shadowedFlowID}, ids,
			"A super-admin naming a tenant is offered what that tenant would be offered")
	})

	s.Run("FailsClosedWithoutTenant", func() {
		_, err := s.handler.Handle(s.ctx, query.FindAvailableFlowsQuery{UserID: "user-z", Page: 1, Size: 10})
		s.Require().ErrorIs(err, approval.ErrCrossTenantAccess, "A caller without a tenant must be denied")
	})

	s.Run("RejectsSharedTenantAsStartTenant", func() {
		_, err := s.handler.Handle(s.ctx, query.FindAvailableFlowsQuery{
			UserID:   "user-z",
			TenantID: new(approval.SharedTenantID),
			Caller:   approval.SystemCaller,
			Page:     1,
			Size:     10,
		})
		s.Require().ErrorIs(err, approval.ErrCrossTenantAccess,
			"Nothing starts in the shared tenant, so nothing is listed for it")
	})
}
