package security

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisTrustCodePrefix = "vef:security:trust_code:"

// RedisTrustCodeStore implements TrustCodeStore using Redis, so a code issued
// on one replica can be redeemed on any other. Single use rides on GETDEL,
// which reads and deletes in one round trip (Redis 6.2+).
type RedisTrustCodeStore struct {
	client *redis.Client
}

// NewRedisTrustCodeStore creates a new Redis-backed trust code store.
func NewRedisTrustCodeStore(client *redis.Client) TrustCodeStore {
	return &RedisTrustCodeStore{client: client}
}

func (*RedisTrustCodeStore) buildKey(code string) string {
	return redisTrustCodePrefix + HashOpaqueToken(code)
}

func (s *RedisTrustCodeStore) Issue(ctx context.Context, state TrustCodeState, ttl time.Duration) (string, error) {
	code, err := GenerateOpaqueToken()
	if err != nil {
		return "", err
	}

	payload, err := json.Marshal(state)
	if err != nil {
		return "", err
	}

	if err := s.client.Set(ctx, s.buildKey(code), payload, ttl).Err(); err != nil {
		return "", err
	}

	return code, nil
}

func (s *RedisTrustCodeStore) Consume(ctx context.Context, code string) (*TrustCodeState, error) {
	if code == "" {
		return nil, ErrTrustCodeInvalid
	}

	payload, err := s.client.GetDel(ctx, s.buildKey(code)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrTrustCodeInvalid
	}

	if err != nil {
		return nil, err
	}

	var state TrustCodeState
	if err := json.Unmarshal(payload, &state); err != nil {
		return nil, err
	}

	return &state, nil
}
