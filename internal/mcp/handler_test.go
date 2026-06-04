package mcp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/security"
)

// MCPAuthModeTestSuite covers how config.MCPConfig.RequireAuth gates the /mcp
// endpoint: each test boots its own app so it can vary the pointer value
// (unset/nil vs explicit false), which a single shared app cannot.
type MCPAuthModeTestSuite struct {
	apptest.Suite

	userLoader *MockUserLoader
}

func TestMCPAuthMode(t *testing.T) {
	suite.Run(t, new(MCPAuthModeTestSuite))
}

// bootWithRequireAuth starts an MCP-enabled app with the given RequireAuth
// pointer. The caller is responsible for TearDownApp.
func (s *MCPAuthModeTestSuite) bootWithRequireAuth(requireAuth *bool) {
	s.userLoader = new(MockUserLoader)

	s.SetupApp(
		fx.Supply(
			fx.Annotate(s.userLoader, fx.As(new(security.UserLoader))),
		),
		fx.Replace(
			&config.MCPConfig{Enabled: true, RequireAuth: requireAuth},
			&config.SecurityConfig{
				Secret:       security.DefaultJWTSecret,
				TokenExpires: 24 * time.Hour,
			},
		),
	)
}

func (s *MCPAuthModeTestSuite) postMCPWithoutToken() *http.Response {
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`

	req := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := s.App.Test(req, 30*time.Second)
	s.Require().NoError(err, "MCP request should not fail")

	return resp
}

// TestUnsetRequireAuthDefaultsToSecure verifies the secure default: when
// require_auth is absent (nil), an unauthenticated request is rejected.
func (s *MCPAuthModeTestSuite) TestUnsetRequireAuthDefaultsToSecure() {
	s.bootWithRequireAuth(nil)
	defer s.TearDownApp()

	resp := s.postMCPWithoutToken()
	s.Equal(http.StatusUnauthorized, resp.StatusCode, "an unset require_auth must default to auth-required")
}

// TestExplicitFalseAllowsAnonymous verifies the opt-in: require_auth=false
// lets an unauthenticated request through. The pointer is built with a local
// variable, not ptr.Of(false), which returns nil for the zero value and would
// silently exercise the nil-default branch instead.
func (s *MCPAuthModeTestSuite) TestExplicitFalseAllowsAnonymous() {
	anonymous := false

	s.bootWithRequireAuth(&anonymous)
	defer s.TearDownApp()

	resp := s.postMCPWithoutToken()
	s.NotEqual(http.StatusUnauthorized, resp.StatusCode, "require_auth=false must allow anonymous access")
}
