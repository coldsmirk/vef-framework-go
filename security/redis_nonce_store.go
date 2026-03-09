package security

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisNoncePrefix = "vef:security:nonce:"

// RedisNonceStore implements NonceStore using Redis for distributed deployments.
type RedisNonceStore struct {
	client *redis.Client
}

// NewRedisNonceStore creates a new Redis-backed nonce store.
func NewRedisNonceStore(client *redis.Client) NonceStore {
	return &RedisNonceStore{client: client}
}

func (*RedisNonceStore) buildKey(appID, nonce string) string {
	return redisNoncePrefix + appID + ":" + nonce
}

// StoreIfAbsent atomically stores the nonce only when it does not exist.
func (s *RedisNonceStore) StoreIfAbsent(ctx context.Context, appID, nonce string, ttl time.Duration) (bool, error) {
	key := s.buildKey(appID, nonce)

	err := s.client.SetArgs(ctx, key, "1", redis.SetArgs{
		Mode: "NX",
		TTL:  ttl,
	}).Err()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}

	return err == nil, err
}
