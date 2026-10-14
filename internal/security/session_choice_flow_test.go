package security_test

import (
	"context"
	"maps"
	"slices"
	"sync"
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
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

// departmentDirectory is a host's department catalog for one user: the login
// offers every department the user holds, and the user record alone yields the
// default one, as a host's UserLoader.LoadByID does.
type departmentDirectory struct {
	mu       sync.Mutex
	held     []string
	password string
}

func (*departmentDirectory) principal(departmentID string) *security.Principal {
	principal := security.NewUser("user1", "Alice", "staff")
	principal.Details = map[string]any{"departmentId": departmentID}

	return principal
}

func (d *departmentDirectory) LoadByUsername(_ context.Context, username string) (*security.Principal, string, error) {
	if username != "alice" {
		return nil, "", result.ErrRecordNotFound
	}

	return d.principal("dept-default"), d.password, nil
}

func (d *departmentDirectory) LoadByID(_ context.Context, id string) (*security.Principal, error) {
	if id != "user1" {
		return nil, result.ErrRecordNotFound
	}

	return d.principal("dept-default"), nil
}

func (d *departmentDirectory) LoadDepartments(context.Context, *security.LoginContext) (*security.DepartmentSelectionChallengeData, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	options := make([]security.DepartmentOption, 0, len(d.held))
	for _, id := range d.held {
		options = append(options, security.DepartmentOption{ID: id, Name: id})
	}

	return &security.DepartmentSelectionChallengeData{Departments: options}, nil
}

func (d *departmentDirectory) SelectDepartment(_ context.Context, principal *security.Principal, departmentID string) (*security.Principal, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !slices.Contains(d.held, departmentID) {
		return nil, result.Err("department not available")
	}

	selected := *principal
	selected.Details = map[string]any{"departmentId": departmentID}

	return &selected, nil
}

func (d *departmentDirectory) hold(departments ...string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.held = departments
}

// SessionChoiceFlowTestSuite pins the department chosen at login across JWT
// refreshes, over the real login → resolve_challenge → refresh endpoints. The
// refresh rebuilds the principal from LoadByID, which yields the default
// department; the session must keep the chosen one instead.
type SessionChoiceFlowTestSuite struct {
	apptest.Suite

	directory *departmentDirectory
	publisher *MockPublisher
	jwt       *security.JWT
}

func (s *SessionChoiceFlowTestSuite) SetupSuite() {
	hashed, err := password.NewBcryptEncoder().Encode("password123")
	s.Require().NoError(err, "The test password should hash")

	s.directory = &departmentDirectory{password: hashed}
	s.publisher = new(MockPublisher)
	s.publisher.On("Publish", mock.Anything).Maybe()

	s.SetupApp(
		fx.Supply(
			fx.Annotate(s.directory, fx.As(new(security.UserLoader))),
			fx.Annotate(
				security.NewDepartmentSelectionChallengeProvider(s.directory, s.directory),
				fx.As(new(security.ChallengeProvider)),
				fx.ResultTags(`group:"vef:security:challenge_providers"`),
			),
		),
		fx.Replace(
			fx.Annotate(s.publisher, fx.As(new(event.Bus))),
			&config.SecurityConfig{
				Secret:           testJWTSecret,
				TokenExpires:     24 * time.Hour,
				RefreshNotBefore: time.Millisecond,
				LoginRateLimit:   1000,
				RefreshRateLimit: 1000,
			},
		),
		fx.Populate(&s.jwt),
	)
}

func (s *SessionChoiceFlowTestSuite) TearDownSuite() {
	s.TearDownApp()
}

func (s *SessionChoiceFlowTestSuite) SetupTest() {
	s.directory.hold("dept-default", "dept-2")
}

// call sends one security/auth action and returns its result.
func (s *SessionChoiceFlowTestSuite) call(action string, params map[string]any) result.Result {
	resp := s.MakeRPCRequest(api.Request{Resource: "security/auth", Action: action, Version: "v1", Params: params})
	s.Require().Equal(200, resp.StatusCode, "%s should answer HTTP 200", action)

	return s.ReadResult(resp)
}

// loginChoosing logs alice in, choosing departmentID, and returns her tokens.
func (s *SessionChoiceFlowTestSuite) loginChoosing(departmentID string) map[string]any {
	login := s.call("login", map[string]any{
		"type":        security.AuthTypePassword,
		"principal":   "alice",
		"credentials": "password123",
	})
	s.Require().True(login.IsOk(), "Login should succeed: %s", login.Message)

	challenge := s.ReadDataAsMap(login.Data)
	s.Require().NotEmpty(challenge["challengeToken"], "A user holding two departments should be asked to choose")

	resolved := s.call("resolve_challenge", map[string]any{
		"challengeToken": challenge["challengeToken"],
		"type":           security.ChallengeTypeDepartmentSelection,
		"response":       departmentID,
	})
	s.Require().True(resolved.IsOk(), "Choosing a held department should succeed: %s", resolved.Message)

	return s.ReadDataAsMap(s.ReadDataAsMap(resolved.Data)["tokens"])
}

// refresh exchanges a refresh token for a new token pair.
func (s *SessionChoiceFlowTestSuite) refresh(tokens map[string]any) result.Result {
	return s.call("refresh", map[string]any{"refreshToken": tokens["refreshToken"]})
}

// departmentOf reads the department the access token's session acts in.
func (s *SessionChoiceFlowTestSuite) departmentOf(tokens map[string]any) any {
	claims, err := s.jwt.Parse(tokens["accessToken"].(string))
	s.Require().NoError(err, "The access token should parse")

	details, ok := claims.Details().(map[string]any)
	s.Require().True(ok, "The access token should carry the principal details")

	return details["departmentId"]
}

func (s *SessionChoiceFlowTestSuite) TestLoginActsInTheChosenDepartment() {
	tokens := s.loginChoosing("dept-2")

	s.Equal("dept-2", s.departmentOf(tokens), "The login should act in the department chosen")
}

func (s *SessionChoiceFlowTestSuite) TestRefreshKeepsTheChosenDepartment() {
	tokens := s.loginChoosing("dept-2")

	for i := range 2 {
		refreshed := s.refresh(tokens)
		s.Require().True(refreshed.IsOk(), "Refresh %d should succeed: %s", i+1, refreshed.Message)

		tokens = s.ReadDataAsMap(refreshed.Data)
		s.Equal("dept-2", s.departmentOf(tokens),
			"Refresh %d should keep the department chosen at login, not fall back to the default", i+1)
	}
}

func (s *SessionChoiceFlowTestSuite) TestRefreshFailsOnceTheDepartmentIsLost() {
	tokens := s.loginChoosing("dept-2")

	s.directory.hold("dept-default")

	refreshed := s.refresh(tokens)

	s.False(refreshed.IsOk(), "A session in a department the user no longer holds should not refresh")
	s.Equal("department not available", refreshed.Message, "The selector's refusal should reach the client")
}

func (s *SessionChoiceFlowTestSuite) TestRefreshTokenCarriesOnlyTheChoice() {
	tokens := s.loginChoosing("dept-2")

	claims, err := s.jwt.Parse(tokens["refreshToken"].(string))
	s.Require().NoError(err, "The refresh token should parse")

	s.True(maps.Equal(map[string]string{security.ChallengeTypeDepartmentSelection: "dept-2"}, claims.SessionChoices()),
		"The refresh token should carry the department chosen and nothing else of the answer")
	s.Nil(claims.Details(), "The refresh token should still carry no principal details")
}

func TestSessionChoiceFlow(t *testing.T) {
	suite.Run(t, new(SessionChoiceFlowTestSuite))
}
