package security_test

import (
	"context"
	"encoding/json"
	"net/http"
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
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	"github.com/coldsmirk/vef-framework-go/password"
	"github.com/coldsmirk/vef-framework-go/security"
)

// User agents of equal length, so the second request's header lands exactly on
// the first one's bytes when the pooled request buffer is reused. Equal length
// is what makes the aliasing observable rather than a matter of luck.
const (
	openingUserAgent = "Mozilla/5.0 (SessionMetaOpeningRequestAAAAAAAAAAAAAAAAAAAAAAAAA)"
	laterUserAgent   = "Mozilla/5.0 (SessionMetaLaterRequestBBBBBBBBBBBBBBBBBBBBBBBBBBB)"
)

// SessionMetaTestSuite pins that the client context recorded on a session is
// copied out of the request that opened it. Fiber runs with Immutable off, so
// the User-Agent is a view into the pooled request buffer, while an opaque
// session outlives its request by up to its whole maximum lifetime — an
// uncopied value silently becomes whatever later traffic wrote there.
type SessionMetaTestSuite struct {
	apptest.Suite

	userLoader *MockUserLoader
	publisher  *MockPublisher
	sessions   security.SessionStore
	testUser   *security.Principal
}

func (s *SessionMetaTestSuite) SetupSuite() {
	s.testUser = security.NewUser("user001", "Test User", "admin")
	s.userLoader = new(MockUserLoader)
	s.publisher = new(MockPublisher)

	hashedPassword, err := password.NewBcryptEncoder().Encode("password123")
	s.Require().NoError(err, "The test user's password should hash successfully")

	s.SetupApp(
		fx.Supply(fx.Annotate(s.userLoader, fx.As(new(security.UserLoader)))),
		fx.Replace(
			fx.Annotate(s.publisher, fx.As(new(event.Bus))),
			&config.SecurityConfig{
				Secret:         testJWTSecret,
				TokenExpires:   24 * time.Hour,
				LoginRateLimit: 1000,
				TokenType:      config.TokenTypeOpaque,
			},
		),
		fx.Populate(&s.sessions),
		fx.Invoke(func() {
			s.userLoader.On("LoadByUsername", mock.Anything, "testuser").
				Return(s.testUser, hashedPassword, nil).Maybe()
			s.userLoader.On("LoadByID", mock.Anything, "user001").
				Return(s.testUser, nil).Maybe()
			s.publisher.On("Publish", mock.Anything).Maybe()
		}),
	)
}

// SetupTest clears the sessions the previous method opened so each one counts
// only its own logins.
func (s *SessionMetaTestSuite) SetupTest() {
	s.Require().NoError(s.sessions.RevokeUser(context.Background(), "user001"),
		"Clearing the test user's sessions should succeed")
}

func (s *SessionMetaTestSuite) TearDownSuite() {
	s.TearDownApp()
}

// login posts a password login carrying the given User-Agent.
func (s *SessionMetaTestSuite) login(userAgent string) *http.Response {
	s.T().Helper()

	body, err := json.Marshal(api.Request{
		Resource: "security/auth",
		Action:   "login",
		Version:  "v1",
		Params: map[string]any{
			"type":        "password",
			"principal":   "testuser",
			"credentials": "password123",
		},
	})
	s.Require().NoError(err, "Marshaling the login request should succeed")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api", strings.NewReader(string(body)))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderUserAgent, userAgent)

	resp, err := s.App.Test(req, 30*time.Second)
	s.Require().NoError(err, "The login request should complete")

	return resp
}

// TestRecordedUserAgentSurvivesLaterTraffic drives two logins whose User-Agents
// occupy the same bytes and then reads the first session back. Without the copy
// in sessionMeta the first session reports the second request's User-Agent.
func (s *SessionMetaTestSuite) TestRecordedUserAgentSurvivesLaterTraffic() {
	s.Require().Equal(http.StatusOK, s.login(openingUserAgent).StatusCode, "The first login should succeed")
	s.Require().Equal(http.StatusOK, s.login(laterUserAgent).StatusCode, "The second login should succeed")

	sessions, err := s.sessions.ListByUser(context.Background(), "user001")
	s.Require().NoError(err, "Listing the user's sessions should succeed")
	s.Require().Len(sessions, 2, "Both logins should have opened a session")

	agents := make([]string, 0, len(sessions))
	for _, session := range sessions {
		agents = append(agents, session.UserAgent)
	}

	s.Contains(agents, openingUserAgent,
		"The first session must still report the User-Agent of the request that opened it")
	s.Contains(agents, laterUserAgent,
		"The second session must report its own User-Agent")
}

// TestAuditedUserAgentSurvivesLaterTraffic covers the same defect on the audit
// path, where it bites harder: the login event is published asynchronously, so
// a subscriber that persists it reads the fields well after the request that
// raised them has returned its buffer to the pool.
func (s *SessionMetaTestSuite) TestAuditedUserAgentSurvivesLaterTraffic() {
	before := len(s.publisher.GetPublishedEvents())

	s.Require().Equal(http.StatusOK, s.login(openingUserAgent).StatusCode, "The first login should succeed")
	s.Require().Equal(http.StatusOK, s.login(laterUserAgent).StatusCode, "The second login should succeed")

	agents := make([]string, 0, 2)

	for _, evt := range s.publisher.GetPublishedEvents()[before:] {
		if loginEvent, ok := evt.(*security.LoginEvent); ok {
			agents = append(agents, loginEvent.UserAgent)
		}
	}

	s.Require().Len(agents, 2, "Both logins should have published an audit event")
	s.Contains(agents, openingUserAgent,
		"The first login event must still report the User-Agent of the request that raised it")
	s.Contains(agents, laterUserAgent, "The second login event must report its own User-Agent")
}

func TestSessionMeta(t *testing.T) {
	suite.Run(t, new(SessionMetaTestSuite))
}
