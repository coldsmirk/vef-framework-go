package security

import (
	"context"

	"github.com/coldsmirk/vef-framework-go/cache"
	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/result"
)

// MemoryChallengeTokenStore implements ChallengeTokenStore using an in-memory cache.
// Suitable for single-instance deployments or testing; for distributed setups use Redis-backed stores.
type MemoryChallengeTokenStore struct {
	cache cache.Cache[ChallengeState]
}

// NewMemoryChallengeTokenStore creates a new memory-backed challenge token store.
func NewMemoryChallengeTokenStore() ChallengeTokenStore {
	return &MemoryChallengeTokenStore{
		cache: cache.NewMemory[ChallengeState](),
	}
}

func (s *MemoryChallengeTokenStore) Generate(principal *Principal, pending, resolved []string) (string, error) {
	token := id.GenerateUUID()

	state := ChallengeState{Principal: principal, Pending: pending, Resolved: resolved}
	if err := s.cache.Set(context.Background(), token, state, ChallengeTokenExpires); err != nil {
		return "", err
	}

	return token, nil
}

func (s *MemoryChallengeTokenStore) Parse(token string) (*ChallengeState, error) {
	if token == "" {
		return nil, result.ErrTokenInvalid
	}

	state, ok := s.cache.Get(context.Background(), token)
	if !ok {
		return nil, result.ErrTokenInvalid
	}

	return &state, nil
}
