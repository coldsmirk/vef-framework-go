package app_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go"
	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

// APIKeyStrategy is a custom auth strategy registered through
// vef.ProvideAuthStrategy; it authenticates a static header credential.
type APIKeyStrategy struct{}

func NewAPIKeyStrategy() api.AuthStrategy { return &APIKeyStrategy{} }

func (*APIKeyStrategy) Name() string { return "api-key" }

func (*APIKeyStrategy) Authenticate(ctx fiber.Ctx, _ map[string]any) (*security.Principal, error) {
	if ctx.Get("X-API-Key") != "test-secret" {
		return nil, fiber.ErrUnauthorized
	}

	return security.NewExternalApp("keyed-app", "Keyed App"), nil
}

// PingResource guards a single ping operation behind the given auth config.
type PingResource struct {
	api.Resource
}

func NewPingResource(name string, auth *api.AuthConfig) api.Resource {
	return &PingResource{
		Resource: api.NewRPCResource(
			name,
			api.WithAuth(auth),
			api.WithOperations(api.OperationSpec{Action: "ping"}),
		),
	}
}

func (*PingResource) Ping(ctx fiber.Ctx) error {
	return result.Ok("pong").Response(ctx)
}

// AuthStrategySuite exercises auth-strategy wiring end to end: a custom
// strategy provided from the application scope must reach the registry inside
// the fx.Private auth module, and the built-in "ip" strategy must authenticate
// against the configured whitelists. Under app.Test the client IP resolves to
// "0.0.0.0".
type AuthStrategySuite struct {
	suite.Suite

	app  *app.App
	stop func()
}

func (s *AuthStrategySuite) SetupSuite() {
	s.app, s.stop = apptest.NewTestApp(
		s.T(),
		fx.Decorate(func(cfg *config.SecurityConfig) *config.SecurityConfig {
			cfg.IPWhitelists = map[string][]string{
				"internal": {"0.0.0.0"},
				"blocked":  {"10.9.9.9"},
			}

			return cfg
		}),
		vef.ProvideAuthStrategy(NewAPIKeyStrategy),
		vef.ProvideAPIResource(func() api.Resource {
			return NewPingResource("authtest/key", &api.AuthConfig{Strategy: "api-key"})
		}),
		vef.ProvideAPIResource(func() api.Resource {
			return NewPingResource("authtest/ip_allowed", api.IPAuth("internal"))
		}),
		vef.ProvideAPIResource(func() api.Resource {
			return NewPingResource("authtest/ip_denied", api.IPAuth("blocked"))
		}),
	)

	s.Require().NotNil(s.app, "App should be initialized")
}

func (s *AuthStrategySuite) TearDownSuite() {
	if s.stop != nil {
		s.stop()
	}
}

// postPing sends the RPC ping for the given resource with optional headers and
// returns the response.
func (s *AuthStrategySuite) postPing(resource string, headers map[string]string) *http.Response {
	s.T().Helper()

	req := httptest.NewRequestWithContext(
		context.Background(),
		fiber.MethodPost,
		"/api",
		strings.NewReader(fmt.Sprintf(`{"resource": %q, "action": "ping", "version": "v1"}`, resource)),
	)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := s.app.Test(req, 2*time.Second)
	s.Require().NoError(err, "API request should not fail")
	s.Require().NotNil(resp, "Response should not be nil")

	return resp
}

// TestCustomStrategy verifies strategies registered via vef.ProvideAuthStrategy.
func (s *AuthStrategySuite) TestCustomStrategy() {
	s.Run("AllowsValidCredential", func() {
		resp := s.postPing("authtest/key", map[string]string{"X-API-Key": "test-secret"})
		s.Equal(http.StatusOK, resp.StatusCode, "A valid API key should authenticate through the custom strategy")

		body, err := io.ReadAll(resp.Body)
		s.Require().NoError(err, "Response body should be readable")
		s.Contains(string(body), "pong", "The guarded handler should run")
	})

	s.Run("RejectsInvalidCredential", func() {
		resp := s.postPing("authtest/key", map[string]string{"X-API-Key": "wrong"})
		s.Equal(http.StatusUnauthorized, resp.StatusCode, "An invalid API key should be rejected by the custom strategy")
	})

	s.Run("RejectsMissingCredential", func() {
		resp := s.postPing("authtest/key", nil)
		s.Equal(http.StatusUnauthorized, resp.StatusCode, "A missing API key should be rejected by the custom strategy")
	})
}

// TestIPStrategy verifies the built-in "ip" strategy end to end.
func (s *AuthStrategySuite) TestIPStrategy() {
	s.Run("AllowsWhitelistedIP", func() {
		resp := s.postPing("authtest/ip_allowed", nil)
		s.Equal(http.StatusOK, resp.StatusCode, "A whitelisted client IP should authenticate")

		body, err := io.ReadAll(resp.Body)
		s.Require().NoError(err, "Response body should be readable")
		s.Contains(string(body), "pong", "The guarded handler should run")
	})

	s.Run("DeniesUnlistedIP", func() {
		resp := s.postPing("authtest/ip_denied", nil)
		s.Equal(http.StatusUnauthorized, resp.StatusCode, "A client IP outside the whitelist must be denied")
	})
}

// TestAuthStrategy runs the suite.
func TestAuthStrategy(t *testing.T) {
	suite.Run(t, new(AuthStrategySuite))
}
