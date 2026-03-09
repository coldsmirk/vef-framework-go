package resource_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/approval"
	"github.com/coldsmirk/vef-framework-go/config"
	iapproval "github.com/coldsmirk/vef-framework-go/internal/approval"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

type DenyPermissionChecker struct{}

func (*DenyPermissionChecker) HasPermission(context.Context, *security.Principal, string) (bool, error) {
	return false, nil
}

type PermissionEnforcementResourceTestSuite struct {
	apptest.Suite

	ctx   context.Context
	db    orm.DB
	token string
}

func TestPermissionEnforcementResource(t *testing.T) {
	suite.Run(t, new(PermissionEnforcementResourceTestSuite))
}

func (s *PermissionEnforcementResourceTestSuite) SetupSuite() {
	s.ctx = context.Background()
	pgContainer := testx.NewPostgresContainer(s.ctx, s.T())

	s.SetupApp(
		fx.Replace(
			pgContainer.DataSource,
			&security.JWTConfig{
				Secret:   security.DefaultJWTSecret,
				Audience: "test_app",
			},
			&config.ApprovalConfig{AutoMigrate: true},
		),
		fx.Provide(func() context.Context { return s.ctx }),
		iapproval.Module,
		fx.Provide(
			fx.Annotate(func() approval.AssigneeService { return &MockAssigneeService{} }, fx.As(new(approval.AssigneeService))),
			fx.Annotate(func() approval.UserInfoResolver { return &MockUserInfoResolver{} }, fx.As(new(approval.UserInfoResolver))),
			fx.Annotate(func() approval.PrincipalDeptResolver { return &MockPrincipalDeptResolver{} }, fx.As(new(approval.PrincipalDeptResolver))),
			fx.Annotate(func() approval.InstanceNoGenerator { return &MockInstanceNoGenerator{} }, fx.As(new(approval.InstanceNoGenerator))),
		),
		fx.Decorate(
			fx.Annotate(func() security.PermissionChecker { return &DenyPermissionChecker{} }, fx.As(new(security.PermissionChecker))),
		),
		fx.Populate(&s.db),
	)

	s.token = s.GenerateToken(security.NewUser("test-user", "user", "user"))
	cleanAllApprovalData(s.ctx, s.db)
	_, _ = s.db.NewInsert().Model(&approval.FlowCategory{
		TenantID: "default",
		Code:     "perm-cat",
		Name:     "Permission Category",
		IsActive: true,
	}).Exec(s.ctx)
}

func (s *PermissionEnforcementResourceTestSuite) TearDownSuite() {
	s.TearDownApp()
}

func (s *PermissionEnforcementResourceTestSuite) TestFlowCreateShouldReturn403() {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/flow", Action: "create", Version: "v1"},
		Params: map[string]any{
			"tenantId":               "default",
			"code":                   "perm-flow",
			"name":                   "Permission Flow",
			"categoryId":             "some-category-id",
			"bindingMode":            "standalone",
			"isAllInitiationAllowed": true,
			"instanceTitleTemplate":  "Test",
		},
	}, s.token)

	s.Require().Equal(http.StatusForbidden, resp.StatusCode, "Flow create should be denied by permission middleware")
	body := s.ReadResult(resp)
	s.False(body.IsOk(), "Flow create should fail")
	s.Equal(result.ErrCodeAccessDenied, body.Code, "Flow create should return access denied code")
}

func (s *PermissionEnforcementResourceTestSuite) TestCategoryCreateShouldReturn403() {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/category", Action: "create", Version: "v1"},
		Params: map[string]any{
			"tenantId": "default",
			"code":     "perm-category",
			"name":     "Permission Category",
		},
	}, s.token)

	s.Require().Equal(http.StatusForbidden, resp.StatusCode, "Category create should be denied by permission middleware")
	body := s.ReadResult(resp)
	s.False(body.IsOk(), "Category create should fail")
	s.Equal(result.ErrCodeAccessDenied, body.Code, "Category create should return access denied code")
}

func (s *PermissionEnforcementResourceTestSuite) TestDelegationFindPageShouldReturn403() {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/delegation", Action: "find_page", Version: "v1"},
		Params:     map[string]any{"page": 1, "pageSize": 10},
	}, s.token)

	s.Require().Equal(http.StatusForbidden, resp.StatusCode, "Delegation query should be denied by permission middleware")
	body := s.ReadResult(resp)
	s.False(body.IsOk(), "Delegation query should fail")
	s.Equal(result.ErrCodeAccessDenied, body.Code, "Delegation query should return access denied code")
}

func (s *PermissionEnforcementResourceTestSuite) TestMyPendingCountsShouldStillAllowAuthenticatedAccess() {
	resp := s.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{Resource: "approval/my", Action: "get_pending_counts", Version: "v1"},
	}, s.token)

	s.Require().Equal(http.StatusOK, resp.StatusCode, "Self-service query without PermToken should still pass")
	body := s.ReadResult(resp)
	s.True(body.IsOk(), "Self-service query should succeed without permission token")
	data := s.ReadDataAsMap(body.Data)
	s.Equal(float64(0), data["pendingTaskCount"], "Pending task count should default to zero")
	s.Equal(float64(0), data["unreadCcCount"], "Unread CC count should default to zero")
}
