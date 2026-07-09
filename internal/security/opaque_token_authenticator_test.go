package security

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

type OpaqueTokenAuthenticatorTestSuite struct {
	suite.Suite
}

// storeToken opens a session for a fresh token and returns the raw token.
func (s *OpaqueTokenAuthenticatorTestSuite) storeToken(store security.SessionStore, session security.Session) string {
	token, err := security.GenerateOpaqueToken()
	s.Require().NoError(err, "should generate an opaque token")
	s.Require().NoError(store.Create(context.Background(), security.HashOpaqueToken(token), session, time.Hour), "should store the session")

	return token
}

func (s *OpaqueTokenAuthenticatorTestSuite) TestSupports() {
	auth := NewOpaqueTokenAuthenticator(security.NewMemorySessionStore(), security.SessionPolicy{})
	s.True(auth.Supports(AuthTypeOpaqueToken), "should support opaque_token")
	s.False(auth.Supports(AuthTypeJWTToken), "should not support jwt_token")
	s.False(auth.Supports(""), "should not support the empty type")
}

func (s *OpaqueTokenAuthenticatorTestSuite) TestAuthenticate() {
	ctx := context.Background()

	s.Run("EmptyToken", func() {
		auth := NewOpaqueTokenAuthenticator(security.NewMemorySessionStore(), security.SessionPolicy{})

		_, err := auth.Authenticate(ctx, security.Authentication{Type: AuthTypeOpaqueToken, Principal: ""})

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "an empty token should be a result.Error")
		s.Equal(security.ErrCodeTokenInvalid, resErr.Code, "an empty token should be invalid")
	})

	s.Run("UnknownToken", func() {
		auth := NewOpaqueTokenAuthenticator(security.NewMemorySessionStore(), security.SessionPolicy{})

		_, err := auth.Authenticate(ctx, security.Authentication{Type: AuthTypeOpaqueToken, Principal: "not-a-real-token"})

		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "an unknown token should be a result.Error")
		s.Equal(security.ErrCodeTokenInvalid, resErr.Code, "an unknown token should be invalid")
	})

	s.Run("ResolvesPrincipalSnapshot", func() {
		store := security.NewMemorySessionStore()
		auth := NewOpaqueTokenAuthenticator(store, security.SessionPolicy{IdleTTL: time.Hour})

		principal := security.NewUser("u1", "Alice", "admin")
		now := time.Now()
		token := s.storeToken(store, security.Session{
			ID: "s1", UserID: "u1", Principal: principal,
			CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Minute),
		})

		got, err := auth.Authenticate(ctx, security.Authentication{Type: AuthTypeOpaqueToken, Principal: token})
		s.Require().NoError(err, "a valid opaque token should authenticate")
		s.Equal("u1", got.ID, "should return the session's principal id")
		s.Equal([]string{"admin"}, got.Roles, "should return the session's principal roles")
	})

	s.Run("SlidingRenewalExtendsExpiry", func() {
		store := security.NewMemorySessionStore()
		auth := NewOpaqueTokenAuthenticator(store, security.SessionPolicy{IdleTTL: time.Hour, Sliding: true})

		now := time.Now()
		token := s.storeToken(store, security.Session{
			ID: "s1", UserID: "u1", Principal: security.NewUser("u1", "Alice"),
			CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Minute),
		})

		_, err := auth.Authenticate(ctx, security.Authentication{Type: AuthTypeOpaqueToken, Principal: token})
		s.Require().NoError(err, "authentication should succeed")

		session, err := store.Lookup(ctx, security.HashOpaqueToken(token))
		s.Require().NoError(err, "lookup should not error")
		s.Require().NotNil(session, "the session should still exist")
		s.True(session.ExpiresAt.After(now.Add(30*time.Minute)), "sliding renewal should push the expiry out by the idle ttl")
	})

	s.Run("RenewalRespectsMaxLifetime", func() {
		store := security.NewMemorySessionStore()
		auth := NewOpaqueTokenAuthenticator(store, security.SessionPolicy{IdleTTL: time.Hour, MaxLifetime: time.Hour, Sliding: true})

		created := time.Now().Add(-50 * time.Minute)
		token := s.storeToken(store, security.Session{
			ID: "s1", UserID: "u1", Principal: security.NewUser("u1", "Alice"),
			CreatedAt: created, LastSeenAt: created, ExpiresAt: time.Now().Add(time.Minute),
		})

		_, err := auth.Authenticate(ctx, security.Authentication{Type: AuthTypeOpaqueToken, Principal: token})
		s.Require().NoError(err, "authentication should succeed")

		session, err := store.Lookup(ctx, security.HashOpaqueToken(token))
		s.Require().NoError(err, "lookup should not error")
		s.Require().NotNil(session, "the session should still exist")
		// created + 1h max-lifetime = now + ~10m, well before the full idle ttl.
		s.True(session.ExpiresAt.Before(time.Now().Add(30*time.Minute)), "renewal must not exceed the absolute max lifetime")
	})
}

func TestOpaqueTokenAuthenticator(t *testing.T) {
	suite.Run(t, new(OpaqueTokenAuthenticatorTestSuite))
}
