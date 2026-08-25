package security_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/event"
	"github.com/coldsmirk/vef-framework-go/internal/apptest"
	isecurity "github.com/coldsmirk/vef-framework-go/internal/security"
	"github.com/coldsmirk/vef-framework-go/security"
)

// MockExternalAppLoader is a mock implementation of security.ExternalAppLoader.
type MockExternalAppLoader struct {
	mock.Mock
}

func (m *MockExternalAppLoader) LoadByID(ctx context.Context, id string) (*security.Principal, string, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.String(1), args.Error(2)
	}

	return args.Get(0).(*security.Principal), args.String(1), args.Error(2)
}

// MockTrustUserResolver is a mock implementation of security.TrustUserResolver.
type MockTrustUserResolver struct {
	mock.Mock
}

func (m *MockTrustUserResolver) ResolveUser(ctx context.Context, appID, externalUserID string) (*security.Principal, error) {
	args := m.Called(ctx, appID, externalUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*security.Principal), args.Error(1)
}

const (
	trustGatewayPath = "/sso/trust"
	trustAppID       = "his"
	trustAppSecret   = "5f4dcc3b5aa765d61d8327deb882cf995f4dcc3b5aa765d61d8327deb882cf99"
	trustExternalID  = "5756"
	// trustVictimID is a second user this system knows. Tamper tests swap it in
	// so a handoff whose signature failed to cover the identifier would succeed
	// rather than merely fail for a different reason.
	trustVictimID = "6001"
	trustRedirect = "http://192.168.10.197:8101/approval/todo"
)

// TrustLoginMiddlewareTestSuite drives the trust-login gateway through the real
// application: a signed handoff redirects with a code, that code redeems into a
// session through the ordinary login endpoint, and every tampered or unverified
// handoff is refused.
type TrustLoginMiddlewareTestSuite struct {
	apptest.Suite

	apps      *MockExternalAppLoader
	users     *MockUserLoader
	publisher *MockPublisher
	appUser   *security.Principal
	endUser   *security.Principal
	victim    *security.Principal
}

func (s *TrustLoginMiddlewareTestSuite) SetupSuite() {
	s.appUser = security.NewExternalApp(trustAppID, "HIS")
	s.endUser = security.NewUser(trustExternalID, "Test User", "admin")
	s.victim = security.NewUser(trustVictimID, "Victim User", "admin")
	s.apps = new(MockExternalAppLoader)
	s.users = new(MockUserLoader)
	s.publisher = new(MockPublisher)

	s.SetupApp(
		fx.Supply(
			fx.Annotate(s.apps, fx.As(new(security.ExternalAppLoader))),
			fx.Annotate(s.users, fx.As(new(security.UserLoader))),
		),
		fx.Replace(
			fx.Annotate(s.publisher, fx.As(new(event.Bus))),
			&config.SecurityConfig{
				Secret:           testJWTSecret,
				TokenExpires:     24 * time.Hour,
				RefreshNotBefore: time.Millisecond,
				LoginRateLimit:   1000,
				RefreshRateLimit: 1000,
				TrustLogin: config.TrustLoginConfig{
					Enabled: true,
					Apps: map[string]config.TrustLoginAppConfig{
						trustAppID: {RedirectURLs: []string{"http://192.168.10.197:8101/approval"}},
					},
				},
			},
		),
		fx.Invoke(func() {
			s.apps.On("LoadByID", mock.Anything, trustAppID).
				Return(s.appUser, trustAppSecret, nil).Maybe()
			s.apps.On("LoadByID", mock.Anything, mock.Anything).
				Return(nil, "", nil).Maybe()
			s.users.On("LoadByID", mock.Anything, trustExternalID).
				Return(s.endUser, nil).Maybe()
			s.users.On("LoadByID", mock.Anything, trustVictimID).
				Return(s.victim, nil).Maybe()
			s.users.On("LoadByID", mock.Anything, mock.Anything).
				Return(nil, nil).Maybe()
			s.publisher.On("Publish", mock.Anything).Maybe()
		}),
	)
}

func (s *TrustLoginMiddlewareTestSuite) TearDownSuite() {
	s.TearDownApp()
}

// handoff builds a correctly signed handoff query for the given user and
// redirect target. Callers tamper with the returned values to build the
// negative cases, which is exactly what an attacker holding one valid link can
// do.
func (s *TrustLoginMiddlewareTestSuite) handoff(userID, redirect string) url.Values {
	s.T().Helper()

	signer, err := security.NewSignature(trustAppSecret, security.WithNonceStore(nil))
	s.Require().NoError(err, "Building the test signer should succeed")

	signed, err := signer.Sign(security.SignatureRequest{
		AppID:       trustAppID,
		Method:      http.MethodGet,
		Path:        trustGatewayPath,
		BoundParams: map[string]string{"user_id": userID, "redirect": redirect},
	})
	s.Require().NoError(err, "Signing the handoff should succeed")

	return url.Values{
		"app_id":    {trustAppID},
		"user_id":   {userID},
		"redirect":  {redirect},
		"timestamp": {strconv.FormatInt(signed.Timestamp, 10)},
		"nonce":     {signed.Nonce},
		"signature": {signed.Signature},
	}
}

// get issues the handoff request and returns the raw response.
func (s *TrustLoginMiddlewareTestSuite) get(query url.Values) *http.Response {
	return s.getAs(query, "")
}

// getAs issues the handoff request presenting the given User-Agent.
func (s *TrustLoginMiddlewareTestSuite) getAs(query url.Values, userAgent string) *http.Response {
	s.T().Helper()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, trustGatewayPath+"?"+query.Encode(), nil)
	if userAgent != "" {
		req.Header.Set(fiber.HeaderUserAgent, userAgent)
	}

	resp, err := s.App.Test(req, 30*time.Second)
	s.Require().NoError(err, "The gateway request should complete")

	return resp
}

// redeem exchanges a code through the ordinary login endpoint.
func (s *TrustLoginMiddlewareTestSuite) redeem(appID, code string) *http.Response {
	return s.MakeRPCRequest(api.Request{
		Resource: "security/auth",
		Action:   "login",
		Version:  "v1",
		Params: map[string]any{
			"type":        "trust_code",
			"principal":   appID,
			"credentials": code,
		},
	})
}

// redeemAs exchanges a code through the ordinary login endpoint, presenting the
// given User-Agent. It builds the request rather than going through the suite
// helper because the header is the thing under test: the suite's requests carry
// no User-Agent, so a binding check against them only ever compares "" to "".
func (s *TrustLoginMiddlewareTestSuite) redeemAs(appID, code, userAgent string) *http.Response {
	s.T().Helper()

	body, err := json.Marshal(api.Request{
		Resource: "security/auth",
		Action:   "login",
		Version:  "v1",
		Params: map[string]any{
			"type":        "trust_code",
			"principal":   appID,
			"credentials": code,
		},
	})
	s.Require().NoError(err, "Marshaling the redemption request should succeed")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api", strings.NewReader(string(body)))
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	req.Header.Set(fiber.HeaderUserAgent, userAgent)

	resp, err := s.App.Test(req, 30*time.Second)
	s.Require().NoError(err, "The redemption request should complete")

	return resp
}

// codeFrom asserts a successful redirect and returns the issued code.
func (s *TrustLoginMiddlewareTestSuite) codeFrom(resp *http.Response) string {
	s.T().Helper()

	s.Require().Equal(fiber.StatusFound, resp.StatusCode, "A verified handoff should redirect")

	location, err := url.Parse(resp.Header.Get(fiber.HeaderLocation))
	s.Require().NoError(err, "The Location header should be a URL")

	code := location.Query().Get("code")
	s.Require().NotEmpty(code, "The redirect should carry a code")

	return code
}

// TestHandoffRedirects covers the successful gateway leg.
func (s *TrustLoginMiddlewareTestSuite) TestHandoffRedirects() {
	resp := s.get(s.handoff(trustExternalID, trustRedirect))
	s.Require().Equal(fiber.StatusFound, resp.StatusCode, "A verified handoff should redirect")

	location, err := url.Parse(resp.Header.Get(fiber.HeaderLocation))
	s.Require().NoError(err, "The Location header should be a URL")

	s.Equal("192.168.10.197:8101", location.Host, "The redirect should land on the requested host")
	s.Equal("/approval/todo", location.Path, "The redirect should preserve the requested deep link")
	s.Equal(trustAppID, location.Query().Get("app_id"), "The redirect should name the app that earned the code")
	s.NotEmpty(location.Query().Get("code"), "The redirect should carry a code")
	s.NotContains(resp.Header.Get(fiber.HeaderLocation), "signature",
		"The signed parameters must not be forwarded to the browser")
}

// TestEndToEnd walks the whole flow: handoff, redirect, redemption, and a
// protected call with the resulting token.
func (s *TrustLoginMiddlewareTestSuite) TestEndToEnd() {
	code := s.codeFrom(s.get(s.handoff(trustExternalID, trustRedirect)))

	resp := s.redeem(trustAppID, code)
	s.Require().Equal(http.StatusOK, resp.StatusCode, "Redeeming a code should return HTTP 200")

	body := s.ReadResult(resp)
	s.Require().True(body.IsOk(), "Redeeming a code should log the user in")

	tokens, ok := s.ReadDataAsMap(body.Data)["tokens"].(map[string]any)
	s.Require().True(ok, "The redemption should issue tokens")

	accessToken, ok := tokens["accessToken"].(string)
	s.Require().True(ok, "The redemption should issue an access token")

	logout := s.MakeRPCRequestWithToken(api.Request{
		Resource: "security/auth",
		Action:   "logout",
		Version:  "v1",
	}, accessToken)
	s.Equal(http.StatusOK, logout.StatusCode, "The issued token should authenticate a protected call")
}

// TestBrowserBindingIsWired drives both legs with a real User-Agent, which is
// what makes the binding a live check rather than a comparison of "" to "".
// It spans the gateway (which records the redirected browser) and the API auth
// middleware (which must publish the redeeming request's User-Agent into the
// request context for the authenticator to read) — deleting either half leaves
// the unit tests of both green, so only an end-to-end pass pins the wiring.
func (s *TrustLoginMiddlewareTestSuite) TestBrowserBindingIsWired() {
	const (
		issuingBrowser = "Mozilla/5.0 (TrustLoginIssuingBrowser)"
		anotherBrowser = "Mozilla/5.0 (TrustLoginAnotherBrowser)"
	)

	s.Run("SameBrowserRedeems", func() {
		code := s.codeFrom(s.getAs(s.handoff(trustExternalID, trustRedirect), issuingBrowser))

		s.True(s.ReadResult(s.redeemAs(trustAppID, code, issuingBrowser)).IsOk(),
			"The browser the gateway redirected must be able to redeem its own code")
	})

	s.Run("DifferentBrowserRejected", func() {
		code := s.codeFrom(s.getAs(s.handoff(trustExternalID, trustRedirect), issuingBrowser))

		redeemed := s.ReadResult(s.redeemAs(trustAppID, code, anotherBrowser))
		s.False(redeemed.IsOk(),
			"A code lifted out of the URL and replayed from another browser must be refused")
		s.Equal(security.ErrCodeTrustCodeInvalid, redeemed.Code,
			"A browser mismatch must report the same verdict as an unknown code")
	})
}

// TestCodeIsSingleUse proves the redirect URL left in browser history is inert
// once redeemed.
func (s *TrustLoginMiddlewareTestSuite) TestCodeIsSingleUse() {
	code := s.codeFrom(s.get(s.handoff(trustExternalID, trustRedirect)))

	s.Require().True(s.ReadResult(s.redeem(trustAppID, code)).IsOk(), "The first redemption should succeed")

	replayed := s.ReadResult(s.redeem(trustAppID, code))
	s.False(replayed.IsOk(), "A redeemed code must not log anyone in a second time")
	s.Equal(security.ErrCodeTrustCodeInvalid, replayed.Code, "A replayed code should report the trust-code verdict")
}

// TestBogusCodesDoNotLockOutTheApp pins that the exchange stays outside the
// brute-force guard. The identity a trust_code login presents is the app ID, so
// counting its failures would put every user of that system in one lockout
// bucket — and the default max_failures is reached here in a dozen requests
// anybody can send. The code itself is unguessable, single-use and seconds-
// lived, so the guard has nothing to protect on this path anyway.
func (s *TrustLoginMiddlewareTestSuite) TestBogusCodesDoNotLockOutTheApp() {
	// Comfortably past config.LockoutConfig's default MaxFailures of 10.
	for range 15 {
		rejected := s.ReadResult(s.redeem(trustAppID, "not-a-real-code"))
		s.Require().False(rejected.IsOk(), "A bogus code must be refused")
		s.Require().Equal(security.ErrCodeTrustCodeInvalid, rejected.Code,
			"A bogus code must report the trust-code verdict, never a lockout")
	}

	code := s.codeFrom(s.get(s.handoff(trustExternalID, trustRedirect)))

	s.True(s.ReadResult(s.redeem(trustAppID, code)).IsOk(),
		"A real handoff must still be redeemable after a flood of bogus codes")
}

// TestTamperedHandoff is the regression lock for the vulnerability this design
// exists to close: every parameter a link carries is covered by the signature,
// so holding one valid link buys nothing beyond replaying it verbatim.
func (s *TrustLoginMiddlewareTestSuite) TestTamperedHandoff() {
	// The swapped identifier names a user this system really has, so an
	// unsigned user_id would sail through every later gate and redirect with a
	// code that logs the victim in. Only the signature can refuse it.
	s.Run("SwappedUserID", func() {
		query := s.handoff(trustExternalID, trustRedirect)
		query.Set("user_id", trustVictimID)

		resp := s.get(query)
		s.Equal(http.StatusUnauthorized, resp.StatusCode,
			"Rewriting user_id in a captured link must not log in another user")
	})

	s.Run("SwappedRedirect", func() {
		query := s.handoff(trustExternalID, trustRedirect)
		query.Set("redirect", "http://192.168.10.197:8101/approval/other")

		resp := s.get(query)
		s.Equal(http.StatusUnauthorized, resp.StatusCode,
			"Rewriting the redirect target in a captured link must fail verification")
	})

	s.Run("SwappedAppID", func() {
		query := s.handoff(trustExternalID, trustRedirect)
		query.Set("app_id", "other-app")

		resp := s.get(query)
		s.Equal(http.StatusUnauthorized, resp.StatusCode, "An app that is not configured should be refused")
	})

	s.Run("CorruptedSignature", func() {
		query := s.handoff(trustExternalID, trustRedirect)
		query.Set("signature", "00000000000000000000000000000000000000000000000000000000000000ff")

		resp := s.get(query)
		s.Equal(http.StatusUnauthorized, resp.StatusCode, "A wrong signature should be refused")
	})

	s.Run("MalformedTimestamp", func() {
		query := s.handoff(trustExternalID, trustRedirect)
		query.Set("timestamp", "not-a-number")

		resp := s.get(query)
		s.Equal(http.StatusUnauthorized, resp.StatusCode, "A non-numeric timestamp should be refused")
	})

	s.Run("ExpiredTimestamp", func() {
		query := s.handoff(trustExternalID, trustRedirect)
		query.Set("timestamp", strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10))

		resp := s.get(query)
		s.Equal(http.StatusUnauthorized, resp.StatusCode, "A stale link should be refused")
	})
}

// TestReplayedHandoff proves the nonce store closes the window in which a
// captured link could be navigated twice.
func (s *TrustLoginMiddlewareTestSuite) TestReplayedHandoff() {
	query := s.handoff(trustExternalID, trustRedirect)

	s.Require().Equal(fiber.StatusFound, s.get(query).StatusCode, "The first navigation should be verified")
	s.Equal(http.StatusUnauthorized, s.get(query).StatusCode,
		"Navigating the same signed link twice must be refused by nonce replay protection")
}

// TestRedirectAllowlist covers the open-redirect defense. The target is signed,
// so these are the cases where the external system itself asks for somewhere it
// is not entitled to send a user.
func (s *TrustLoginMiddlewareTestSuite) TestRedirectAllowlist() {
	s.Run("ForeignHost", func() {
		resp := s.get(s.handoff(trustExternalID, "http://evil.example.com/approval/todo"))
		s.Equal(http.StatusBadRequest, resp.StatusCode, "A target on another host should be refused")
	})

	s.Run("ForeignScheme", func() {
		resp := s.get(s.handoff(trustExternalID, "https://192.168.10.197:8101/approval/todo"))
		s.Equal(http.StatusBadRequest, resp.StatusCode, "A target on another scheme should be refused")
	})

	s.Run("UncoveredPath", func() {
		resp := s.get(s.handoff(trustExternalID, "http://192.168.10.197:8101/admin"))
		s.Equal(http.StatusBadRequest, resp.StatusCode, "A target outside the allowed path should be refused")
	})

	s.Run("PathPrefixIsNotSegmentPrefix", func() {
		resp := s.get(s.handoff(trustExternalID, "http://192.168.10.197:8101/approvalx"))
		s.Equal(http.StatusBadRequest, resp.StatusCode,
			"An entry of /approval must not cover /approvalx — matching is per path segment")
	})

	s.Run("DotSegmentsCannotEscape", func() {
		resp := s.get(s.handoff(trustExternalID, "http://192.168.10.197:8101/approval/../admin"))
		s.Equal(http.StatusBadRequest, resp.StatusCode,
			"A traversal that resolves outside the allowed path should be refused")
	})

	s.Run("RelativeTarget", func() {
		resp := s.get(s.handoff(trustExternalID, "/approval/todo"))
		s.Equal(http.StatusBadRequest, resp.StatusCode, "A target without scheme and host should be refused")
	})
}

// TestUnresolvableUser covers a signed handoff naming somebody this system does
// not know.
func (s *TrustLoginMiddlewareTestSuite) TestUnresolvableUser() {
	resp := s.get(s.handoff("no-such-user", trustRedirect))
	s.Equal(http.StatusUnauthorized, resp.StatusCode, "A handoff naming an unknown user should be refused")
}

func TestTrustLoginMiddleware(t *testing.T) {
	suite.Run(t, new(TrustLoginMiddlewareTestSuite))
}

// TestDisabledExternalApp proves an app switched off in the ExternalAppLoader
// loses browser single sign-on together with its API access.
func (s *TrustLoginMiddlewareTestSuite) TestDisabledExternalApp() {
	disabled := security.NewExternalApp("disabled-app", "Retired HIS")
	disabled.Details = &security.ExternalAppConfig{Enabled: false}

	s.apps.On("LoadByID", mock.Anything, "disabled-app").Return(disabled, trustAppSecret, nil).Maybe()

	signer, err := security.NewSignature(trustAppSecret, security.WithNonceStore(nil))
	s.Require().NoError(err, "Building the test signer should succeed")

	signed, err := signer.Sign(security.SignatureRequest{
		AppID:       "disabled-app",
		Method:      http.MethodGet,
		Path:        trustGatewayPath,
		BoundParams: map[string]string{"user_id": trustExternalID, "redirect": trustRedirect},
	})
	s.Require().NoError(err, "Signing the handoff should succeed")

	// The app is configured for trust login, so only the loader's disabled flag
	// can refuse it.
	resp := s.get(url.Values{
		"app_id":    {"disabled-app"},
		"user_id":   {trustExternalID},
		"redirect":  {trustRedirect},
		"timestamp": {strconv.FormatInt(signed.Timestamp, 10)},
		"nonce":     {signed.Nonce},
		"signature": {signed.Signature},
	})
	s.Equal(http.StatusUnauthorized, resp.StatusCode, "A disabled external app must not initiate a handoff")
}

// TrustLoginResolverTestSuite pins the precedence between the two ways an
// external identifier becomes a local principal.
type TrustLoginResolverTestSuite struct {
	apptest.Suite

	apps      *MockExternalAppLoader
	users     *MockUserLoader
	resolver  *MockTrustUserResolver
	publisher *MockPublisher
}

func (s *TrustLoginResolverTestSuite) SetupSuite() {
	s.apps = new(MockExternalAppLoader)
	s.users = new(MockUserLoader)
	s.resolver = new(MockTrustUserResolver)
	s.publisher = new(MockPublisher)

	s.SetupApp(
		fx.Supply(
			fx.Annotate(s.apps, fx.As(new(security.ExternalAppLoader))),
			fx.Annotate(s.users, fx.As(new(security.UserLoader))),
			fx.Annotate(s.resolver, fx.As(new(security.TrustUserResolver))),
		),
		fx.Replace(
			fx.Annotate(s.publisher, fx.As(new(event.Bus))),
			&config.SecurityConfig{
				Secret:         testJWTSecret,
				TokenExpires:   24 * time.Hour,
				LoginRateLimit: 1000,
				TrustLogin: config.TrustLoginConfig{
					Enabled: true,
					Apps: map[string]config.TrustLoginAppConfig{
						trustAppID: {RedirectURLs: []string{"http://192.168.10.197:8101/approval"}},
					},
				},
			},
		),
		fx.Invoke(func() {
			s.apps.On("LoadByID", mock.Anything, trustAppID).
				Return(security.NewExternalApp(trustAppID, "HIS"), trustAppSecret, nil).Maybe()

			// The resolver knows "mapped-only"; the loader knows "loader-only".
			// Whichever handoff succeeds names the source the gateway consulted.
			s.resolver.On("ResolveUser", mock.Anything, trustAppID, "mapped-only").
				Return(security.NewUser("user001", "Mapped User"), nil).Maybe()
			s.resolver.On("ResolveUser", mock.Anything, mock.Anything, mock.Anything).
				Return(nil, nil).Maybe()
			s.users.On("LoadByID", mock.Anything, "loader-only").
				Return(security.NewUser("user002", "Loader User"), nil).Maybe()
			s.users.On("LoadByID", mock.Anything, mock.Anything).
				Return(nil, nil).Maybe()
			s.publisher.On("Publish", mock.Anything).Maybe()
		}),
	)
}

func (s *TrustLoginResolverTestSuite) TearDownSuite() {
	s.TearDownApp()
}

func (s *TrustLoginResolverTestSuite) get(userID string) *http.Response {
	s.T().Helper()

	signer, err := security.NewSignature(trustAppSecret, security.WithNonceStore(nil))
	s.Require().NoError(err, "Building the test signer should succeed")

	signed, err := signer.Sign(security.SignatureRequest{
		AppID:       trustAppID,
		Method:      http.MethodGet,
		Path:        trustGatewayPath,
		BoundParams: map[string]string{"user_id": userID, "redirect": trustRedirect},
	})
	s.Require().NoError(err, "Signing the handoff should succeed")

	query := url.Values{
		"app_id":    {trustAppID},
		"user_id":   {userID},
		"redirect":  {trustRedirect},
		"timestamp": {strconv.FormatInt(signed.Timestamp, 10)},
		"nonce":     {signed.Nonce},
		"signature": {signed.Signature},
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, trustGatewayPath+"?"+query.Encode(), nil)

	resp, err := s.App.Test(req, 30*time.Second)
	s.Require().NoError(err, "The gateway request should complete")

	return resp
}

// TestResolverWins proves a registered TrustUserResolver replaces the
// UserLoader fallback outright rather than layering on top of it.
func (s *TrustLoginResolverTestSuite) TestResolverWins() {
	s.Equal(fiber.StatusFound, s.get("mapped-only").StatusCode,
		"An identifier only the resolver knows should resolve")

	s.Equal(http.StatusUnauthorized, s.get("loader-only").StatusCode,
		"With a resolver registered the UserLoader must not be consulted as a second chance")

	// Not redundant with the status assertions: it inspects the arguments the
	// resolver was handed *after* both requests are over, which is what pins
	// the copy out of the request buffer. Drop the gateway's strings.Clone and
	// this fails, reporting the first call's identifier as the second's.
	s.resolver.AssertCalled(s.T(), "ResolveUser", mock.Anything, trustAppID, "mapped-only")
}

func TestTrustLoginResolver(t *testing.T) {
	suite.Run(t, new(TrustLoginResolverTestSuite))
}

// TrustLoginDisabledTestSuite proves the feature leaves no surface behind when
// it is off: no route, and no mechanism the login endpoint will entertain.
type TrustLoginDisabledTestSuite struct {
	apptest.Suite

	publisher *MockPublisher
}

func (s *TrustLoginDisabledTestSuite) SetupSuite() {
	s.publisher = new(MockPublisher)

	s.SetupApp(
		fx.Replace(
			fx.Annotate(s.publisher, fx.As(new(event.Bus))),
			&config.SecurityConfig{Secret: testJWTSecret, TokenExpires: 24 * time.Hour, LoginRateLimit: 1000},
		),
		fx.Invoke(func() {
			s.publisher.On("Publish", mock.Anything).Maybe()
		}),
	)
}

func (s *TrustLoginDisabledTestSuite) TearDownSuite() {
	s.TearDownApp()
}

func (s *TrustLoginDisabledTestSuite) TestGatewayNotMounted() {
	resp := s.MakeRESTRequest(http.MethodGet, trustGatewayPath+"?app_id=his", "")
	s.Equal(http.StatusNotFound, resp.StatusCode, "The gateway route must not exist while trust login is disabled")
}

func (s *TrustLoginDisabledTestSuite) TestMechanismNotRegistered() {
	resp := s.MakeRPCRequest(api.Request{
		Resource: "security/auth",
		Action:   "login",
		Version:  "v1",
		Params: map[string]any{
			"type":        "trust_code",
			"principal":   trustAppID,
			"credentials": "any-code",
		},
	})

	body := s.ReadResult(resp)
	s.False(body.IsOk(), "A trust_code login must fail while the feature is disabled")
	s.Equal(security.ErrCodeUnsupportedAuthenticationType, body.Code,
		"The mechanism should be reported as unsupported rather than as an invalid code")
}

func TestTrustLoginDisabled(t *testing.T) {
	suite.Run(t, new(TrustLoginDisabledTestSuite))
}

// TestGatewayConstruction covers the boot-time contract: the gateway refuses to
// start without the collaborators it needs, rather than starting and answering
// 401 to every handoff.
func TestGatewayConstruction(t *testing.T) {
	enabled := &config.SecurityConfig{
		TrustLogin: config.TrustLoginConfig{
			Enabled: true,
			Apps:    map[string]config.TrustLoginAppConfig{trustAppID: {RedirectURLs: []string{"http://app.local/portal"}}},
		},
	}

	t.Run("DisabledYieldsNoMiddleware", func(t *testing.T) {
		middleware, err := isecurity.NewTrustLoginMiddleware(isecurity.TrustLoginMiddlewareParams{
			Codes:    security.NewMemoryTrustCodeStore(),
			Security: new(config.SecurityConfig),
		})

		require.NoError(t, err, "A disabled gateway should not fail construction")
		assert.Nil(t, middleware, "A disabled gateway should mount nothing")
	})

	t.Run("RequiresExternalAppLoader", func(t *testing.T) {
		_, err := isecurity.NewTrustLoginMiddleware(isecurity.TrustLoginMiddlewareParams{
			Users:    new(MockUserLoader),
			Codes:    security.NewMemoryTrustCodeStore(),
			Security: enabled,
		})

		require.ErrorIs(t, err, isecurity.ErrTrustLoginExternalAppLoaderMissing,
			"Without app secrets no handoff could ever be verified, so the boot should fail")
	})

	t.Run("RequiresUserResolution", func(t *testing.T) {
		_, err := isecurity.NewTrustLoginMiddleware(isecurity.TrustLoginMiddlewareParams{
			Apps:     new(MockExternalAppLoader),
			Codes:    security.NewMemoryTrustCodeStore(),
			Security: enabled,
		})

		require.ErrorIs(t, err, isecurity.ErrTrustLoginUserResolutionMissing,
			"Without any way to resolve external users the boot should fail")
	})

	t.Run("ResolverAloneSuffices", func(t *testing.T) {
		middleware, err := isecurity.NewTrustLoginMiddleware(isecurity.TrustLoginMiddlewareParams{
			Apps:     new(MockExternalAppLoader),
			Resolver: new(MockTrustUserResolver),
			Codes:    security.NewMemoryTrustCodeStore(),
			Security: enabled,
		})

		require.NoError(t, err, "A resolver alone should satisfy user resolution")
		assert.NotNil(t, middleware, "An enabled gateway should mount")
	})
}
