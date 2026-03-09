package security

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/redis/go-redis/v9"

	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/result"
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

func (s *RedisChallengeTokenStore) Generate(principal *Principal, pending, resolved []string) (string, error) {
	token := id.GenerateUUID()
	state := ChallengeState{Principal: principal, Pending: pending, Resolved: resolved}

	data, err := json.Marshal(state)
	if err != nil {
		return "", err
	}

	key := redisChallengePrefix + token
	if err := s.client.Set(context.Background(), key, data, ChallengeTokenExpires).Err(); err != nil {
		return "", err
	}

	return token, nil
}

func (s *RedisChallengeTokenStore) Parse(token string) (*ChallengeState, error) {
	if token == "" {
		return nil, result.ErrTokenInvalid
	}

	key := redisChallengePrefix + token

	data, err := s.client.Get(context.Background(), key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, result.ErrTokenInvalid
		}

		return nil, err
	}

	var state ChallengeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, result.ErrTokenInvalid
	}

	return &state, nil
}
