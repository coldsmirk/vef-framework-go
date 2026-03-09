package api_test

import (
	"context"
	"encoding/json"
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

// MockUserLoader is a mock implementation of security.UserLoader for testing.
type MockUserLoader struct {
	mock.Mock
}

func (m *MockUserLoader) LoadByUsername(ctx context.Context, username string) (*security.Principal, string, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.String(1), args.Error(2)
	}

	return args.Get(0).(*security.Principal), args.String(1), args.Error(2)
}

func (m *MockUserLoader) LoadByID(ctx context.Context, id string) (*security.Principal, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*security.Principal), args.Error(1)
}

// MockPermissionChecker is a mock implementation of security.PermissionChecker.
type MockPermissionChecker struct {
	mock.Mock
}

func (m *MockPermissionChecker) HasPermission(ctx context.Context, principal *security.Principal, permToken string) (bool, error) {
	args := m.Called(ctx, principal, permToken)

	return args.Bool(0), args.Error(1)
}

// TestRPCResource is a test resource for RPC API testing.
type TestRPCResource struct {
	api.Resource
}

func NewTestRPCResource() api.Resource {
	return &TestRPCResource{
		Resource: api.NewRPCResource(
			"test",
			api.WithOperations(
				api.OperationSpec{
					Action: "ping",
					Public: true,
				},
				api.OperationSpec{
					Action: "echo",
					Public: true,
				},
				api.OperationSpec{
					Action: "echo_complex",
					Public: true,
				},
				api.OperationSpec{
					Action: "echo_data",
					Public: true,
				},
				api.OperationSpec{
					Action: "protected",
				},
				api.OperationSpec{
					Action:      "audited",
					Public:      true,
					EnableAudit: true,
				},
				api.OperationSpec{
					Action:  "slow",
					Public:  true,
					Timeout: 50 * time.Millisecond,
				},
				api.OperationSpec{
					Action: "error",
					Public: true,
				},
				api.OperationSpec{
					Action:    "admin",
					PermToken: "test:admin",
				},
				api.OperationSpec{
					Action:    "restricted",
					PermToken: "test:restricted",
				},
				api.OperationSpec{
					Action: "panic",
					Public: true,
				},
			),
		),
	}
}

type EchoParams struct {
	api.P

	Message string  `json:"message"`
	Count   float64 `json:"count"`
}

type EchoComplexParams struct {
	api.P

	String  string         `json:"string"`
	Number  float64        `json:"number"`
	Float   float64        `json:"float"`
	Boolean bool           `json:"boolean"`
	Null    *string        `json:"null"`
	Array   []any          `json:"array"`
	Object  map[string]any `json:"object"`
}

type EchoDataParams struct {
	api.P

	Data string `json:"data"`
}

type PrincipalResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (*TestRPCResource) Ping(ctx fiber.Ctx) error {
	return result.Ok("pong").Response(ctx)
}

func (*TestRPCResource) Echo(ctx fiber.Ctx, params *EchoParams) error {
	return result.Ok(params).Response(ctx)
}

func (*TestRPCResource) EchoComplex(ctx fiber.Ctx, params *EchoComplexParams) error {
	return result.Ok(params).Response(ctx)
}

func (*TestRPCResource) EchoData(ctx fiber.Ctx, params *EchoDataParams) error {
	return result.Ok(params).Response(ctx)
}

func (*TestRPCResource) Protected(ctx fiber.Ctx, principal *security.Principal) error {
	return result.Ok(&PrincipalResponse{
		ID:   principal.ID,
		Name: principal.Name,
	}).Response(ctx)
}

func (*TestRPCResource) Audited(ctx fiber.Ctx) error {
	return result.Ok("audited action").Response(ctx)
}

func (*TestRPCResource) Slow(ctx fiber.Ctx) error {
	select {
	case <-time.After(100 * time.Millisecond):
	case <-ctx.Context().Done():
		return nil
	}

	return result.Ok("slow response").Response(ctx)
}

func (*TestRPCResource) Error(ctx fiber.Ctx) error {
	return result.Result{Code: result.ErrCodeDefault, Message: "intentional error"}.Response(ctx)
}

func (*TestRPCResource) Admin(ctx fiber.Ctx, principal *security.Principal) error {
	return result.Ok(map[string]any{
		"action": "admin",
		"userId": principal.ID,
	}).Response(ctx)
}

func (*TestRPCResource) Restricted(ctx fiber.Ctx) error {
	return result.Ok(map[string]any{
		"action": "restricted",
	}).Response(ctx)
}

func (*TestRPCResource) Panic(_ fiber.Ctx) error {
	panic("intentional panic for testing")
}

// RPCEngineTestSuite tests RPC API engine functionality.
type RPCEngineTestSuite struct {
	apptest.Suite

	userLoader        *MockUserLoader
	permissionChecker *MockPermissionChecker
	testUser          *security.Principal
	hashedPassword    string
}

func (suite *RPCEngineTestSuite) SetupSuite() {
	suite.T().Log("Setting up RPCEngineTestSuite")

	suite.testUser = security.NewUser("user001", "Test User", "admin", "user")
	suite.testUser.Details = map[string]any{
		"email": "test@example.com",
	}

	var err error

	suite.hashedPassword, err = password.NewBcryptEncoder().Encode("password123")
	suite.Require().NoError(err, "Should not return error")

	suite.userLoader = new(MockUserLoader)
	suite.permissionChecker = new(MockPermissionChecker)

	suite.setupTestApp()

	suite.T().Log("RPCEngineTestSuite setup complete")
}

func (suite *RPCEngineTestSuite) TearDownSuite() {
	suite.T().Log("Tearing down RPCEngineTestSuite")
	suite.TearDownApp()
	suite.T().Log("RPCEngineTestSuite teardown complete")
}

func (suite *RPCEngineTestSuite) SetupTest() {
	suite.userLoader.Calls = nil
	suite.permissionChecker.Calls = nil
}

func (suite *RPCEngineTestSuite) setupTestApp() {
	suite.userLoader.On("LoadByUsername", mock.Anything, "testuser").
		Return(suite.testUser, suite.hashedPassword, nil).
		Maybe()

	suite.userLoader.On("LoadByID", mock.Anything, "user001").
		Return(suite.testUser, nil).
		Maybe()

	suite.userLoader.On("LoadByUsername", mock.Anything, "nonexistent").
		Return(nil, "", nil).
		Maybe()

	suite.permissionChecker.On("HasPermission", mock.Anything, mock.Anything, "test:admin").
		Return(true, nil).
		Maybe()

	// Permission denied for test:restricted
	suite.permissionChecker.On("HasPermission", mock.Anything, mock.Anything, "test:restricted").
		Return(false, nil).
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
				NewTestRPCResource,
				fx.As(new(api.Resource)),
				fx.ResultTags(`group:"vef:api:resources"`),
			),
		),
	)
}

func (suite *RPCEngineTestSuite) login() string {
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

func (suite *RPCEngineTestSuite) TestPublicAPIPing() {
	suite.T().Log("Testing public API ping endpoint")

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test",
			Action:   "ping",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Ping should succeed")
	suite.Equal("pong", body.Data, "Should return pong")
}

func (suite *RPCEngineTestSuite) TestPublicAPIEcho() {
	suite.T().Log("Testing public API echo endpoint with params")

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test",
			Action:   "echo",
			Version:  "v1",
		},
		Params: map[string]any{
			"message": "hello world",
			"count":   42,
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Echo should succeed")

	data := suite.ReadDataAsMap(body.Data)
	suite.Equal("hello world", data["message"], "Message should match")
	suite.Equal(float64(42), data["count"], "Count should match")
}

func (suite *RPCEngineTestSuite) TestProtectedAPIWithoutToken() {
	suite.T().Log("Testing protected API without token")

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test",
			Action:   "protected",
			Version:  "v1",
		},
	})

	suite.Equal(401, resp.StatusCode, "Should return 401 Unauthorized")
}

func (suite *RPCEngineTestSuite) TestProtectedAPIWithValidToken() {
	suite.T().Log("Testing protected API with valid token")

	token := suite.GenerateToken(suite.testUser)

	resp := suite.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "test",
			Action:   "protected",
			Version:  "v1",
		},
	}, token)

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Protected API should succeed with valid token")

	data := suite.ReadDataAsMap(body.Data)
	suite.Equal("user001", data["id"], "User ID should match")
	suite.Equal("Test User", data["name"], "User name should match")
}

func (suite *RPCEngineTestSuite) TestProtectedAPIWithInvalidToken() {
	suite.T().Log("Testing protected API with invalid token")

	resp := suite.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "test",
			Action:   "protected",
			Version:  "v1",
		},
	}, "invalid.token.here")

	suite.Equal(401, resp.StatusCode, "Should return 401 Unauthorized")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail with invalid token")
	suite.Equal(result.ErrCodeTokenInvalid, body.Code, "Should return token invalid error")
}

func (suite *RPCEngineTestSuite) TestOperationNotFound() {
	suite.T().Log("Testing operation not found")

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test",
			Action:   "nonexistent",
			Version:  "v1",
		},
	})

	suite.Equal(404, resp.StatusCode, "Should return 404 Not Found")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail for non-existent operation")
}

func (suite *RPCEngineTestSuite) TestResourceNotFound() {
	suite.T().Log("Testing resource not found")

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "nonexistent",
			Action:   "ping",
			Version:  "v1",
		},
	})

	suite.Equal(404, resp.StatusCode, "Should return 404 Not Found")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail for non-existent resource")
}

func (suite *RPCEngineTestSuite) TestVersionMismatch() {
	suite.T().Log("Testing version mismatch")

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test",
			Action:   "ping",
			Version:  "v99",
		},
	})

	suite.Equal(404, resp.StatusCode, "Should return 404 Not Found")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail for version mismatch")
}

func (suite *RPCEngineTestSuite) TestInvalidJsonRequest() {
	suite.T().Log("Testing invalid JSON request")

	req := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/api", strings.NewReader("{invalid json}"))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	resp, err := suite.App.Test(req, 30*time.Second)
	suite.Require().NoError(err, "Should not return error")

	suite.Equal(500, resp.StatusCode, "Should return 500 Internal Server Error for invalid JSON")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail for invalid JSON")
}

func (suite *RPCEngineTestSuite) TestEmptyRequestBody() {
	suite.T().Log("Testing empty request body")

	req := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/api", strings.NewReader(""))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	resp, err := suite.App.Test(req, 30*time.Second)
	suite.Require().NoError(err, "Should not return error")

	suite.Equal(500, resp.StatusCode, "Should return 500 Internal Server Error for empty body")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail for empty request body")
}

func (suite *RPCEngineTestSuite) TestMissingRequiredFields() {
	suite.T().Log("Testing missing required fields")

	suite.Run("MissingResource", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "",
				Action:   "ping",
				Version:  "v1",
			},
		})

		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail for missing resource")
	})

	suite.Run("MissingAction", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test",
				Action:   "",
				Version:  "v1",
			},
		})

		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail for missing action")
	})

	suite.Run("MissingVersion", func() {
		resp := suite.MakeRPCRequest(api.Request{
			Identifier: api.Identifier{
				Resource: "test",
				Action:   "ping",
				Version:  "",
			},
		})

		body := suite.ReadResult(resp)
		suite.False(body.IsOk(), "Should fail for missing version")
	})
}

func (suite *RPCEngineTestSuite) TestAuditedEndpoint() {
	suite.T().Log("Testing audited endpoint")

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test",
			Action:   "audited",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Audited endpoint should succeed")
	suite.Equal("audited action", body.Data, "Should return audited action")
}

func (suite *RPCEngineTestSuite) TestErrorResponse() {
	suite.T().Log("Testing error response")

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test",
			Action:   "error",
			Version:  "v1",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should return error")
	suite.Equal(result.ErrCodeDefault, body.Code, "Should return default error code")
	suite.Equal("intentional error", body.Message, "Should return error message")
}

func (suite *RPCEngineTestSuite) TestRequestWithMeta() {
	suite.T().Log("Testing request with meta")

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test",
			Action:   "echo_data",
			Version:  "v1",
		},
		Params: map[string]any{
			"data": "test",
		},
		Meta: map[string]any{
			"traceId": "trace-123",
			"source":  "test-suite",
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Echo should succeed")
}

func (suite *RPCEngineTestSuite) TestI18nErrorMessages() {
	suite.T().Log("Testing i18n error messages")

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test",
			Action:   "protected",
			Version:  "v1",
		},
	})

	suite.Equal(401, resp.StatusCode, "Should return 401 Unauthorized")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail without token")
	suite.Equal(i18n.T(result.ErrMessageUnauthenticated), body.Message, "Should return i18n translated message")
}

func (suite *RPCEngineTestSuite) TestContentTypeValidation() {
	suite.T().Log("Testing content type validation")

	jsonBytes, err := json.Marshal(api.Request{
		Identifier: api.Identifier{
			Resource: "test",
			Action:   "ping",
			Version:  "v1",
		},
	})
	suite.Require().NoError(err, "Should not return error")

	req := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/api", strings.NewReader(string(jsonBytes)))
	req.Header.Set(fiber.HeaderContentType, "text/plain")

	resp, err := suite.App.Test(req, 30*time.Second)
	suite.Require().NoError(err, "Should not return error")

	suite.Equal(415, resp.StatusCode, "Should return 415 Unsupported Media Type")
}

func (suite *RPCEngineTestSuite) TestComplexParams() {
	suite.T().Log("Testing complex params")

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test",
			Action:   "echo_complex",
			Version:  "v1",
		},
		Params: map[string]any{
			"string":  "hello",
			"number":  123,
			"float":   3.14,
			"boolean": true,
			"null":    nil,
			"array":   []any{1, 2, 3},
			"object": map[string]any{
				"nested": "value",
			},
		},
	})

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Echo should succeed")

	data := suite.ReadDataAsMap(body.Data)
	suite.Equal("hello", data["string"], "String should match")
	suite.Equal(float64(123), data["number"], "Number should match")
	suite.Equal(3.14, data["float"], "Float should match")
	suite.Equal(true, data["boolean"], "Boolean should match")
	suite.Nil(data["null"], "Null should be nil")
	suite.NotNil(data["array"], "Array should not be nil")
	suite.NotNil(data["object"], "Object should not be nil")
}

func (suite *RPCEngineTestSuite) TestTokenInQueryParam() {
	suite.T().Log("Testing token in query parameter")

	token := suite.GenerateToken(suite.testUser)

	jsonBytes, err := json.Marshal(api.Request{
		Identifier: api.Identifier{
			Resource: "test",
			Action:   "protected",
			Version:  "v1",
		},
	})
	suite.Require().NoError(err, "Should not return error")

	req := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/api?"+security.QueryKeyAccessToken+"="+token, strings.NewReader(string(jsonBytes)))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	resp, err := suite.App.Test(req, 30*time.Second)
	suite.Require().NoError(err, "Should not return error")

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Protected API should succeed with token in query param")
}

func (suite *RPCEngineTestSuite) TestPermissionDenied() {
	suite.T().Log("Testing permission denied (403)")

	token := suite.GenerateToken(suite.testUser)

	resp := suite.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "test",
			Action:   "restricted",
			Version:  "v1",
		},
	}, token)

	suite.Equal(403, resp.StatusCode, "Should return 403 Forbidden")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail with permission denied")
	suite.Equal(result.ErrCodeAccessDenied, body.Code, "Should return access denied error code")
}

func (suite *RPCEngineTestSuite) TestSlowOperationTimeout() {
	suite.T().Log("Testing slow operation timeout")

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test",
			Action:   "slow",
			Version:  "v1",
		},
	})

	// The slow handler sleeps for 100ms but timeout is 50ms
	suite.Equal(408, resp.StatusCode, "Should return 408 Request Timeout")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail with timeout")
	suite.Equal(result.ErrCodeRequestTimeout, body.Code, "Should return request timeout error code")
}

func (suite *RPCEngineTestSuite) TestNonexistentUserLogin() {
	suite.T().Log("Testing login with nonexistent user")

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "security/auth",
			Action:   "login",
			Version:  "v1",
		},
		Params: map[string]any{
			"type":        "password",
			"principal":   "nonexistent",
			"credentials": "password123",
		},
	})

	suite.Equal(401, resp.StatusCode, "Should return 401 Unauthorized")

	body := suite.ReadResult(resp)
	suite.False(body.IsOk(), "Should fail for nonexistent user")
}

func (suite *RPCEngineTestSuite) TestAdminWithPermission() {
	suite.T().Log("Testing admin action with permission")

	token := suite.GenerateToken(suite.testUser)

	resp := suite.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "test",
			Action:   "admin",
			Version:  "v1",
		},
	}, token)

	suite.Equal(200, resp.StatusCode, "Should return 200 OK")

	body := suite.ReadResult(resp)
	suite.True(body.IsOk(), "Admin action should succeed with permission")

	data := suite.ReadDataAsMap(body.Data)
	suite.Equal("admin", data["action"], "Action should be admin")
	suite.Equal("user001", data["userId"], "User ID should match")
}

func (suite *RPCEngineTestSuite) TestHandlerPanic() {
	suite.T().Log("Testing handler panic returns 500")

	resp := suite.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{
			Resource: "test",
			Action:   "panic",
			Version:  "v1",
		},
	})

	suite.Equal(500, resp.StatusCode, "Should return 500 Internal Server Error")
}

func (suite *RPCEngineTestSuite) TestUserLoaderCalledOnLogin() {
	suite.T().Log("Testing UserLoader is called during login")

	_ = suite.login()

	suite.userLoader.AssertCalled(suite.T(), "LoadByUsername", mock.Anything, "testuser")
}

func (suite *RPCEngineTestSuite) TestPermissionCheckerCalledOnAdmin() {
	suite.T().Log("Testing PermissionChecker is called for admin action")

	token := suite.GenerateToken(suite.testUser)

	suite.permissionChecker.Calls = nil

	_ = suite.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "test",
			Action:   "admin",
			Version:  "v1",
		},
	}, token)

	suite.permissionChecker.AssertCalled(suite.T(), "HasPermission", mock.Anything, mock.Anything, "test:admin")
}

func (suite *RPCEngineTestSuite) TestPermissionCheckerCalledOnRestricted() {
	suite.T().Log("Testing PermissionChecker is called for restricted action")

	token := suite.GenerateToken(suite.testUser)

	suite.permissionChecker.Calls = nil

	_ = suite.MakeRPCRequestWithToken(api.Request{
		Identifier: api.Identifier{
			Resource: "test",
			Action:   "restricted",
			Version:  "v1",
		},
	}, token)

	suite.permissionChecker.AssertCalled(suite.T(), "HasPermission", mock.Anything, mock.Anything, "test:restricted")
}

// TestRPCEngineTestSuite tests RPC engine test suite scenarios.
func TestRPCEngine(t *testing.T) {
	suite.Run(t, new(RPCEngineTestSuite))
}
