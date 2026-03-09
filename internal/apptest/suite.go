package apptest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/suite"
	"github.com/uptrace/bun"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

// testJWTAudience matches lo.SnakeCase of the test app name "test-app".
const testJWTAudience = "test_app"

// Suite provides common integration test infrastructure for suites
// that boot a full FX App and make HTTP requests against it.
// Embed this struct instead of suite.Suite to get app lifecycle management,
// RPC/REST request helpers, response parsing, and JWT token generation.
type Suite struct {
	suite.Suite

	// App is the test application instance, available after SetupApp/SetupAppWithDB.
	App  *app.App
	stop func()
}

// --- App lifecycle ---

// SetupApp creates a test app with the given FX options.
func (s *Suite) SetupApp(opts ...fx.Option) {
	s.App, s.stop = NewTestApp(s.T(), opts...)
}

// SetupAppWithDB creates a test app using an existing *bun.DB
// instead of creating a new database connection.
func (s *Suite) SetupAppWithDB(db *bun.DB, opts ...fx.Option) {
	s.App, s.stop = NewTestAppWithDB(s.T(), db, opts...)
}

// TearDownApp stops the app gracefully.
func (s *Suite) TearDownApp() {
	if s.stop != nil {
		s.stop()
	}
}

// --- RPC requests (POST /api) ---

// MakeRPCRequest sends an RPC API request (POST /api with JSON body)
// and returns the raw HTTP response.
func (s *Suite) MakeRPCRequest(body api.Request) *http.Response {
	jsonBytes, err := json.Marshal(body)
	s.Require().NoError(err)

	req := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/api", strings.NewReader(string(jsonBytes)))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	resp, err := s.App.Test(req)
	s.Require().NoError(err)

	return resp
}

// MakeRPCRequestWithToken sends an RPC API request with a Bearer authorization header.
func (s *Suite) MakeRPCRequestWithToken(body api.Request, token string) *http.Response {
	jsonBytes, err := json.Marshal(body)
	s.Require().NoError(err)

	req := httptest.NewRequestWithContext(context.Background(), fiber.MethodPost, "/api", strings.NewReader(string(jsonBytes)))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderAuthorization, security.AuthSchemeBearer+" "+token)

	resp, err := s.App.Test(req)
	s.Require().NoError(err)

	return resp
}

// --- REST requests (any method, any path) ---

// MakeRESTRequest sends a REST API request with the given HTTP method, path, and optional JSON body.
func (s *Suite) MakeRESTRequest(method, path, body string) *http.Response {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	} else {
		req = httptest.NewRequestWithContext(context.Background(), method, path, nil)
	}

	resp, err := s.App.Test(req)
	s.Require().NoError(err)

	return resp
}

// MakeRESTRequestWithToken sends a REST API request with a Bearer authorization header.
func (s *Suite) MakeRESTRequestWithToken(method, path, body, token string) *http.Response {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	} else {
		req = httptest.NewRequestWithContext(context.Background(), method, path, nil)
	}

	req.Header.Set(fiber.HeaderAuthorization, security.AuthSchemeBearer+" "+token)

	resp, err := s.App.Test(req)
	s.Require().NoError(err)

	return resp
}

// --- Response helpers ---

// ReadResult reads and decodes the HTTP response body as result.Result.
// The response body is closed after reading.
func (s *Suite) ReadResult(resp *http.Response) result.Result {
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var res result.Result

	err = json.Unmarshal(body, &res)
	s.Require().NoError(err)

	return res
}

// ReadDataAsMap asserts that data is a map[string]any and returns it.
func (s *Suite) ReadDataAsMap(data any) map[string]any {
	m, ok := data.(map[string]any)
	s.Require().True(ok, "Expected data to be a map")

	return m
}

// ReadDataAsSlice asserts that data is a []any and returns it.
func (s *Suite) ReadDataAsSlice(data any) []any {
	slice, ok := data.([]any)
	s.Require().True(ok, "Expected data to be a slice")

	return slice
}

// --- Auth helpers ---

// GenerateToken creates a valid JWT access token for the given principal.
// Uses security.DefaultJWTSecret and the test app audience ("test_app").
// The token is valid for 1 hour with no notBefore delay.
//
// Tests using this method must configure the same JWT secret in their FX app:
//
//	fx.Replace(&security.JWTConfig{Secret: security.DefaultJWTSecret, Audience: "test_app"})
func (s *Suite) GenerateToken(principal *security.Principal) string {
	jwtCfg := &security.JWTConfig{
		Secret:   security.DefaultJWTSecret,
		Audience: testJWTAudience,
	}

	jwtInstance, err := security.NewJWT(jwtCfg)
	s.Require().NoError(err)

	claims := security.NewJWTClaimsBuilder().
		WithSubject(principal.ID + "@" + principal.Name).
		WithRoles(principal.Roles).
		WithType(security.TokenTypeAccess)

	token, err := jwtInstance.Generate(claims, 1*time.Hour, 0)
	s.Require().NoError(err)

	return token
}
