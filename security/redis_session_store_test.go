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
