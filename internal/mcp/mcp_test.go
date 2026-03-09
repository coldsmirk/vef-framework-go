package mcp_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/password"
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

// MCPTestSuite tests the MCP endpoint functionality with authentication.
type MCPTestSuite struct {
	apptest.Suite

	ctx        context.Context
	userLoader *MockUserLoader
	testUser   *security.Principal
}

// SetupSuite runs once before all tests in the suite.
func (suite *MCPTestSuite) SetupSuite() {
	suite.T().Log("Setting up MCPTestSuite - initializing test app with MCP")

	suite.ctx = context.Background()

	suite.testUser = security.NewUser("user001", "Test User", "admin", "user")
	suite.testUser.Details = map[string]any{
		"email": "test@example.com",
	}

	suite.userLoader = new(MockUserLoader)

	suite.setupTestApp()

	suite.T().Log("MCPTestSuite setup complete - test app ready")
}

// TearDownSuite runs once after all tests in the suite.
func (suite *MCPTestSuite) TearDownSuite() {
	suite.T().Log("Tearing down MCPTestSuite")
	suite.TearDownApp()
	suite.T().Log("MCPTestSuite teardown complete")
}

// SetupTest runs before each test.
func (suite *MCPTestSuite) SetupTest() {
	suite.userLoader.Calls = nil
}

func (suite *MCPTestSuite) setupTestApp() {
	hashedPassword, err := password.NewBcryptEncoder().Encode("password123")
	suite.Require().NoError(err, "Should not return error")

	suite.SetupApp(
		fx.Supply(
			fx.Annotate(
				suite.userLoader,
				fx.As(new(security.UserLoader)),
			),
		),
		fx.Replace(
			&config.DataSourceConfig{
				Kind: "sqlite",
			},
			&config.MCPConfig{
				Enabled:     true,
				RequireAuth: true,
			},
			&config.SecurityConfig{
				TokenExpires: 24 * time.Hour,
			},
			&security.JWTConfig{
				Secret:   security.DefaultJWTSecret,
				Audience: "test_app",
			},
		),
		fx.Invoke(func() {
			suite.userLoader.On("LoadByUsername", mock.Anything, "testuser").
				Return(suite.testUser, hashedPassword, nil).
				Maybe()

			suite.userLoader.On("LoadByID", mock.Anything, "user001").
				Return(suite.testUser, nil).
				Maybe()
		}),
	)
}

// Helper methods

func (suite *MCPTestSuite) makeMCPRequest(body string) *http.Response {
	req := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := suite.App.Test(req, 30*time.Second)
	suite.Require().NoError(err, "MCP request should not fail")

	return resp
}

func (suite *MCPTestSuite) makeMCPRequestWithToken(body, token string) *http.Response {
	req := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set(fiber.HeaderAuthorization, security.AuthSchemeBearer+" "+token)

	resp, err := suite.App.Test(req, 30*time.Second)
	suite.Require().NoError(err, "MCP request should not fail")

	return resp
}

func (suite *MCPTestSuite) readBody(resp *http.Response) string {
	body, err := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	suite.Require().NoError(err, "Should read response body")

	return string(body)
}

// Test Cases

// TestMCPEndpointRequiresAuthentication tests that MCP endpoint requires authentication.
// It verifies that requests without valid tokens are rejected with 401 Unauthorized.
func (suite *MCPTestSuite) TestMCPEndpointRequiresAuthentication() {
	suite.T().Log("Testing MCP endpoint authentication requirement")

	// Test 1: Request without any token
	suite.Run("RequestWithoutToken", func() {
		body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`

		resp := suite.makeMCPRequest(body)

		suite.Equal(401, resp.StatusCode, "Should return 401 Unauthorized without token")
		suite.T().Log("Request without token correctly rejected with 401")
	})

	// Test 2: Request with invalid token
	suite.Run("RequestWithInvalidToken", func() {
		body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`

		resp := suite.makeMCPRequestWithToken(body, "invalid.token.here")

		suite.Equal(401, resp.StatusCode, "Should return 401 Unauthorized with invalid token")
		suite.T().Log("Request with invalid token correctly rejected with 401")
	})

	// Test 3: Request with empty token
	suite.Run("RequestWithEmptyToken", func() {
		body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`

		resp := suite.makeMCPRequestWithToken(body, "")

		suite.Equal(401, resp.StatusCode, "Should return 401 Unauthorized with empty token")
		suite.T().Log("Request with empty token correctly rejected with 401")
	})
}

// TestMCPEndpointWithValidToken tests MCP endpoint accepts valid authentication.
// It verifies that authenticated requests are processed successfully.
func (suite *MCPTestSuite) TestMCPEndpointWithValidToken() {
	suite.T().Log("Testing MCP endpoint with valid authentication")

	// Test 1: Initialize with valid JWT token
	suite.Run("InitializeWithValidToken", func() {
		token := suite.GenerateToken(suite.testUser)
		suite.NotEmpty(token, "Should get valid access token")
		suite.T().Logf("Got access token: %s...", token[:20])

		body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`

		resp := suite.makeMCPRequestWithToken(body, token)

		// Step 1: Should not return 401 when token is valid
		suite.NotEqual(401, resp.StatusCode, "Should not return 401 with valid token")
		suite.Equal(200, resp.StatusCode, "Should return 200 OK with valid token")

		// Step 2: Validate response body
		responseBody := suite.readBody(resp)
		suite.T().Logf("MCP Response: %s", responseBody)

		// Step 3: Check for valid JSON-RPC response
		suite.Contains(responseBody, "jsonrpc", "Response should contain jsonrpc field")
		suite.Contains(responseBody, "result", "Response should contain result field")
		suite.Contains(responseBody, "serverInfo", "Response should contain serverInfo")
	})
}

// TestMCPEndpointMethods tests various MCP methods with authentication.
func (suite *MCPTestSuite) TestMCPEndpointMethods() {
	suite.T().Log("Testing MCP endpoint methods")

	token := suite.GenerateToken(suite.testUser)
	suite.NotEmpty(token, "Should get valid access token")

	suite.Run("ListTools", func() {
		// First initialize
		initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`
		initResp := suite.makeMCPRequestWithToken(initBody, token)
		suite.NotEqual(401, initResp.StatusCode, "Initialize should not return 401")

		// Then list tools
		body := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`

		resp := suite.makeMCPRequestWithToken(body, token)

		suite.NotEqual(401, resp.StatusCode, "ListTools should not return 401 with valid token")

		responseBody := suite.readBody(resp)
		suite.T().Logf("ListTools Response: %s", responseBody)
	})

	suite.Run("ListResources", func() {
		body := `{"jsonrpc":"2.0","id":3,"method":"resources/list","params":{}}`

		resp := suite.makeMCPRequestWithToken(body, token)

		suite.NotEqual(401, resp.StatusCode, "ListResources should not return 401 with valid token")
	})

	suite.Run("ListPrompts", func() {
		body := `{"jsonrpc":"2.0","id":4,"method":"prompts/list","params":{}}`

		resp := suite.makeMCPRequestWithToken(body, token)

		suite.NotEqual(401, resp.StatusCode, "ListPrompts should not return 401 with valid token")
	})
}

// TestMCPEndpointTokenExpiration tests that expired tokens are rejected.
// It verifies token expiration and signature validation.
func (suite *MCPTestSuite) TestMCPEndpointTokenExpiration() {
	suite.T().Log("Testing MCP endpoint token expiration handling")

	// Test 1: Expired or malformed token
	suite.Run("ExpiredToken", func() {
		// Create a JWT with expired timestamp (exp: 1 = 1970-01-01)
		expiredToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyMDAxIiwiZXhwIjoxfQ.invalid"

		body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`

		resp := suite.makeMCPRequestWithToken(body, expiredToken)

		suite.Equal(401, resp.StatusCode, "Should return 401 with expired/invalid token")
		suite.T().Log("Expired token correctly rejected with 401")
	})
}

// TestMCPEndpointAuthorizationHeader tests different Authorization header formats.
func (suite *MCPTestSuite) TestMCPEndpointAuthorizationHeader() {
	suite.T().Log("Testing MCP endpoint Authorization header formats")

	token := suite.GenerateToken(suite.testUser)
	suite.NotEmpty(token, "Should get valid access token")

	suite.Run("BearerPrefix", func() {
		body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`

		req := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/mcp", strings.NewReader(body))
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)

		resp, err := suite.App.Test(req, 30*time.Second)
		suite.Require().NoError(err, "Should not return error")

		suite.NotEqual(401, resp.StatusCode, "Should accept Bearer prefix")
	})

	suite.Run("LowerCaseBearer", func() {
		body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`

		req := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/mcp", strings.NewReader(body))
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set(fiber.HeaderAuthorization, "bearer "+token)

		resp, err := suite.App.Test(req, 30*time.Second)
		suite.Require().NoError(err, "Should not return error")

		// SDK accepts lowercase bearer
		suite.NotEqual(401, resp.StatusCode, "Should accept lowercase bearer prefix")
	})

	suite.Run("NoPrefix", func() {
		body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`

		req := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/mcp", strings.NewReader(body))
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set(fiber.HeaderAuthorization, token) // No "Bearer " prefix

		resp, err := suite.App.Test(req, 30*time.Second)
		suite.Require().NoError(err, "Should not return error")

		suite.Equal(401, resp.StatusCode, "Should reject token without Bearer prefix")
	})
}

// TestMCPTestSuite tests MCP test suite functionality.
func TestMCP(t *testing.T) {
	suite.Run(t, new(MCPTestSuite))
}
