package security

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/security"
)

const (
	trustTestAppID     = "his"
	trustTestUserAgent = "Mozilla/5.0 (Test)"
	trustTestClientIP  = "192.168.10.9"
)

// TrustCodeAuthenticatorTestSuite covers redeeming a trust-login code: the
// happy path, and every way the redemption must be refused.
type TrustCodeAuthenticatorTestSuite struct {
	suite.Suite

	store security.TrustCodeStore
	user  *security.Principal
}

func (s *TrustCodeAuthenticatorTestSuite) SetupTest() {
	s.store = security.NewMemoryTrustCodeStore()
	s.user = security.NewUser("user001", "Test User", "admin")
}

// issue parks a code for the standard test browser.
func (s *TrustCodeAuthenticatorTestSuite) issue() string {
	s.T().Helper()

	code, err := s.store.Issue(context.Background(), security.TrustCodeState{
		Principal: s.user,
		AppID:     trustTestAppID,
		UserAgent: trustTestUserAgent,
		ClientIP:  trustTestClientIP,
	}, time.Minute)
	s.Require().NoError(err, "Issuing a trust code should succeed")

	return code
}

// requestContext builds the request metadata the API auth middleware records
// for an exchange arriving from the given browser.
func (*TrustCodeAuthenticatorTestSuite) requestContext(userAgent, clientIP string) context.Context {
	ctx := contextx.SetRequestUserAgent(context.Background(), userAgent)

	return contextx.SetRequestIP(ctx, clientIP)
}

// authenticator builds the authenticator under the given binding policy.
func (s *TrustCodeAuthenticatorTestSuite) authenticator(cfg config.TrustLoginConfig) *TrustCodeAuthenticator {
	return NewTrustCodeAuthenticator(s.store, cfg)
}

func (s *TrustCodeAuthenticatorTestSuite) TestSupports() {
	auth := s.authenticator(config.TrustLoginConfig{})

	s.True(auth.Supports(AuthTypeTrustCode), "The authenticator should support its own mechanism")
	s.False(auth.Supports(AuthTypePassword), "The authenticator should not claim other mechanisms")
}

func (s *TrustCodeAuthenticatorTestSuite) TestAuthenticate() {
	s.Run("RedeemsCode", func() {
		auth := s.authenticator(config.TrustLoginConfig{})
		code := s.issue()

		principal, err := auth.Authenticate(
			s.requestContext(trustTestUserAgent, trustTestClientIP),
			security.Authentication{Type: AuthTypeTrustCode, Principal: trustTestAppID, Credentials: code},
		)

		s.Require().NoError(err, "A code redeemed by the browser it was issued to should authenticate")
		s.Require().NotNil(principal, "Redeeming should yield the resolved principal")
		s.Equal("user001", principal.ID, "The principal the gateway resolved should reach the login pipeline")
		s.Equal([]string{"admin"}, principal.Roles, "The principal's roles should reach the login pipeline")
	})

	s.Run("CodeIsSingleUse", func() {
		auth := s.authenticator(config.TrustLoginConfig{})
		code := s.issue()
		ctx := s.requestContext(trustTestUserAgent, trustTestClientIP)
		authentication := security.Authentication{Type: AuthTypeTrustCode, Principal: trustTestAppID, Credentials: code}

		_, err := auth.Authenticate(ctx, authentication)
		s.Require().NoError(err, "The first redemption should succeed")

		_, err = auth.Authenticate(ctx, authentication)
		s.ErrorIs(err, security.ErrTrustCodeInvalid, "A code must not be redeemable twice")
	})

	s.Run("UnknownCode", func() {
		auth := s.authenticator(config.TrustLoginConfig{})

		_, err := auth.Authenticate(
			s.requestContext(trustTestUserAgent, trustTestClientIP),
			security.Authentication{Type: AuthTypeTrustCode, Principal: trustTestAppID, Credentials: "never-issued"},
		)
		s.ErrorIs(err, security.ErrTrustCodeInvalid, "An unknown code should be rejected")
	})

	s.Run("NonStringCredentials", func() {
		auth := s.authenticator(config.TrustLoginConfig{})

		_, err := auth.Authenticate(
			s.requestContext(trustTestUserAgent, trustTestClientIP),
			security.Authentication{Type: AuthTypeTrustCode, Principal: trustTestAppID, Credentials: map[string]any{"code": "x"}},
		)
		s.ErrorIs(err, security.ErrTrustCodeInvalid, "A credential that is not a code string should be rejected")
	})

	s.Run("EmptyCredentials", func() {
		auth := s.authenticator(config.TrustLoginConfig{})

		_, err := auth.Authenticate(
			s.requestContext(trustTestUserAgent, trustTestClientIP),
			security.Authentication{Type: AuthTypeTrustCode, Principal: trustTestAppID, Credentials: ""},
		)
		s.ErrorIs(err, security.ErrTrustCodeInvalid, "An empty code should be rejected")
	})

	// The app ID reaches the lockout counter and the audit trail, so it must be
	// the one the gateway recorded rather than whatever the client asserted.
	s.Run("AppIDMismatch", func() {
		auth := s.authenticator(config.TrustLoginConfig{})
		code := s.issue()

		_, err := auth.Authenticate(
			s.requestContext(trustTestUserAgent, trustTestClientIP),
			security.Authentication{Type: AuthTypeTrustCode, Principal: "other-app", Credentials: code},
		)
		s.ErrorIs(err, security.ErrTrustCodeInvalid, "A code presented under another app's ID should be rejected")
	})
}

// TestBrowserBinding covers the defense against a code lifted out of a URL and
// redeemed elsewhere inside its short lifetime.
func (s *TrustCodeAuthenticatorTestSuite) TestBrowserBinding() {
	disabled := false

	s.Run("UserAgentBoundByDefault", func() {
		auth := s.authenticator(config.TrustLoginConfig{})
		code := s.issue()

		_, err := auth.Authenticate(
			s.requestContext("curl/8.0", trustTestClientIP),
			security.Authentication{Type: AuthTypeTrustCode, Principal: trustTestAppID, Credentials: code},
		)
		s.ErrorIs(err, security.ErrTrustCodeInvalid,
			"An omitted bind_user_agent must resolve to enabled, so another browser cannot redeem the code")
	})

	s.Run("UserAgentBindingDisabled", func() {
		auth := s.authenticator(config.TrustLoginConfig{BindUserAgent: &disabled})
		code := s.issue()

		_, err := auth.Authenticate(
			s.requestContext("curl/8.0", trustTestClientIP),
			security.Authentication{Type: AuthTypeTrustCode, Principal: trustTestAppID, Credentials: code},
		)
		s.NoError(err, "An explicitly disabled user-agent binding should admit a different browser")
	})

	s.Run("ClientIPUnboundByDefault", func() {
		auth := s.authenticator(config.TrustLoginConfig{})
		code := s.issue()

		_, err := auth.Authenticate(
			s.requestContext(trustTestUserAgent, "10.0.0.1"),
			security.Authentication{Type: AuthTypeTrustCode, Principal: trustTestAppID, Credentials: code},
		)
		s.NoError(err, "IP binding is off by default so a network change mid-redirect does not break the login")
	})

	s.Run("ClientIPBindingEnabled", func() {
		auth := s.authenticator(config.TrustLoginConfig{BindClientIP: true})
		code := s.issue()

		_, err := auth.Authenticate(
			s.requestContext(trustTestUserAgent, "10.0.0.1"),
			security.Authentication{Type: AuthTypeTrustCode, Principal: trustTestAppID, Credentials: code},
		)
		s.ErrorIs(err, security.ErrTrustCodeInvalid, "An enabled IP binding should reject a different source address")
	})

	// A rejected binding still consumes the code: the store redeems before the
	// check, so a failed guess cannot be retried from the right browser.
	s.Run("RejectedBindingStillConsumesTheCode", func() {
		auth := s.authenticator(config.TrustLoginConfig{})
		code := s.issue()

		_, err := auth.Authenticate(
			s.requestContext("curl/8.0", trustTestClientIP),
			security.Authentication{Type: AuthTypeTrustCode, Principal: trustTestAppID, Credentials: code},
		)
		s.Require().Error(err, "The mismatched browser should be rejected")

		_, err = auth.Authenticate(
			s.requestContext(trustTestUserAgent, trustTestClientIP),
			security.Authentication{Type: AuthTypeTrustCode, Principal: trustTestAppID, Credentials: code},
		)
		s.ErrorIs(err, security.ErrTrustCodeInvalid,
			"The code was already spent by the rejected attempt and must not work afterwards")
	})
}

func TestTrustCodeAuthenticator(t *testing.T) {
	suite.Run(t, new(TrustCodeAuthenticatorTestSuite))
}
