package api_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/i18n"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/password"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

// TestRESTResource is a test resource for REST API testing.
type TestRESTResource struct {
	api.Resource
}

func NewTestRESTResource() api.Resource {
	return &TestRESTResource{
		Resource: api.NewRESTResource(
			"items",
			api.WithOperations(
				api.OperationSpec{
					Action:  "get",
					Public:  true,
					Handler: "Get",
				},
				api.OperationSpec{
					Action:  "post",
					Public:  true,
					Handler: "Post",
				},
				api.OperationSpec{
					Action:  "put",
					Handler: "Put",
				},
				api.OperationSpec{
					Action:  "delete",
					Handler: "Delete",
				},
				api.OperationSpec{
					Action:    "patch",
					Handler:   "Patch",
					PermToken: "items:patch",
				},
				api.OperationSpec{
					Action:    "post admin",
					Handler:   "Admin",
					PermToken: "items:admin",
				},
				api.OperationSpec{
					Action:  "get panic",
					Public:  true,
					Handler: "Panic",
				},
			),
		),
	}
}

type GetItemsParams struct {
	api.P

	ID      string `json:"id"`
	Keyword string `json:"keyword"`
	Page    string `json:"page"`
	Size    string `json:"size"`
}

type CreateItemParams struct {
	api.P

	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

type UpdateItemParams struct {
	api.P

	ID   string `json:"id" validate:"required" label:"ID"`
	Name string `json:"name"`
}

type DeleteItemParams struct {
	api.P

	ID string `json:"id" validate:"required" label:"ID"`
}

type PatchItemParams struct {
	api.P

	ID     string `json:"id" validate:"required" label:"ID"`
	Status string `json:"status"`
}

type Item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ItemListResponse struct {
	Items   []Item `json:"items"`
	Keyword string `json:"keyword"`
	Page    string `json:"page"`
	Size    string `json:"size"`
}

type CreatedItemResponse struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

type UpdatedItemResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	UpdatedBy string `json:"updatedBy"`
}

type DeletedItemResponse struct {
	ID        string `json:"id"`
	DeletedBy string `json:"deletedBy"`
}

type PatchedItemResponse struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Patched bool   `json:"patched"`
}

type AdminResponse struct {
	Action string `json:"action"`
	Admin  bool   `json:"admin"`
}

func (*TestRESTResource) Get(ctx fiber.Ctx, params *GetItemsParams) error {
	// If id is provided, return single item
	if params.ID != "" {
		if params.ID == "notfound" {
			return result.Result{Code: result.ErrCodeRecordNotFound, Message: i18n.T(result.ErrMessageRecordNotFound)}.Response(ctx)
		}

		return result.Ok(&Item{
			ID:   params.ID,
			Name: "Item " + params.ID,
		}).Response(ctx)
	}

	// Default values
	page := params.Page
	if page == "" {
		page = "1"
	}

	size := params.Size
	if size == "" {
		size = "10"
	}

	return result.Ok(&ItemListResponse{
		Items: []Item{
			{ID: "1", Name: "Item 1"},
			{ID: "2", Name: "Item 2"},
		},
		Keyword: params.Keyword,
		Page:    page,
		Size:    size,
	}).Response(ctx)
}

func (*TestRESTResource) Post(ctx fiber.Ctx, params *CreateItemParams) error {
	return result.Ok(&CreatedItemResponse{
		ID:    "new-id",
		Name:  params.Name,
		Price: params.Price,
	}).Response(ctx)
}

func (*TestRESTResource) Put(ctx fiber.Ctx, params *UpdateItemParams, principal *security.Principal) error {
	return result.Ok(&UpdatedItemResponse{
		ID:        params.ID,
		Name:      params.Name,
		UpdatedBy: principal.ID,
	}).Response(ctx)
}

func (*TestRESTResource) Delete(ctx fiber.Ctx, params *DeleteItemParams, principal *security.Principal) error {
	return result.Ok(&DeletedItemResponse{
		ID:        params.ID,
		DeletedBy: principal.ID,
	}).Response(ctx)
}

func (*TestRESTResource) Patch(ctx fiber.Ctx, params *PatchItemParams) error {
	return result.Ok(&PatchedItemResponse{
		ID:      params.ID,
		Status:  params.Status,
		Patched: true,
	}).Response(ctx)
}

func (*TestRESTResource) Admin(ctx fiber.Ctx) error {
	return result.Ok(&AdminResponse{
		Action: "admin",
		Admin:  true,
	}).Response(ctx)
}

func (*TestRESTResource) Panic(_ fiber.Ctx) error {
	panic("intentional panic for testing")
}

// RESTEngineTestSuite tests REST API engine functionality.
type RESTEngineTestSuite struct {
	apptest.Suite

	userLoader        *MockUserLoader
	permissionChecker *MockPermissionChecker
	testUser          *security.Principal
	adminUser         *security.Principal
	hashedPassword    string
}

func (suite *RESTEngineTestSuite) SetupSuite() {
	suite.T().Log("Setting up RESTEngineTestSuite")

	suite.testUser = security.NewUser("user001", "Test User", "admin", "user")
	suite.testUser.Details = map[string]any{
		"email": "test@example.com",
	}

	// Admin user with items:admin permission
	suite.adminUser = security.NewUser("admin001", "Admin User", "superadmin")
	suite.adminUser.Details = map[string]any{
		"email": "admin@example.com",
	}

	var err error

	suite.hashedPassword, err = password.NewBcryptEncoder().Encode("password123")
	suite.Require().NoError(err, "Should not return error")

	suite.userLoader = new(MockUserLoader)
	suite.permissionChecker = new(MockPermissionChecker)

	suite.setupTestApp()

	suite.T().Log("RESTEngineTestSuite setup complete")
}

func (suite *RESTEngineTestSuite) TearDownSuite() {
	suite.T().Log("Tearing down RESTEngineTestSuite")
	suite.TearDownApp()
	suite.T().Log("RESTEngineTestSuite teardown complete")
}

func (suite *RESTEngineTestSuite) SetupTest() {
	suite.userLoader.Calls = nil
	suite.permissionChecker.Calls = nil
}

func (suite *RESTEngineTestSuite) setupTestApp() {
	suite.userLoader.On("LoadByUsername", mock.Anything, "testuser").
		Return(suite.testUser, suite.hashedPassword, nil).
		Maybe()

	suite.userLoader.On("LoadByID", mock.Anything, "user001").
		Return(suite.testUser, nil).
		Maybe()

	// Admin user for admin-allow test
	suite.userLoader.On("LoadByUsername", mock.Anything, "adminuser").
		Return(suite.adminUser, suite.hashedPassword, nil).
		Maybe()

	suite.userLoader.On("LoadByID", mock.Anything, "admin001").
		Return(suite.adminUser, nil).
		Maybe()

	suite.permissionChecker.On("HasPermission", mock.Anything, mock.Anything, "items:patch").
		Return(true, nil).
		Maybe()

	// Permission denied for items:admin for regular user
	suite.permissionChecker.On("HasPermission", mock.Anything, mock.MatchedBy(func(p *security.Principal) bool {
		return p.ID == suite.testUser.ID
	}), "items:admin").
		Return(false, nil).
		Maybe()

	// Permission allowed for items:admin for admin user
	suite.permissionChecker.On("HasPermission", mock.Anything, mock.MatchedBy(func(p *security.Principal) bool {
		return p.ID == suite.adminUser.ID
	}), "items:admin").
		Return(true, nil).
		Maybe()

	suite.SetupApp(
		fx.Supply(
			fx.Annotate(
				suite.userLoader,
				fx.As(new(security.UserLoader)),
			),
		),
		fx.Decorate(func() security.PermissionChecker {
			return suite.permissionChecker
		}),
		fx.Replace(
			&config.DataSourceConfig{
				Kind: config.SQLite,
			},
			&security.JWTConfig{
				Secret:   security.DefaultJWTSecret,
				Audience: "test_app",
			},
		),
		fx.Provide(
			fx.Annotate(
				NewTestRESTResource,
				fx.As(new(api.Resource)),
				fx.ResultTags(`group:"vef:api:resources"`),
			),
		),
	)
}

func (suite *RESTEngineTestSuite) login() string {
	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "login",
			Version:  "v1",
		},
		Params: map[string]any{
			"type":        "password",
			"principal":   "testuser",
			"credentials": "password123",
		},
	})

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Login should succeed")

	data := suite.ReadDataAsMap(body.Data)
	tokens := suite.ReadDataAsMap(data["tokens"])

	return tokens["accessToken"].(string)
}

func (suite *RESTEngineTestSuite) TestGetList() {
	suite.T().Log("Testing GET list endpoint")

	resp := suite.MakeRESTRequest(fiber.MethodGet, "/api/items", "")

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "GET list should succeed")

	data := suite.ReadDataAsMap(body.Data)
	suite.NotNil(data["items"], "Should have items")
	suite.Equal("1", data["page"], "Default page should be 1")
	suite.Equal("10", data["size"], "Default size should be 10")
}

func (suite *RESTEngineTestSuite) TestGetListWithQueryParams() {
	suite.T().Log("Testing GET list with query parameters")

	resp := suite.MakeRESTRequest(fiber.MethodGet, "/api/items?keyword=test&page=2&size=20", "")

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "GET list should succeed")

	data := suite.ReadDataAsMap(body.Data)
	suite.Equal("test", data["keyword"], "Keyword should match")
	suite.Equal("2", data["page"], "Page should match")
	suite.Equal("20", data["size"], "Size should match")
}

func (suite *RESTEngineTestSuite) TestGetByID() {
	suite.T().Log("Testing GET by ID endpoint")

	resp := suite.MakeRESTRequest(fiber.MethodGet, "/api/items?id=123", "")

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "GET by ID should succeed")

	data := suite.ReadDataAsMap(body.Data)
	suite.Equal("123", data["id"], "ID should match")
	suite.Equal("Item 123", data["name"], "Name should match")
}

func (suite *RESTEngineTestSuite) TestGetByIDNotFound() {
	suite.T().Log("Testing GET by ID not found")

	resp := suite.MakeRESTRequest(fiber.MethodGet, "/api/items?id=notfound", "")

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "GET by ID should fail for not found")
	suite.Equal(result.ErrCodeRecordNotFound, body.Code, "Should return record not found error")
}

func (suite *RESTEngineTestSuite) TestPostCreate() {
	suite.T().Log("Testing POST create endpoint")

	resp := suite.MakeRESTRequest(fiber.MethodPost, "/api/items", `{"name":"New Item","price":100}`)

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "POST create should succeed")

	data := suite.ReadDataAsMap(body.Data)
	suite.Equal("new-id", data["id"], "Should have generated ID")
	suite.Equal("New Item", data["name"], "Name should match")
	suite.Equal(float64(100), data["price"], "Price should match")
}

func (suite *RESTEngineTestSuite) TestPostCreateInvalidBody() {
	suite.T().Log("Testing POST create with invalid body")

	resp := suite.MakeRESTRequest(fiber.MethodPost, "/api/items", "{invalid json}")

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "POST create should fail with invalid body")
	suite.Equal(result.ErrCodeBadRequest, body.Code, "Should return bad request error")
}

func (suite *RESTEngineTestSuite) TestPutUpdateWithoutToken() {
	suite.T().Log("Testing PUT update without token")

	resp := suite.MakeRESTRequest(fiber.MethodPut, "/api/items", `{"id":"123","name":"Updated Item"}`)

	suite.Equal(401, resp.StatusCode, "Should return 401 Unauthorized")
}

func (suite *RESTEngineTestSuite) TestPutUpdateWithToken() {
	suite.T().Log("Testing PUT update with token")

	token := suite.GenerateToken(suite.testUser)

	resp := suite.MakeRESTRequestWithToken(fiber.MethodPut, "/api/items", `{"id":"123","name":"Updated Item"}`, token)

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "PUT update should succeed")

	data := suite.ReadDataAsMap(body.Data)
	suite.Equal("123", data["id"], "ID should match")
	suite.Equal("Updated Item", data["name"], "Name should match")
	suite.Equal("user001", data["updatedBy"], "UpdatedBy should match principal ID")
}

func (suite *RESTEngineTestSuite) TestDeleteWithoutToken() {
	suite.T().Log("Testing DELETE without token")

	resp := suite.MakeRESTRequest(fiber.MethodDelete, "/api/items?id=123", "")

	suite.Equal(401, resp.StatusCode, "Should return 401 Unauthorized")
}

func (suite *RESTEngineTestSuite) TestDeleteWithToken() {
	suite.T().Log("Testing DELETE with token")

	token := suite.GenerateToken(suite.testUser)

	resp := suite.MakeRESTRequestWithToken(fiber.MethodDelete, "/api/items?id=123", "", token)

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "DELETE should succeed")

	data := suite.ReadDataAsMap(body.Data)
	suite.Equal("123", data["id"], "ID should match")
	suite.Equal("user001", data["deletedBy"], "DeletedBy should match principal ID")
}

func (suite *RESTEngineTestSuite) TestPatchWithPermission() {
	suite.T().Log("Testing PATCH with permission")

	token := suite.GenerateToken(suite.testUser)

	resp := suite.MakeRESTRequestWithToken(fiber.MethodPatch, "/api/items", `{"id":"123","status":"active"}`, token)

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "PATCH should succeed with permission")

	data := suite.ReadDataAsMap(body.Data)
	suite.Equal("123", data["id"], "ID should match")
	suite.Equal("active", data["status"], "Status should match")
	suite.Equal(true, data["patched"], "Patched flag should be true")
}

func (suite *RESTEngineTestSuite) TestRouteNotFound() {
	suite.T().Log("Testing route not found")

	resp := suite.MakeRESTRequest(fiber.MethodGet, "/api/nonexistent", "")

	// REST router returns 404 for unmatched routes since each route has its own middleware
	suite.Equal(404, resp.StatusCode, "Should return 404 Not Found for unmatched route")
}

func (suite *RESTEngineTestSuite) TestMethodNotAllowed() {
	suite.T().Log("Testing method not allowed")

	resp := suite.MakeRESTRequest(fiber.MethodOptions, "/api/items", "")

	// REST router returns 405 Method Not Allowed for unregistered methods on existing paths
	suite.Equal(405, resp.StatusCode, "Should return 405 Method Not Allowed for unregistered method")
}

func (suite *RESTEngineTestSuite) TestInvalidToken() {
	suite.T().Log("Testing invalid token")

	resp := suite.MakeRESTRequestWithToken(fiber.MethodPut, "/api/items", `{"id":"123","name":"Test"}`, "invalid.token.here")

	suite.Equal(401, resp.StatusCode, "Should return 401 Unauthorized")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail with invalid token")
	suite.Equal(result.ErrCodeTokenInvalid, body.Code, "Should return token invalid error")
}

func (suite *RESTEngineTestSuite) TestI18nErrorMessages() {
	suite.T().Log("Testing i18n error messages")

	resp := suite.MakeRESTRequest(fiber.MethodPut, "/api/items", `{"id":"123","name":"Test"}`)

	suite.Equal(401, resp.StatusCode, "Should return 401 Unauthorized")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail without token")
	suite.Equal(i18n.T(result.ErrMessageUnauthenticated), body.Message, "Should return i18n translated message")
}

func (suite *RESTEngineTestSuite) TestEmptyRequestBody() {
	suite.T().Log("Testing empty request body for POST")

	req := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/api/items", strings.NewReader(""))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	resp, err := suite.App.Test(req, 30*time.Second)
	suite.Require().NoError(err, "Should not return error")

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	// Empty body is allowed for REST POST - handler receives empty params
	suite.True(body.IsOk(), "Should succeed with empty body")
}

func (suite *RESTEngineTestSuite) TestTokenInQueryParam() {
	suite.T().Log("Testing token in query parameter")

	token := suite.GenerateToken(suite.testUser)

	req := httptest.NewRequestWithContext(context.Background(), fiber.MethodPut, "/api/items?"+security.QueryKeyAccessToken+"="+token, strings.NewReader(`{"id":"123","name":"Test"}`))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	resp, err := suite.App.Test(req, 30*time.Second)
	suite.Require().NoError(err, "Should not return error")

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "PUT should succeed with token in query param")
}

func (suite *RESTEngineTestSuite) TestComplexQueryParams() {
	suite.T().Log("Testing complex query parameters")

	resp := suite.MakeRESTRequest(fiber.MethodGet, "/api/items?keyword=test%20item&page=1&size=10", "")

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "GET should succeed")

	data := suite.ReadDataAsMap(body.Data)
	suite.Equal("test item", data["keyword"], "URL-encoded keyword should be decoded")
}

func (suite *RESTEngineTestSuite) TestPermissionDenied() {
	suite.T().Log("Testing permission denied (403)")

	token := suite.GenerateToken(suite.testUser)

	resp := suite.MakeRESTRequestWithToken(fiber.MethodPost, "/api/items/admin", `{}`, token)

	suite.Equal(403, resp.StatusCode, "Should return 403 Forbidden")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail with permission denied")
	suite.Equal(result.ErrCodeAccessDenied, body.Code, "Should return access denied error code")
}

func (suite *RESTEngineTestSuite) TestAdminWithPermission() {
	suite.T().Log("Testing admin action with permission")

	token := suite.GenerateToken(suite.adminUser)

	resp := suite.MakeRESTRequestWithToken(fiber.MethodPost, "/api/items/admin", `{}`, token)

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Admin action should succeed with permission")

	data := suite.ReadDataAsMap(body.Data)
	suite.Equal("admin", data["action"], "Action should be admin")
	suite.Equal(true, data["admin"], "Admin flag should be true")
}

func (suite *RESTEngineTestSuite) TestPutMissingRequiredID() {
	suite.T().Log("Testing PUT with missing required ID")

	token := suite.GenerateToken(suite.testUser)

	// PUT without id in body
	resp := suite.MakeRESTRequestWithToken(fiber.MethodPut, "/api/items", `{"name":"Test"}`, token)

	suite.Equal(400, resp.StatusCode, "Should return 400 Bad Request")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail with validation error")
	suite.Equal(result.ErrCodeBadRequest, body.Code, "Should return bad request error code")
}

func (suite *RESTEngineTestSuite) TestDeleteMissingRequiredID() {
	suite.T().Log("Testing DELETE with missing required ID")

	token := suite.GenerateToken(suite.testUser)

	// DELETE without id - send empty JSON body
	resp := suite.MakeRESTRequestWithToken(fiber.MethodDelete, "/api/items", `{}`, token)

	suite.Equal(400, resp.StatusCode, "Should return 400 Bad Request")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail with validation error")
	suite.Equal(result.ErrCodeBadRequest, body.Code, "Should return bad request error code")
}

func (suite *RESTEngineTestSuite) TestPatchMissingRequiredID() {
	suite.T().Log("Testing PATCH with missing required ID")

	token := suite.GenerateToken(suite.testUser)

	// PATCH without id in body
	resp := suite.MakeRESTRequestWithToken(fiber.MethodPatch, "/api/items", `{"status":"active"}`, token)

	suite.Equal(400, resp.StatusCode, "Should return 400 Bad Request")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail with validation error")
	suite.Equal(result.ErrCodeBadRequest, body.Code, "Should return bad request error code")
}

func (suite *RESTEngineTestSuite) TestHandlerPanic() {
	suite.T().Log("Testing handler panic returns 500")

	resp := suite.MakeRESTRequest(fiber.MethodGet, "/api/items/panic", "")

	suite.Equal(500, resp.StatusCode, "Should return 500 Internal Server Error")
}

func (suite *RESTEngineTestSuite) TestContentTypeValidation() {
	suite.T().Log("Testing content type validation for REST")

	req := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/api/items", strings.NewReader(`{"name":"Test"}`))
	req.Header.Set(fiber.HeaderContentType, "text/plain")

	resp, err := suite.App.Test(req, 30*time.Second)
	suite.Require().NoError(err, "Should not return error")

	suite.Equal(415, resp.StatusCode, "Should return 415 Unsupported Media Type")
}

func (suite *RESTEngineTestSuite) TestPutInvalidJSON() {
	suite.T().Log("Testing PUT with invalid JSON")

	token := suite.GenerateToken(suite.testUser)

	resp := suite.MakeRESTRequestWithToken(fiber.MethodPut, "/api/items", "{invalid json}", token)

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail with invalid JSON")
	suite.Equal(result.ErrCodeBadRequest, body.Code, "Should return bad request error code")
}

func (suite *RESTEngineTestSuite) TestPatchInvalidJSON() {
	suite.T().Log("Testing PATCH with invalid JSON")

	token := suite.GenerateToken(suite.testUser)

	resp := suite.MakeRESTRequestWithToken(fiber.MethodPatch, "/api/items", "{invalid json}", token)

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail with invalid JSON")
	suite.Equal(result.ErrCodeBadRequest, body.Code, "Should return bad request error code")
}

func (suite *RESTEngineTestSuite) TestDeleteInvalidJSON() {
	suite.T().Log("Testing DELETE with invalid JSON body")

	token := suite.GenerateToken(suite.testUser)

	resp := suite.MakeRESTRequestWithToken(fiber.MethodDelete, "/api/items", "{invalid json}", token)

	suite.Equal(400, resp.StatusCode, "Should return 400 Bad Request")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail with invalid JSON")
	suite.Equal(result.ErrCodeBadRequest, body.Code, "Should return bad request error code")
}

func (suite *RESTEngineTestSuite) TestUserLoaderCalledOnLogin() {
	suite.T().Log("Testing UserLoader is called during login")

	_ = suite.login()

	suite.userLoader.AssertCalled(suite.T(), "LoadByUsername", mock.Anything, "testuser")
}

func (suite *RESTEngineTestSuite) TestPermissionCheckerCalledOnPatch() {
	suite.T().Log("Testing PermissionChecker is called for PATCH")

	token := suite.GenerateToken(suite.testUser)

	suite.permissionChecker.Calls = nil

	_ = suite.MakeRESTRequestWithToken(fiber.MethodPatch, "/api/items", `{"id":"123","status":"active"}`, token)

	suite.permissionChecker.AssertCalled(suite.T(), "HasPermission", mock.Anything, mock.Anything, "items:patch")
}

func (suite *RESTEngineTestSuite) TestPermissionCheckerCalledOnAdmin() {
	suite.T().Log("Testing PermissionChecker is called for admin action")

	token := suite.GenerateToken(suite.testUser)

	suite.permissionChecker.Calls = nil

	_ = suite.MakeRESTRequestWithToken(fiber.MethodPost, "/api/items/admin", `{}`, token)

	suite.permissionChecker.AssertCalled(suite.T(), "HasPermission", mock.Anything, mock.Anything, "items:admin")
}

// TestRESTEngineTestSuite tests REST engine test suite scenarios.
func TestRESTEngine(t *testing.T) {
	suite.Run(t, new(RESTEngineTestSuite))
}
