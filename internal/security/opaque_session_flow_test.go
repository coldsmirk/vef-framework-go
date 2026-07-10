package security_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/password"
	"github.com/coldsmirk/vef-framework-go/security"
)

// OpaqueSessionFlowTestSuite drives the opaque-token mechanism end to end: a
// password login issues an opaque token, that token authenticates a protected
// call, and logout revokes it so it can no longer authenticate.
type OpaqueSessionFlowTestSuite struct {
	apptest.Suite

	userLoader *MockUserLoader
	publisher  *MockPublisher
	testUser   *security.Principal
}

func (s *OpaqueSessionFlowTestSuite) SetupSuite() {
	s.testUser = security.NewUser("user001", "Test User", "admin")
	s.userLoader = new(MockUserLoader)
	s.publisher = new(MockPublisher)

	hashedPassword, err := password.NewBcryptEncoder().Encode("password123")
	s.Require().NoError(err, "Opaque flow test user password should hash successfully")

	s.SetupApp(
		fx.Supply(
			fx.Annotate(
				s.userLoader,
				fx.As(new(security.UserLoader)),
			),
		),
		fx.Replace(
			fx.Annotate(
				s.publisher,
				fx.As(new(event.Bus)),
			),
		),
		fx.Replace(
			&config.SecurityConfig{
				Secret:           testJWTSecret,
				TokenExpires:     24 * time.Hour,
				RefreshNotBefore: time.Millisecond,
				LoginRateLimit:   1000,
				RefreshRateLimit: 1000,
				TokenType:        config.TokenTypeOpaque,
			},
		),
		fx.Invoke(func() {
			s.userLoader.On("LoadByUsername", mock.Anything, "testuser").
				Return(s.testUser, hashedPassword, nil).Maybe()
			s.userLoader.On("LoadByID", mock.Anything, "user001").
				Return(s.testUser, nil).Maybe()
			s.publisher.On("Publish", mock.Anything).Maybe()
		}),
	)
}

func (s *OpaqueSessionFlowTestSuite) TearDownSuite() {
	s.TearDownApp()
}

func (*OpaqueSessionFlowTestSuite) loginRequest() api.Request {
	return api.Request{
		Identifier: api.Identifier{Resource: "security/auth", Action: "login", Version: "v1"},
		Params: map[string]any{
			"type":        "password",
			"principal":   "testuser",
			"credentials": "password123",
		},
	}
}

func (*OpaqueSessionFlowTestSuite) logoutRequest() api.Request {
	return api.Request{
		Identifier: api.Identifier{Resource: "security/auth", Action: "logout", Version: "v1"},
	}
}

func (s *OpaqueSessionFlowTestSuite) TestLoginAuthenticateLogout() {
	resp := s.MakeRPCRequest(s.loginRequest())
	s.Require().Equal(200, resp.StatusCode, "opaque login should return HTTP 200")

	body := s.ReadResult(resp)
	s.Require().True(body.IsOk(), "opaque login should be ok")

	data := s.ReadDataAsMap(body.Data)
	tokens, ok := data["tokens"].(map[string]any)
	s.Require().True(ok, "login should return tokens")

	accessToken, ok := tokens["accessToken"].(string)
	s.Require().True(ok, "login should return an access token")
	s.NotEmpty(accessToken, "the opaque access token should be non-empty")
	s.Empty(tokens["refreshToken"], "an opaque login issues no refresh token")

	// The opaque token authenticates a protected endpoint.
	logoutResp := s.MakeRPCRequestWithToken(s.logoutRequest(), accessToken)
	s.Equal(200, logoutResp.StatusCode, "a valid opaque token should authenticate the logout call")

	// Logout revoked the session, so the same token no longer authenticates.
	revokedResp := s.MakeRPCRequestWithToken(s.logoutRequest(), accessToken)
	s.Equal(401, revokedResp.StatusCode, "a revoked opaque token must no longer authenticate")
}

// TestRefreshUnavailable proves the refresh operation is not mounted under the
// opaque mechanism: sessions renew themselves on use, and a leftover refresh
// JWT from a previous jwt_token deployment must not mint opaque sessions.
func (s *OpaqueSessionFlowTestSuite) TestRefreshUnavailable() {
	resp := s.MakeRPCRequest(api.Request{
		Identifier: api.Identifier{Resource: "security/auth", Action: "refresh", Version: "v1"},
		Params:     map[string]any{"refreshToken": "any-value"},
	})
	s.Equal(404, resp.StatusCode, "the refresh operation must not exist under the opaque mechanism")
}

func TestOpaqueSessionFlow(t *testing.T) {
	suite.Run(t, new(OpaqueSessionFlowTestSuite))
}
