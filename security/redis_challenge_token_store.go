package security

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/redis/go-redis/v9"

	"github.com/coldsmirk/vef-framework-go/id"
)

const redisChallengePrefix = "vef:security:challenge:"

// RedisChallengeTokenStore implements ChallengeTokenStore using Redis for distributed deployments.
type RedisChallengeTokenStore struct {
	client *redis.Client
}

// NewRedisChallengeTokenStore creates a new Redis-backed challenge token store.
func NewRedisChallengeTokenStore(client *redis.Client) ChallengeTokenStore {
	return &RedisChallengeTokenStore{client: client}
}

func (s *RedisChallengeTokenStore) Generate(ctx context.Context, principal *Principal, pending, resolved []string) (string, error) {
	token := id.GenerateUUID()
	state := ChallengeState{Principal: principal, Pending: pending, Resolved: resolved}

	data, err := json.Marshal(state)
	if err != nil {
		return "", err
	}

	key := redisChallengePrefix + token
	if err := s.client.Set(ctx, key, data, ChallengeTokenExpires).Err(); err != nil {
		return "", err
	}

	return token, nil
}

func (s *RedisChallengeTokenStore) Parse(ctx context.Context, token string) (*ChallengeState, error) {
	if token == "" {
		return nil, ErrTokenInvalid
	}

	key := redisChallengePrefix + token

	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrTokenInvalid
		}

		return nil, err
	}

	var state ChallengeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, ErrTokenInvalid
	}

	return &state, nil
}
