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

type RedisLoginGuardTestSuite struct {
	suite.Suite

	container *testx.RedisContainer
	client    *redis.Client
	guard     LoginGuard
}

func (s *RedisLoginGuardTestSuite) SetupSuite() {
	ctx := context.Background()
	s.container = testx.NewRedisContainer(ctx, s.T())

	s.client = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", s.container.Redis.Host, s.container.Redis.Port),
		DB:   int(s.container.Redis.Database),
	})

	err := s.client.Ping(ctx).Err()
	s.Require().NoError(err, "Should connect to Redis")

	s.guard = NewRedisLoginGuard(s.client, LockoutPolicy{
		MaxFailures:  3,
		Window:       time.Minute,
		LockDuration: 15 * time.Minute,
		Strategy:     LockoutStrategyLock,
		Key:          LockoutKeyUserIP,
	})
}

func (s *RedisLoginGuardTestSuite) TearDownSuite() {
	if s.client != nil {
		s.client.Close()
	}
}

func (s *RedisLoginGuardTestSuite) SetupTest() {
	s.client.FlushDB(context.Background())
}

func (s *RedisLoginGuardTestSuite) TestLockout() {
	ctx := context.Background()

	s.Run("AllowsBelowThreshold", func() {
		attempt := LoginAttempt{Identity: "alice", ClientIP: "10.0.0.1"}

		for range 2 {
			decision, err := s.guard.RecordFailure(ctx, attempt)
			s.Require().NoError(err, "Should record a failure without error")
			s.True(decision.Allowed, "Should stay allowed below the threshold")
		}

		decision, err := s.guard.Check(ctx, attempt)
		s.Require().NoError(err, "Check should not error")
		s.True(decision.Allowed, "Should be allowed after 2 of 3 failures")
	})

	s.Run("LocksAtThreshold", func() {
		attempt := LoginAttempt{Identity: "bob", ClientIP: "10.0.0.2"}

		var last LoginDecision
		for range 3 {
			d, err := s.guard.RecordFailure(ctx, attempt)
			s.Require().NoError(err, "Should record a failure without error")

			last = d
		}

		s.False(last.Allowed, "Third failure should trip the lock")
		s.Positive(last.RetryAfter, "Locked failure should report a positive retry-after")

		decision, err := s.guard.Check(ctx, attempt)
		s.Require().NoError(err, "Check should not error")
		s.False(decision.Allowed, "A locked identity should be denied on check")
		s.InDelta((15 * time.Minute).Seconds(), decision.RetryAfter.Seconds(), 5, "Retry-after should reflect the shared cooldown TTL")
	})

	s.Run("SuccessClearsSharedCounters", func() {
		attempt := LoginAttempt{Identity: "carol", ClientIP: "10.0.0.3"}

		for range 3 {
			_, err := s.guard.RecordFailure(ctx, attempt)
			s.Require().NoError(err, "Should record a failure without error")
		}

		s.Require().NoError(s.guard.RecordSuccess(ctx, attempt), "Should clear counters without error")

		decision, err := s.guard.Check(ctx, attempt)
		s.Require().NoError(err, "Check should not error")
		s.True(decision.Allowed, "A successful login should clear the shared lock")
	})

	s.Run("IsolatesByKey", func() {
		locked := LoginAttempt{Identity: "dave", ClientIP: "10.0.0.4"}
		for range 3 {
			_, err := s.guard.RecordFailure(ctx, locked)
			s.Require().NoError(err, "Should record a failure without error")
		}

		other := LoginAttempt{Identity: "dave", ClientIP: "10.0.0.5"}
		decision, err := s.guard.Check(ctx, other)
		s.Require().NoError(err, "Check should not error")
		s.True(decision.Allowed, "A different source IP must not inherit another key's lock")
	})
}

func TestRedisLoginGuard(t *testing.T) {
	suite.Run(t, new(RedisLoginGuardTestSuite))
}
