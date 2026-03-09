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

type RedisNonceStoreTestSuite struct {
	suite.Suite

	container *testx.RedisContainer
	client    *redis.Client
	store     NonceStore
}

func (s *RedisNonceStoreTestSuite) SetupSuite() {
	ctx := context.Background()
	s.container = testx.NewRedisContainer(ctx, s.T())

	s.client = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", s.container.Redis.Host, s.container.Redis.Port),
		DB:   int(s.container.Redis.Database),
	})

	err := s.client.Ping(ctx).Err()
	s.Require().NoError(err, "Should connect to Redis")

	s.store = NewRedisNonceStore(s.client)
}

func (s *RedisNonceStoreTestSuite) TearDownSuite() {
	if s.client != nil {
		s.client.Close()
	}
}

func (s *RedisNonceStoreTestSuite) SetupTest() {
	s.client.FlushDB(context.Background())
}

// TestStoreIfAbsent tests atomic nonce reservation behavior.
func (s *RedisNonceStoreTestSuite) TestStoreIfAbsent() {
	ctx := context.Background()

	s.Run("StoreNewNonce", func() {
		stored, err := s.store.StoreIfAbsent(ctx, "test-app", "new-nonce", 5*time.Minute)

		s.Require().NoError(err, "Should atomically store nonce without error")
		s.True(stored, "First StoreIfAbsent should store nonce")
	})

	s.Run("StoreDuplicateNonce", func() {
		stored, err := s.store.StoreIfAbsent(ctx, "test-app", "duplicate-nonce", 5*time.Minute)
		s.Require().NoError(err, "Should atomically store nonce without error")
		s.True(stored, "First StoreIfAbsent should store nonce")

		stored, err = s.store.StoreIfAbsent(ctx, "test-app", "duplicate-nonce", 5*time.Minute)
		s.Require().NoError(err, "Should atomically check duplicate without error")
		s.False(stored, "Second StoreIfAbsent should not store duplicate nonce")
	})

	s.Run("SameNonceDifferentApps", func() {
		stored, err := s.store.StoreIfAbsent(ctx, "app-1", "shared-nonce", 5*time.Minute)
		s.Require().NoError(err, "Should store nonce for first app")
		s.True(stored, "First app should store nonce")

		stored, err = s.store.StoreIfAbsent(ctx, "app-2", "shared-nonce", 5*time.Minute)
		s.Require().NoError(err, "Should store same nonce for different app")
		s.True(stored, "Different app should store same nonce independently")
	})

	s.Run("EmptyAppID", func() {
		stored, err := s.store.StoreIfAbsent(ctx, "", "test-nonce", 5*time.Minute)
		s.Require().NoError(err, "Should support empty appID")
		s.True(stored, "Should store nonce with empty appID")
	})

	s.Run("EmptyNonce", func() {
		stored, err := s.store.StoreIfAbsent(ctx, "test-app", "", 5*time.Minute)
		s.Require().NoError(err, "Should support empty nonce")
		s.True(stored, "Should store nonce with empty nonce")
	})
}

// TestTTLExpiration tests nonce expiration behavior.
func (s *RedisNonceStoreTestSuite) TestTTLExpiration() {
	ctx := context.Background()

	s.Run("NonceCanBeReusedAfterTTL", func() {
		stored, err := s.store.StoreIfAbsent(ctx, "test-app", "expiring-nonce", 50*time.Millisecond)
		s.Require().NoError(err, "Should store expiring nonce")
		s.True(stored, "First store should succeed")

		stored, err = s.store.StoreIfAbsent(ctx, "test-app", "expiring-nonce", 50*time.Millisecond)
		s.Require().NoError(err, "Should check duplicate before expiry")
		s.False(stored, "Duplicate should be rejected before TTL expiration")

		time.Sleep(100 * time.Millisecond)

		stored, err = s.store.StoreIfAbsent(ctx, "test-app", "expiring-nonce", 50*time.Millisecond)
		s.Require().NoError(err, "Should allow reuse after expiration")
		s.True(stored, "Nonce should be reusable after expiration")
	})
}

// TestKeyFormat tests special app/nonce value handling.
func (s *RedisNonceStoreTestSuite) TestKeyFormat() {
	ctx := context.Background()

	s.Run("SpecialCharacters", func() {
		testCases := []struct {
			appID string
			nonce string
		}{
			{"app:with:colons", "nonce:with:colons"},
			{"app-with-dashes", "nonce-with-dashes"},
			{"app_with_underscores", "nonce_with_underscores"},
			{"app.with.dots", "nonce.with.dots"},
			{"app/with/slashes", "nonce/with/slashes"},
			{"应用", "随机数"},
		}

		for _, tc := range testCases {
			stored, err := s.store.StoreIfAbsent(ctx, tc.appID, tc.nonce, 5*time.Minute)
			s.Require().NoError(err, "Should store nonce with special characters")
			s.True(stored, "First store should succeed for appID=%q nonce=%q", tc.appID, tc.nonce)
		}
	})
}

// TestConcurrency tests concurrent reservation behavior.
func (s *RedisNonceStoreTestSuite) TestConcurrency() {
	ctx := context.Background()

	s.Run("ConcurrentSameNonceOnlyOneSucceeds", func() {
		const numGoroutines = 100

		var successCount atomic.Int32

		errCh := make(chan error, numGoroutines)

		var wg sync.WaitGroup
		for range numGoroutines {
			wg.Go(func() {
				stored, err := s.store.StoreIfAbsent(ctx, "test-app", "same-nonce", 5*time.Minute)
				if err != nil {
					errCh <- err

					return
				}

				if stored {
					successCount.Add(1)
				}
			})
		}

		wg.Wait()
		close(errCh)

		for err := range errCh {
			s.Require().NoError(err, "Concurrent StoreIfAbsent should not error")
		}

		s.Equal(int32(1), successCount.Load(), "Exactly one goroutine should reserve the nonce")
	})

	s.Run("ConcurrentDifferentAppsAllSucceed", func() {
		const numGoroutines = 50

		var successCount atomic.Int32

		errCh := make(chan error, numGoroutines)

		var wg sync.WaitGroup
		for i := range numGoroutines {
			wg.Go(func() {
				appID := fmt.Sprintf("app-%d", i)

				stored, err := s.store.StoreIfAbsent(ctx, appID, "same-nonce", 5*time.Minute)
				if err != nil {
					errCh <- err

					return
				}

				if stored {
					successCount.Add(1)
				}
			})
		}

		wg.Wait()
		close(errCh)

		for err := range errCh {
			s.Require().NoError(err, "Concurrent StoreIfAbsent should not error")
		}

		s.Equal(int32(numGoroutines), successCount.Load(), "All goroutines should reserve nonce for different apps")
	})
}

// TestRedisNonceStoreTestSuite runs the Redis nonce store test suite.
func TestRedisNonceStore(t *testing.T) {
	suite.Run(t, new(RedisNonceStoreTestSuite))
}
