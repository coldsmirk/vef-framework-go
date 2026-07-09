package security

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

type OpaqueTokenGeneratorTestSuite struct {
	suite.Suite
}

func (s *OpaqueTokenGeneratorTestSuite) TestGenerate() {
	ctx := context.Background()
	principal := security.NewUser("u1", "Alice", "admin")
	meta := security.SessionMeta{ClientIP: "10.0.0.1", UserAgent: "test-agent"}

	s.Run("OpensSessionAndReturnsOpaqueToken", func() {
		store := security.NewMemorySessionStore()
		gen := NewOpaqueTokenGenerator(store, security.SessionPolicy{IdleTTL: time.Hour})

		tokens, err := gen.Generate(ctx, principal, meta)
		s.Require().NoError(err, "generation should succeed")
		s.Require().NotEmpty(tokens.AccessToken, "an opaque access token should be issued")
		s.Empty(tokens.RefreshToken, "an opaque token carries no refresh token")

		session, err := store.Lookup(ctx, security.HashOpaqueToken(tokens.AccessToken))
		s.Require().NoError(err, "lookup should not error")
		s.Require().NotNil(session, "the issued token should resolve to a session")
		s.Equal("u1", session.Principal.ID, "the session should snapshot the principal")
		s.Equal("10.0.0.1", session.ClientIP, "the session should record the client ip")
	})

	s.Run("RejectStrategyDeniesBeyondLimit", func() {
		store := security.NewMemorySessionStore()
		gen := NewOpaqueTokenGenerator(store, security.SessionPolicy{
			MaxConcurrent: 1,
			OnExceed:      security.SessionExceedReject,
			IdleTTL:       time.Hour,
		})

		_, err := gen.Generate(ctx, principal, meta)
		s.Require().NoError(err, "the first session should be admitted")

		_, err = gen.Generate(ctx, principal, meta)
		resErr, ok := result.AsErr(err)
		s.Require().True(ok, "exceeding the limit should be a result.Error")
		s.Equal(security.ErrCodeTooManyConcurrentSessions, resErr.Code, "the reject strategy should deny the new login")
	})

	s.Run("EvictOldestStrategyKicksEarliestSession", func() {
		store := security.NewMemorySessionStore()
		gen := NewOpaqueTokenGenerator(store, security.SessionPolicy{
			MaxConcurrent: 1,
			OnExceed:      security.SessionExceedEvictOldest,
			IdleTTL:       time.Hour,
		})

		first, err := gen.Generate(ctx, principal, meta)
		s.Require().NoError(err, "the first session should be admitted")

		second, err := gen.Generate(ctx, principal, meta)
		s.Require().NoError(err, "evict-oldest should admit the new session")

		oldest, err := store.Lookup(ctx, security.HashOpaqueToken(first.AccessToken))
		s.Require().NoError(err, "lookup should not error")
		s.Nil(oldest, "the oldest session should have been kicked offline")

		newest, err := store.Lookup(ctx, security.HashOpaqueToken(second.AccessToken))
		s.Require().NoError(err, "lookup should not error")
		s.NotNil(newest, "the newest session should remain active")

		active, err := store.ListByUser(ctx, "u1")
		s.Require().NoError(err, "list should not error")
		s.Len(active, 1, "the concurrency limit should hold at exactly one session")
	})

	s.Run("EvictOldestPreservesNewerSessionAtLimitTwo", func() {
		store := security.NewMemorySessionStore()
		gen := NewOpaqueTokenGenerator(store, security.SessionPolicy{
			MaxConcurrent: 2,
			OnExceed:      security.SessionExceedEvictOldest,
			IdleTTL:       time.Hour,
		})

		now := time.Now()
		future := now.Add(time.Hour)
		// Seed two sessions with distinct creation times so the eviction ordering
		// (oldest, not newest) is actually distinguishable — a reversed sort would
		// evict the wrong one and fail this test.
		oldest := security.Session{ID: "old", UserID: "u1", Principal: principal, CreatedAt: now.Add(-2 * time.Hour), LastSeenAt: now.Add(-2 * time.Hour), ExpiresAt: future}
		middle := security.Session{ID: "mid", UserID: "u1", Principal: principal, CreatedAt: now.Add(-time.Hour), LastSeenAt: now.Add(-time.Hour), ExpiresAt: future}
		s.Require().NoError(store.Create(ctx, "hash-old", oldest, time.Hour), "seeding the oldest session should succeed")
		s.Require().NoError(store.Create(ctx, "hash-mid", middle, time.Hour), "seeding the middle session should succeed")

		_, err := gen.Generate(ctx, principal, meta)
		s.Require().NoError(err, "evict-oldest should admit the new session at the limit")

		gone, err := store.Lookup(ctx, "hash-old")
		s.Require().NoError(err, "lookup should not error")
		s.Nil(gone, "the oldest session should be the one evicted")

		survived, err := store.Lookup(ctx, "hash-mid")
		s.Require().NoError(err, "lookup should not error")
		s.NotNil(survived, "the newer of the two existing sessions must survive")

		active, err := store.ListByUser(ctx, "u1")
		s.Require().NoError(err, "list should not error")
		s.Len(active, 2, "the concurrency limit should hold at exactly two sessions")
	})

	s.Run("UnlimitedWhenMaxConcurrentZero", func() {
		store := security.NewMemorySessionStore()
		gen := NewOpaqueTokenGenerator(store, security.SessionPolicy{IdleTTL: time.Hour})

		for range 3 {
			_, err := gen.Generate(ctx, principal, meta)
			s.Require().NoError(err, "unlimited concurrency should admit every session")
		}

		active, err := store.ListByUser(ctx, "u1")
		s.Require().NoError(err, "list should not error")
		s.Len(active, 3, "all sessions should remain when concurrency is unlimited")
	})
}

func TestOpaqueTokenGenerator(t *testing.T) {
	suite.Run(t, new(OpaqueTokenGeneratorTestSuite))
}
