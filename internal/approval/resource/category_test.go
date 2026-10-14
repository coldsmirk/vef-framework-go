package resource_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	iapproval "github.com/coldsmirk/vef-framework-go/internal/approval"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/internal/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

// CategoryTenancyTestSuite pins the tenant rules of approval/category over the
// full RPC path: categories are managed across tenants, so reads and creates
// follow the tenant the request names rather than the caller's own, while a
// category never changes tenant and a tree never spans tenants. It runs on the
// default in-memory SQLite datasource.
type CategoryTenancyTestSuite struct {
	apptest.Suite

	ctx context.Context
	db  orm.DB

	memberToken string
	adminToken  string
}

func TestCategoryTenancy(t *testing.T) {
	suite.Run(t, new(CategoryTenancyTestSuite))
}

func (s *CategoryTenancyTestSuite) SetupSuite() {
	s.ctx = context.Background()

	s.SetupApp(
		fx.Replace(
			&security.JWTConfig{Secret: security.DefaultJWTSecret, Audience: "test_app"},
			newApprovalConfig(),
		),
		iapproval.Module,
		fx.Provide(
			fx.Annotate(func() approval.AssigneeService { return &MockAssigneeService{} }, fx.As(new(approval.AssigneeService))),
			fx.Annotate(func() approval.UserInfoResolver { return &MockUserInfoResolver{} }, fx.As(new(approval.UserInfoResolver))),
			fx.Annotate(func() approval.PrincipalDepartmentResolver { return &MockPrincipalDepartmentResolver{} }, fx.As(new(approval.PrincipalDepartmentResolver))),
			fx.Annotate(func() approval.InstanceNoGenerator { return &MockInstanceNoGenerator{} }, fx.As(new(approval.InstanceNoGenerator))),
		),
		fx.Decorate(
			fx.Annotate(func() security.PermissionChecker { return &MockPermissionChecker{} }, fx.As(new(security.PermissionChecker))),
		),
		fx.Populate(&s.db),
	)

	s.memberToken = s.GenerateToken(newTenantUser("member", "Member", "user"))
	s.adminToken = s.GenerateToken(newTenantUser("root", "Root", approval.SuperAdminRole))
}

func (s *CategoryTenancyTestSuite) TearDownSuite() {
	s.TearDownApp()
}

func (s *CategoryTenancyTestSuite) SetupTest() {
	deleteAll(s.ctx, s.db, (*approval.FlowCategory)(nil))

	s.seedCategory("own", testTenant)
	s.seedCategory("shared", approval.SharedTenantID)
	s.seedCategory("foreign", "tenant-b")
}

func (s *CategoryTenancyTestSuite) TearDownTest() {
	deleteAll(s.ctx, s.db, (*approval.FlowCategory)(nil))
}

func (s *CategoryTenancyTestSuite) seedCategory(id, tenantID string) {
	category := &approval.FlowCategory{TenantID: tenantID, Code: id, Name: id, IsActive: true}
	category.ID = id

	_, err := s.db.NewInsert().Model(category).Exec(s.ctx)
	s.Require().NoError(err, "Should seed category %s", id)
}

func (s *CategoryTenancyTestSuite) loadCategory(id string) approval.FlowCategory {
	var category approval.FlowCategory

	category.ID = id
	s.Require().NoError(s.db.NewSelect().Model(&category).WherePK().Scan(s.ctx), "Should reload category %s", id)

	return category
}

// treeIDs returns the IDs of the root categories find_tree answers.
func (s *CategoryTenancyTestSuite) treeIDs(meta api.Meta, token string) []string {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Resource: "approval/category",
		Action:   "find_tree",
		Version:  "v1",
		Meta:     meta,
	}, token)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "find_tree should succeed")

	body := s.ReadResult(resp)
	s.Require().True(body.IsOk(), "find_tree should report success: %s", body.Message)

	ids := make([]string, 0)
	for _, item := range s.ReadDataAsSlice(body.Data) {
		ids = append(ids, s.ReadDataAsMap(item)["id"].(string))
	}

	return ids
}

// mutate sends one category write, answering the HTTP status and the result code.
func (s *CategoryTenancyTestSuite) mutate(action string, params map[string]any, token string) (status, code int) {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Resource: "approval/category",
		Action:   action,
		Version:  "v1",
		Params:   params,
	}, token)

	return resp.StatusCode, s.ReadResult(resp).Code
}

func (s *CategoryTenancyTestSuite) TestReads() {
	s.Run("ReadsEveryTenantWithoutATenant", func() {
		s.Assert().ElementsMatch([]string{"own", "shared", "foreign"}, s.treeIDs(nil, s.memberToken),
			"A read naming no tenant covers every tenant's categories")
	})

	s.Run("ReadsTheTenantTheRequestNames", func() {
		s.Assert().ElementsMatch([]string{"foreign"}, s.treeIDs(api.Meta{"tenantId": "tenant-b"}, s.memberToken),
			"The read follows the requested tenant, not the caller's own")
	})

	s.Run("AddsSharedCategoriesWhenAsked", func() {
		s.Assert().ElementsMatch([]string{"own", "shared"},
			s.treeIDs(api.Meta{"tenantId": testTenant, "includeShared": true}, s.memberToken),
			"includeShared adds the shared categories to the requested tenant's")
	})

	s.Run("ReadsSharedCategoriesAlone", func() {
		s.Assert().ElementsMatch([]string{"shared"}, s.treeIDs(api.Meta{"tenantId": approval.SharedTenantID}, s.memberToken),
			"The shared tenant can be read on its own")
	})

	s.Run("NeedsNoTenantOfTheCallersOwn", func() {
		s.Assert().ElementsMatch([]string{"own"},
			s.treeIDs(api.Meta{"tenantId": testTenant}, s.GenerateToken(security.NewUser("orphan", "Orphan", "user"))),
			"Managing categories is gated by permission, not by the caller's tenant")
	})
}

func (s *CategoryTenancyTestSuite) TestWrites() {
	s.Run("CreatesInTheRequestedTenant", func() {
		status, _ := s.mutate("create", map[string]any{
			"id": "member-shared", "tenantId": approval.SharedTenantID, "code": "member-shared", "name": "Member Shared",
		}, s.memberToken)
		s.Require().Equal(http.StatusOK, status, "The create itself should succeed")
		s.Assert().Equal(approval.SharedTenantID, s.loadCategory("member-shared").TenantID,
			"A category is stamped with the tenant the request names")
	})

	s.Run("RequiresATenant", func() {
		_, code := s.mutate("create", map[string]any{"id": "no-tenant", "code": "no-tenant", "name": "No Tenant"}, s.memberToken)
		s.Assert().Equal(result.ErrCodeBadRequest, code, "A category naming no tenant should fail validation")
	})

	s.Run("CategoryNeverChangesTenant", func() {
		for _, token := range []string{s.memberToken, s.adminToken} {
			status, _ := s.mutate("update", map[string]any{
				"id": "own", "tenantId": approval.SharedTenantID, "code": "own", "name": "Renamed",
			}, token)
			s.Require().Equal(http.StatusOK, status, "The update itself should succeed")

			stored := s.loadCategory("own")
			s.Assert().Equal(testTenant, stored.TenantID, "An update must not move a category to another tenant")
			s.Assert().Equal("Renamed", stored.Name, "The legitimate change should still apply")
		}
	})

	s.Run("RejectsParentOfAnotherTenant", func() {
		_, code := s.mutate("create", map[string]any{
			"id": "child-shared", "tenantId": testTenant, "code": "child-shared", "name": "Child", "parentId": "shared",
		}, s.memberToken)
		s.Assert().Equal(approval.ErrCodeCategoryNotFound, code, "A tenant category cannot hang off a shared one")

		_, code = s.mutate("update", map[string]any{
			"id": "own", "tenantId": testTenant, "code": "own", "name": "own", "parentId": "foreign",
		}, s.memberToken)
		s.Assert().Equal(approval.ErrCodeCategoryNotFound, code, "A category cannot move under another tenant's")
	})

	s.Run("AcceptsParentOfOwnTenant", func() {
		status, code := s.mutate("create", map[string]any{
			"id": "child-own", "tenantId": testTenant, "code": "child-own", "name": "Child", "parentId": "own",
		}, s.memberToken)
		s.Require().Equal(http.StatusOK, status, "A child of an own category should be created (code %d)", code)
		s.Assert().Equal("own", *s.loadCategory("child-own").ParentID, "The parent should be kept")
	})
}
