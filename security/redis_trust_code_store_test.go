package security

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"

	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

type RedisTrustCodeStoreTestSuite struct {
	suite.Suite

	container *testx.RedisContainer
	client    *redis.Client
	store     TrustCodeStore
}

func (s *RedisTrustCodeStoreTestSuite) SetupSuite() {
	ctx := context.Background()
	s.container = testx.NewRedisContainer(ctx, s.T())

	s.client = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", s.container.Redis.Host, s.container.Redis.Port),
		DB:   int(s.container.Redis.Database),
	})

	s.Require().NoError(s.client.Ping(ctx).Err(), "Should connect to Redis")

	s.store = NewRedisTrustCodeStore(s.client)
}

func (s *RedisTrustCodeStoreTestSuite) TearDownSuite() {
	if s.client != nil {
		s.client.Close()
	}
}

func (s *RedisTrustCodeStoreTestSuite) SetupTest() {
	s.client.FlushDB(context.Background())
}

func (s *RedisTrustCodeStoreTestSuite) TestIssueThenConsume() {
	ctx := context.Background()

	code, err := s.store.Issue(ctx, trustCodeTestState(), trustCodeTestTTL)
	s.Require().NoError(err, "Issuing a code should succeed")
	s.NotEmpty(code, "The issued code should not be empty")

	state, err := s.store.Consume(ctx, code)
	s.Require().NoError(err, "Consuming a freshly issued code should succeed")
	s.Require().NotNil(state, "Consuming should return the parked state")
	s.Equal("his", state.AppID, "The app ID should survive the round trip")
	s.Equal("Mozilla/5.0", state.UserAgent, "The user agent should survive the round trip")
	s.Equal("192.168.10.9", state.ClientIP, "The client IP should survive the round trip")
	s.Require().NotNil(state.Principal, "The principal should survive the round trip")
	s.Equal("user001", state.Principal.ID, "The principal ID should survive the round trip")
	s.Equal(PrincipalTypeUser, state.Principal.Type, "The principal type should survive the round trip")
	s.Equal([]string{"admin"}, state.Principal.Roles, "The principal roles should survive the round trip")
}

func (s *RedisTrustCodeStoreTestSuite) TestConsume() {
	ctx := context.Background()

	s.Run("SecondConsumeFails", func() {
		code, err := s.store.Issue(ctx, trustCodeTestState(), trustCodeTestTTL)
		s.Require().NoError(err, "Issuing a code should succeed")

		_, err = s.store.Consume(ctx, code)
		s.Require().NoError(err, "The first consume should succeed")

		_, err = s.store.Consume(ctx, code)
		s.ErrorIs(err, ErrTrustCodeInvalid, "GETDEL must leave nothing behind for a second redemption")
	})

	s.Run("UnknownCodeFails", func() {
		_, err := s.store.Consume(ctx, "never-issued")
		s.ErrorIs(err, ErrTrustCodeInvalid, "An unknown code should be rejected")
	})

	s.Run("EmptyCodeFails", func() {
		_, err := s.store.Consume(ctx, "")
		s.ErrorIs(err, ErrTrustCodeInvalid, "An empty code should be rejected without a round trip")
	})

	s.Run("ExpiredCodeFails", func() {
		code, err := s.store.Issue(ctx, trustCodeTestState(), 10*time.Millisecond)
		s.Require().NoError(err, "Issuing a code should succeed")

		time.Sleep(80 * time.Millisecond)

		_, err = s.store.Consume(ctx, code)
		s.ErrorIs(err, ErrTrustCodeInvalid, "A code past its TTL should be rejected")
	})

	s.Run("ConcurrentConsumeAdmitsExactlyOne", func() {
		const racers = 16

		code, err := s.store.Issue(ctx, trustCodeTestState(), trustCodeTestTTL)
		s.Require().NoError(err, "Issuing a code should succeed")

		var (
			succeeded atomic.Int32
			start     sync.WaitGroup
			done      sync.WaitGroup
		)

		start.Add(1)
		done.Add(racers)

		for range racers {
			go func() {
				defer done.Done()

				start.Wait()

				if _, err := s.store.Consume(ctx, code); err == nil {
					succeeded.Add(1)
				}
			}()
		}

		start.Done()
		done.Wait()

		s.Equal(int32(1), succeeded.Load(),
			"Exactly one of the racing consumers may redeem the code — this is what a second replica relies on")
	})
}

// TestCodeIsNotStoredVerbatim proves a store dump yields no usable codes: the
// key is the code's hash, mirroring how sessions are keyed.
func (s *RedisTrustCodeStoreTestSuite) TestCodeIsNotStoredVerbatim() {
	ctx := context.Background()

	code, err := s.store.Issue(ctx, trustCodeTestState(), trustCodeTestTTL)
	s.Require().NoError(err, "Issuing a code should succeed")

	keys, err := s.client.Keys(ctx, redisTrustCodePrefix+"*").Result()
	s.Require().NoError(err, "Scanning the trust code keyspace should succeed")
	s.Require().Len(keys, 1, "Exactly one code should be parked")
	s.NotContains(keys[0], code, "The raw code must never appear in a Redis key")
	s.Equal(redisTrustCodePrefix+HashOpaqueToken(code), keys[0], "The key should be the code's hash")
}

func TestRedisTrustCodeStore(t *testing.T) {
	suite.Run(t, new(RedisTrustCodeStoreTestSuite))
}
