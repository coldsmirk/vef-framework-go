package cache

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	goredis "github.com/redis/go-redis/v9"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/redis"
	"github.com/coldsmirk/vef-framework-go/internal/testx"
)

type TestUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type RedisCacheTestSuite struct {
	suite.Suite

	ctx            context.Context
	redisContainer *testx.RedisContainer
	client         *goredis.Client
}

func (suite *RedisCacheTestSuite) SetupSuite() {
	suite.ctx = context.Background()

	container := testx.NewRedisContainer(suite.ctx, suite.T())
	suite.redisContainer = container

	suite.client = redis.NewClient(container.Redis, &config.AppConfig{Name: "test-app"})
	err := suite.client.Ping(suite.ctx).Err()
	suite.Require().NoError(err, "failed to ping redis client")
}

func (suite *RedisCacheTestSuite) TearDownSuite() {
	if suite.client != nil {
		if err := suite.client.Close(); err != nil {
			suite.T().Logf("failed to close redis client: %v", err)
		}
	}
}

func (suite *RedisCacheTestSuite) SetupTest() {
	keys, _ := suite.client.Keys(suite.ctx, "*").Result()
	if len(keys) > 0 {
		suite.client.Del(suite.ctx, keys...)
	}
}

func (suite *RedisCacheTestSuite) setupRedisCache(namespace string, opts ...RedisOption) Cache[TestUser] {
	c := NewRedis[TestUser](suite.client, namespace, opts...)
	suite.Require().NotNil(c, "Should not be nil")

	return c
}

func (suite *RedisCacheTestSuite) setupStringCache(namespace string) Cache[string] {
	c := NewRedis[string](suite.client, namespace)
	suite.Require().NotNil(c, "Should not be nil")

	return c
}

func (suite *RedisCacheTestSuite) TestRedisCacheBasicOperations() {
	userCache := suite.setupRedisCache("test-users")
	defer userCache.Close()

	suite.Run("SetAndGet", func() {
		user := TestUser{ID: 1, Name: "Alice", Age: 30}

		err := userCache.Set(suite.ctx, "user1", user)
		suite.Require().NoError(err, "Should not return error")

		result, found := userCache.Get(suite.ctx, "user1")
		suite.True(found, "Should find cached user")
		suite.Equal(user, result, "Cached user should match original")
	})

	suite.Run("Contains", func() {
		user := TestUser{ID: 2, Name: "Bob", Age: 25}

		err := userCache.Set(suite.ctx, "user2", user)
		suite.Require().NoError(err, "Should not return error")

		suite.True(userCache.Contains(suite.ctx, "user2"), "Should contain user2")
		suite.False(userCache.Contains(suite.ctx, "nonexistent"), "Should not contain nonexistent key")
	})

	suite.Run("Delete", func() {
		user := TestUser{ID: 3, Name: "Charlie", Age: 35}

		err := userCache.Set(suite.ctx, "user3", user)
		suite.Require().NoError(err, "Should not return error")

		suite.True(userCache.Contains(suite.ctx, "user3"), "Should contain user3 before delete")

		err = userCache.Delete(suite.ctx, "user3")
		suite.Require().NoError(err, "Should not return error")

		suite.False(userCache.Contains(suite.ctx, "user3"), "Should not contain user3 after delete")
		_, found := userCache.Get(suite.ctx, "user3")
		suite.False(found, "Should not find deleted user")
	})

	suite.Run("UpdateExistingKey", func() {
		originalUser := TestUser{ID: 4, Name: "David", Age: 40}
		updatedUser := TestUser{ID: 4, Name: "David", Age: 41}

		err := userCache.Set(suite.ctx, "user4", originalUser)
		suite.Require().NoError(err, "Should not return error")

		result, found := userCache.Get(suite.ctx, "user4")
		suite.True(found, "Should find original user")
		suite.Equal(originalUser, result, "Should return original user")

		err = userCache.Set(suite.ctx, "user4", updatedUser)
		suite.Require().NoError(err, "Should not return error")

		result, found = userCache.Get(suite.ctx, "user4")
		suite.True(found, "Should find updated user")
		suite.Equal(updatedUser, result, "Should return updated user")
	})
}

func (suite *RedisCacheTestSuite) TestRedisCacheTtl() {
	userCache := suite.setupRedisCache("test-ttl-users")
	defer userCache.Close()

	suite.Run("CustomTtlExpiration", func() {
		user := TestUser{ID: 5, Name: "Eve", Age: 28}

		err := userCache.Set(suite.ctx, "ttl-user", user, 100*time.Millisecond)
		suite.Require().NoError(err, "Should not return error")

		result, found := userCache.Get(suite.ctx, "ttl-user")
		suite.True(found, "Should find user before TTL expiration")
		suite.Equal(user, result, "Cached user should match")

		time.Sleep(150 * time.Millisecond)

		_, found = userCache.Get(suite.ctx, "ttl-user")
		suite.False(found, "Should not find user after TTL expiration")
	})

	suite.Run("DefaultTtl", func() {
		cacheWithDefaultTTL := suite.setupRedisCache("test-default-ttl", WithRdsDefaultTTL(100*time.Millisecond))
		defer cacheWithDefaultTTL.Close()

		user := TestUser{ID: 6, Name: "Frank", Age: 32}

		err := cacheWithDefaultTTL.Set(suite.ctx, "default-ttl-user", user)
		suite.Require().NoError(err, "Should not return error")

		result, found := cacheWithDefaultTTL.Get(suite.ctx, "default-ttl-user")
		suite.True(found, "Should find user before default TTL expiration")
		suite.Equal(user, result, "Cached user should match")

		time.Sleep(150 * time.Millisecond)

		_, found = cacheWithDefaultTTL.Get(suite.ctx, "default-ttl-user")
		suite.False(found, "Should not find user after default TTL expiration")
	})
}

func (suite *RedisCacheTestSuite) TestRedisCacheGetOrLoad() {
	userCache := suite.setupRedisCache("test-getorload")
	defer userCache.Close()

	user := TestUser{ID: 7, Name: "Grace", Age: 26}

	var loadCount atomic.Int32

	loader := func(context.Context) (TestUser, error) {
		loadCount.Add(1)

		return user, nil
	}

	suite.Run("SingleLoad", func() {
		result, err := userCache.GetOrLoad(suite.ctx, "user7", loader)
		suite.Require().NoError(err, "Should not return error")
		suite.Equal(user, result, "Loaded user should match")
		suite.Equal(int32(1), loadCount.Load(), "Loader should be called once")
	})

	suite.Run("CachedValue", func() {
		result, err := userCache.GetOrLoad(suite.ctx, "user7", loader)
		suite.Require().NoError(err, "Should not return error")
		suite.Equal(user, result, "Cached user should match")
		suite.Equal(int32(1), loadCount.Load(), "loader should not be invoked again for cached value")
	})

	suite.Run("ConcurrentRequests", func() {
		loadCount.Store(0)

		var wg sync.WaitGroup

		const goroutines = 20

		for range goroutines {
			wg.Go(func() {
				_, err := userCache.GetOrLoad(suite.ctx, "concurrent", loader)
				suite.Require().NoError(err, "Should not return error")
			})
		}

		wg.Wait()
		suite.Equal(int32(1), loadCount.Load(), "loader should execute exactly once under contention")
	})
}

func (suite *RedisCacheTestSuite) TestRedisCacheKeyPrefixIsolation() {
	cache1 := suite.setupRedisCache("cache1")
	defer cache1.Close()

	cache2 := suite.setupRedisCache("cache2")
	defer cache2.Close()

	user1 := TestUser{ID: 1, Name: "Alice", Age: 30}
	user2 := TestUser{ID: 2, Name: "Bob", Age: 25}

	err := cache1.Set(suite.ctx, "shared-key", user1)
	suite.Require().NoError(err, "Should not return error")

	err = cache2.Set(suite.ctx, "shared-key", user2)
	suite.Require().NoError(err, "Should not return error")

	result1, found := cache1.Get(suite.ctx, "shared-key")
	suite.True(found, "Should find user in cache1")
	suite.Equal(user1, result1, "Cache1 should return user1")

	result2, found := cache2.Get(suite.ctx, "shared-key")
	suite.True(found, "Should find user in cache2")
	suite.Equal(user2, result2, "Cache2 should return user2")

	keys1, err := cache1.Keys(suite.ctx)
	suite.Require().NoError(err, "Should not return error")
	suite.Contains(keys1, "shared-key", "Should contain expected value")
	suite.Len(keys1, 1, "Cache1 should have 1 key")

	keys2, err := cache2.Keys(suite.ctx)
	suite.Require().NoError(err, "Should not return error")
	suite.Contains(keys2, "shared-key", "Should contain expected value")
	suite.Len(keys2, 1, "Cache2 should have 1 key")
}

func (suite *RedisCacheTestSuite) TestRedisCacheIteration() {
	userCache := suite.setupRedisCache("test-iteration")
	defer userCache.Close()

	testUsers := map[string]TestUser{
		"admin:1": {ID: 1, Name: "Admin Alice", Age: 35},
		"admin:2": {ID: 2, Name: "Admin Bob", Age: 40},
		"user:1":  {ID: 3, Name: "User Charlie", Age: 25},
		"user:2":  {ID: 4, Name: "User David", Age: 30},
		"guest:1": {ID: 5, Name: "Guest Eve", Age: 22},
	}

	for key, user := range testUsers {
		err := userCache.Set(suite.ctx, key, user)
		suite.Require().NoError(err, "Should not return error")
	}

	suite.Run("KeysWithoutPrefix", func() {
		keys, err := userCache.Keys(suite.ctx)
		suite.Require().NoError(err, "Should not return error")

		slices.Sort(keys)

		expectedKeys := []string{
			"admin:1",
			"admin:2",
			"guest:1",
			"user:1",
			"user:2",
		}
		suite.Equal(expectedKeys, keys, "Should return all keys sorted")
	})

	suite.Run("KeysWithPrefix", func() {
		adminKeys, err := userCache.Keys(suite.ctx, "admin")
		suite.Require().NoError(err, "Should not return error")
		slices.Sort(adminKeys)

		expectedAdmin := []string{
			"admin:1",
			"admin:2",
		}
		suite.Equal(expectedAdmin, adminKeys, "Should return admin keys only")

		userKeys, err := userCache.Keys(suite.ctx, "user")
		suite.Require().NoError(err, "Should not return error")
		slices.Sort(userKeys)

		expectedUser := []string{
			"user:1",
			"user:2",
		}
		suite.Equal(expectedUser, userKeys, "Should return user keys only")
	})

	suite.Run("ForEachWithoutPrefix", func() {
		collected := make(map[string]TestUser)

		err := userCache.ForEach(suite.ctx, func(key string, user TestUser) bool {
			collected[key] = user

			return true
		})
		suite.Require().NoError(err, "Should not return error")

		expected := map[string]TestUser{
			"admin:1": testUsers["admin:1"],
			"admin:2": testUsers["admin:2"],
			"guest:1": testUsers["guest:1"],
			"user:1":  testUsers["user:1"],
			"user:2":  testUsers["user:2"],
		}
		suite.Equal(expected, collected, "ForEach should collect all users")
	})

	suite.Run("ForEachWithPrefix", func() {
		collected := make(map[string]TestUser)

		err := userCache.ForEach(suite.ctx, func(key string, user TestUser) bool {
			collected[key] = user

			return true
		}, "admin")
		suite.Require().NoError(err, "Should not return error")

		expected := map[string]TestUser{
			"admin:1": testUsers["admin:1"],
			"admin:2": testUsers["admin:2"],
		}
		suite.Equal(expected, collected, "ForEach with prefix should collect admin users only")
	})

	suite.Run("ForEachEarlyTermination", func() {
		var count int

		err := userCache.ForEach(suite.ctx, func(string, TestUser) bool {
			count++

			return count < 3
		})
		suite.Require().NoError(err, "Should not return error")
		suite.Equal(3, count, "ForEach should stop after 3 iterations")
	})

	suite.Run("Size", func() {
		size, err := userCache.Size(suite.ctx)
		suite.Require().NoError(err, "Should not return error")
		suite.Equal(int64(5), size, "Cache size should be 5")
	})
}

func (suite *RedisCacheTestSuite) TestRedisCacheClear() {
	cache1 := suite.setupRedisCache("clear-test-1")
	defer cache1.Close()

	cache2 := suite.setupRedisCache("clear-test-2")
	defer cache2.Close()

	for i := 1; i <= 5; i++ {
		user := TestUser{ID: i, Name: fmt.Sprintf("User%d", i), Age: 20 + i}
		err := cache1.Set(suite.ctx, fmt.Sprintf("user-%d", i), user)
		suite.Require().NoError(err, "Should not return error")
	}

	user := TestUser{ID: 99, Name: "Other User", Age: 99}
	err := cache2.Set(suite.ctx, "other-user", user)
	suite.Require().NoError(err, "Should not return error")

	size1, err := cache1.Size(suite.ctx)
	suite.Require().NoError(err, "Should not return error")
	suite.Equal(int64(5), size1, "Cache1 should have 5 entries")

	size2, err := cache2.Size(suite.ctx)
	suite.Require().NoError(err, "Should not return error")
	suite.Equal(int64(1), size2, "Cache2 should have 1 entry")

	err = cache1.Clear(suite.ctx)
	suite.Require().NoError(err, "Should not return error")

	size1, err = cache1.Size(suite.ctx)
	suite.Require().NoError(err, "Should not return error")
	suite.Equal(int64(0), size1, "Cache1 should be empty after clear")

	retrieved, found := cache2.Get(suite.ctx, "other-user")
	suite.True(found, "Cache2 should still have its entry after cache1 clear")
	suite.Equal(user, retrieved, "Cache2 entry should be intact")

	size2, err = cache2.Size(suite.ctx)
	suite.Require().NoError(err, "Should not return error")
	suite.Equal(int64(1), size2, "Cache2 size should still be 1")
}

func (suite *RedisCacheTestSuite) TestRedisCacheStringValues() {
	stringCache := suite.setupStringCache("test-strings")
	defer stringCache.Close()

	err := stringCache.Set(suite.ctx, "greeting", "Hello, World!")
	suite.Require().NoError(err, "Should not return error")

	result, found := stringCache.Get(suite.ctx, "greeting")
	suite.True(found, "Should find cached string")
	suite.Equal("Hello, World!", result, "Cached string should match")

	err = stringCache.Set(suite.ctx, "farewell", "Goodbye!")
	suite.Require().NoError(err, "Should not return error")

	keys, err := stringCache.Keys(suite.ctx)
	suite.Require().NoError(err, "Should not return error")
	suite.Len(keys, 2, "Should have 2 string keys")
	suite.Contains(keys, "greeting", "Should contain expected value")
	suite.Contains(keys, "farewell", "Should contain expected value")
}

func (suite *RedisCacheTestSuite) TestRedisCacheClose() {
	cache := suite.setupRedisCache("close-behavior")

	ctx := suite.ctx

	err := cache.Close()
	suite.Require().NoError(err, "Should not return error")

	err = cache.Close()
	suite.Require().NoError(err, "Should not return error")

	err = cache.Set(ctx, "key", TestUser{ID: 1})
	suite.Require().ErrorIs(err, ErrCacheClosed, "Error should match expected value")

	_, found := cache.Get(ctx, "key")
	suite.False(found, "Get should return not-found on closed cache")
	suite.False(cache.Contains(ctx, "key"), "Contains should return false on closed cache")

	suite.Require().NoError(cache.Delete(ctx, "key"), "Should not return error")
	suite.Require().NoError(cache.Clear(ctx), "Should not return error")

	keys, err := cache.Keys(ctx)
	suite.Require().NoError(err, "Should not return error")
	suite.Nil(keys, "Keys should be nil on closed cache")

	called := false
	err = cache.ForEach(ctx, func(string, TestUser) bool {
		called = true

		return true
	})
	suite.Require().NoError(err, "Should not return error")
	suite.False(called, "callback should not be invoked after cache is closed")
}

func (suite *RedisCacheTestSuite) TestRedisCacheKeyStripping() {
	userCache := suite.setupRedisCache("key-stripping-test")
	defer userCache.Close()

	suite.Run("KeysReturnUserOriginalKeys", func() {
		testData := map[string]TestUser{
			"user:1":    {ID: 1, Name: "Alice", Age: 30},
			"user:2":    {ID: 2, Name: "Bob", Age: 25},
			"admin:100": {ID: 100, Name: "Admin", Age: 40},
		}

		for key, user := range testData {
			err := userCache.Set(suite.ctx, key, user)
			suite.Require().NoError(err, "Should not return error")
		}

		// Get all keys
		keys, err := userCache.Keys(suite.ctx)
		suite.Require().NoError(err, "Should not return error")
		suite.Require().Len(keys, 3, "Length should match expected value")

		suite.Contains(keys, "user:1", "Should contain expected value")
		suite.Contains(keys, "user:2", "Should contain expected value")
		suite.Contains(keys, "admin:100", "Should contain expected value")

		for _, key := range keys {
			suite.NotContains(key, "vef:cache:", "Should not contain unexpected value")
			suite.NotContains(key, "key-stripping-test:", "Should not contain unexpected value")
		}
	})

	suite.Run("KeysWithPrefixReturnUserOriginalKeys", func() {
		keys, err := userCache.Keys(suite.ctx, "user")
		suite.Require().NoError(err, "Should not return error")
		suite.Require().Len(keys, 2, "Length should match expected value")

		suite.Contains(keys, "user:1", "Should contain expected value")
		suite.Contains(keys, "user:2", "Should contain expected value")

		suite.NotContains(keys, "admin:100", "Should not contain unexpected value")

		for _, key := range keys {
			suite.NotContains(key, "vef:cache:", "Should not contain unexpected value")
		}
	})

	suite.Run("ForEachReturnsUserOriginalKeys", func() {
		collected := make(map[string]TestUser)

		err := userCache.ForEach(suite.ctx, func(key string, user TestUser) bool {
			collected[key] = user

			return true
		})
		suite.Require().NoError(err, "Should not return error")

		suite.Contains(collected, "user:1", "Should contain expected value")
		suite.Contains(collected, "user:2", "Should contain expected value")
		suite.Contains(collected, "admin:100", "Should contain expected value")

		for key := range collected {
			suite.NotContains(key, "vef:cache:", "Should not contain unexpected value")
			suite.NotContains(key, "key-stripping-test:", "Should not contain unexpected value")
		}
	})

	suite.Run("ForEachWithPrefixReturnsUserOriginalKeys", func() {
		collected := make(map[string]TestUser)

		err := userCache.ForEach(suite.ctx, func(key string, user TestUser) bool {
			collected[key] = user

			return true
		}, "admin")
		suite.Require().NoError(err, "Should not return error")

		suite.Require().Len(collected, 1, "Length should match expected value")
		suite.Contains(collected, "admin:100", "Should contain expected value")

		for key := range collected {
			suite.NotContains(key, "vef:cache:", "Should not contain unexpected value")
		}
	})
}

func (suite *RedisCacheTestSuite) TestRedisCacheKeyStrippingEdgeCases() {
	suite.Run("SingleLevelNamespace", func() {
		cache := suite.setupRedisCache("simple")
		defer cache.Close()

		user := TestUser{ID: 1, Name: "Alice", Age: 30}
		err := cache.Set(suite.ctx, "test-key", user)
		suite.Require().NoError(err, "Should not return error")

		keys, err := cache.Keys(suite.ctx)
		suite.Require().NoError(err, "Should not return error")
		suite.Require().Len(keys, 1, "Length should match expected value")

		suite.Equal("test-key", keys[0], "Key should not contain internal prefix")
		suite.NotContains(keys[0], "vef:cache:", "Should not contain unexpected value")
		suite.NotContains(keys[0], "simple:", "Should not contain unexpected value")
	})

	suite.Run("KeysWithColonInUserKey", func() {
		cache := suite.setupRedisCache("colon-test")
		defer cache.Close()

		complexKey := "namespace:subnamespace:item:123"
		user := TestUser{ID: 123, Name: "Complex", Age: 35}

		err := cache.Set(suite.ctx, complexKey, user)
		suite.Require().NoError(err, "Should not return error")

		keys, err := cache.Keys(suite.ctx)
		suite.Require().NoError(err, "Should not return error")
		suite.Require().Len(keys, 1, "Length should match expected value")

		suite.Equal(complexKey, keys[0], "Complex key should be preserved")
	})

	suite.Run("KeysWithSpecialCharacters", func() {
		cache := suite.setupRedisCache("special-chars")
		defer cache.Close()

		specialKeys := []string{
			"user@domain.com",
			"path/to/resource",
			"key-with-dashes",
			"key_with_underscores",
			"key.with.dots",
		}

		for i, key := range specialKeys {
			user := TestUser{ID: i + 1, Name: key, Age: 20 + i}
			err := cache.Set(suite.ctx, key, user)
			suite.Require().NoError(err, "Should not return error")
		}

		keys, err := cache.Keys(suite.ctx)
		suite.Require().NoError(err, "Should not return error")
		suite.Require().Len(keys, len(specialKeys), "Length should match expected value")

		for _, expectedKey := range specialKeys {
			suite.Contains(keys, expectedKey, "Should contain special key: %s", expectedKey)
		}
	})

	suite.Run("KeyStrippingDoesNotAffectGetAndSet", func() {
		cache := suite.setupRedisCache("get-set-test")
		defer cache.Close()

		userKey := "my-user-key"
		user := TestUser{ID: 999, Name: "TestUser", Age: 50}

		err := cache.Set(suite.ctx, userKey, user)
		suite.Require().NoError(err, "Should not return error")

		retrieved, found := cache.Get(suite.ctx, userKey)
		suite.True(found, "Should find user by key")
		suite.Equal(user, retrieved, "Retrieved user should match")

		keys, err := cache.Keys(suite.ctx)
		suite.Require().NoError(err, "Should not return error")
		suite.Require().Len(keys, 1, "Length should match expected value")
		suite.Equal(userKey, keys[0], "Key should match original user key")
	})
}

// TestRedisCacheTestSuite tests redis cache test suite functionality.
func TestRedisCache(t *testing.T) {
	suite.Run(t, new(RedisCacheTestSuite))
}
