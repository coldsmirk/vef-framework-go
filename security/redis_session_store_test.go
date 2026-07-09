package security

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

type RedisSessionStoreTestSuite struct {
	suite.Suite

	container *testx.RedisContainer
	client    *redis.Client
	store     SessionStore
}

func (s *RedisSessionStoreTestSuite) SetupSuite() {
	ctx := context.Background()
	s.container = testx.NewRedisContainer(ctx, s.T())

	s.client = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", s.container.Redis.Host, s.container.Redis.Port),
		DB:   int(s.container.Redis.Database),
	})

	err := s.client.Ping(ctx).Err()
	s.Require().NoError(err, "Should connect to Redis")

	s.store = NewRedisSessionStore(s.client)
}

func (s *RedisSessionStoreTestSuite) TearDownSuite() {
	if s.client != nil {
		s.client.Close()
	}
}

func (s *RedisSessionStoreTestSuite) SetupTest() {
	s.client.FlushDB(context.Background())
}

func (s *RedisSessionStoreTestSuite) TestSessionLifecycle() {
	ctx := context.Background()
	future := time.Now().Add(time.Hour)

	s.Run("CreateLookupRevoke", func() {
		s.Require().NoError(s.store.Create(ctx, "hash-1", makeSession("s1", "u1", future), time.Hour), "create should succeed")

		got, err := s.store.Lookup(ctx, "hash-1")
		s.Require().NoError(err, "lookup should not error")
		s.Require().NotNil(got, "an active session should be found")
		s.Equal("s1", got.ID, "lookup should return the session")
		s.Equal("u1", got.Principal.ID, "the principal snapshot should round-trip through Redis")

		s.Require().NoError(s.store.Revoke(ctx, "s1"), "revoke should succeed")

		gone, err := s.store.Lookup(ctx, "hash-1")
		s.Require().NoError(err, "lookup should not error")
		s.Nil(gone, "a revoked session should be gone")
	})

	s.Run("TTLExpiry", func() {
		s.Require().NoError(s.store.Create(ctx, "hash-ttl", makeSession("sttl", "u1", time.Now().Add(300*time.Millisecond)), 200*time.Millisecond), "create should succeed")

		time.Sleep(400 * time.Millisecond)

		got, err := s.store.Lookup(ctx, "hash-ttl")
		s.Require().NoError(err, "lookup should not error")
		s.Nil(got, "an expired session key should be gone")
	})

	s.Run("RenewExtendsExpiryThenNoOpAfterRevoke", func() {
		s.Require().NoError(s.store.Create(ctx, "hren", makeSession("sren", "u1", time.Now().Add(time.Minute)), time.Minute), "create should succeed")

		newExpiry := time.Now().Add(2 * time.Hour)
		s.Require().NoError(s.store.Renew(ctx, "hren", newExpiry, 2*time.Hour), "renew should succeed")

		got, err := s.store.Lookup(ctx, "hren")
		s.Require().NoError(err, "lookup should not error")
		s.Require().NotNil(got, "the renewed session should still be found")
		s.WithinDuration(newExpiry, got.ExpiresAt, time.Second, "renew should extend the stored expiry")

		// Renewing after the session is revoked must not recreate it.
		s.Require().NoError(s.store.Revoke(ctx, "sren"), "revoke should succeed")
		s.Require().NoError(s.store.Renew(ctx, "hren", time.Now().Add(time.Hour), time.Hour), "renew after revoke should be a safe no-op")

		after, err := s.store.Lookup(ctx, "hren")
		s.Require().NoError(err, "lookup should not error")
		s.Nil(after, "renew must not resurrect a revoked session")
	})

	s.Run("ExpiresAtIsAuthoritativeOverKeyTTL", func() {
		// Past ExpiresAt but a long Redis key TTL: Lookup must still reject the
		// session so the absolute max-lifetime cap holds on the multi-node path.
		s.Require().NoError(s.store.Create(ctx, "hpast", makeSession("spast", "u1", time.Now().Add(-time.Minute)), time.Hour), "create should succeed")

		got, err := s.store.Lookup(ctx, "hpast")
		s.Require().NoError(err, "lookup should not error")
		s.Nil(got, "a past-ExpiresAt session must be rejected even while its Redis key TTL is alive")
	})

	s.Run("ListAllSpansUsers", func() {
		s.Require().NoError(s.store.Create(ctx, "ha", makeSession("sa", "u1", future), time.Hour), "create should succeed")
		s.Require().NoError(s.store.Create(ctx, "hb", makeSession("sb", "u2", future), time.Hour), "create should succeed")

		inspector, ok := s.store.(SessionInspector)
		s.Require().True(ok, "the redis store should implement SessionInspector")

		all, err := inspector.ListAll(ctx)
		s.Require().NoError(err, "list-all should not error")
		s.Len(all, 2, "list-all should span every user's sessions")
	})

	s.Run("ListAndRevokeUser", func() {
		s.Require().NoError(s.store.Create(ctx, "h1", makeSession("s1", "u1", future), time.Hour), "create should succeed")
		s.Require().NoError(s.store.Create(ctx, "h2", makeSession("s2", "u1", future), time.Hour), "create should succeed")
		s.Require().NoError(s.store.Create(ctx, "h3", makeSession("s3", "u2", future), time.Hour), "create should succeed")

		sessions, err := s.store.ListByUser(ctx, "u1")
		s.Require().NoError(err, "list should not error")
		s.Len(sessions, 2, "only the user's own sessions should be listed")

		s.Require().NoError(s.store.RevokeUser(ctx, "u1"), "revoke-user should succeed")

		u1, err := s.store.ListByUser(ctx, "u1")
		s.Require().NoError(err, "list should not error")
		s.Empty(u1, "the target user's sessions should be cleared")

		other, err := s.store.Lookup(ctx, "h3")
		s.Require().NoError(err, "lookup should not error")
		s.NotNil(other, "another user's session must be untouched")
	})
}

func TestRedisSessionStore(t *testing.T) {
	suite.Run(t, new(RedisSessionStoreTestSuite))
}
