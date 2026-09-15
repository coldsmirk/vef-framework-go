package security

import (
	"context"
	"slices"

	"github.com/coldsmirk/vef-framework-go/cache"
	"github.com/coldsmirk/vef-framework-go/id"
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

func (s *MemoryChallengeTokenStore) Generate(ctx context.Context, state *ChallengeState) (string, error) {
	token := id.GenerateUUID()

	if err := s.cache.Set(ctx, token, detachChallengeState(state), ChallengeTokenExpires); err != nil {
		return "", err
	}

	return token, nil
}

func (s *MemoryChallengeTokenStore) Parse(ctx context.Context, token string) (*ChallengeState, error) {
	if token == "" {
		return nil, ErrTokenInvalid
	}

	cached, ok := s.cache.Get(ctx, token)
	if !ok {
		return nil, ErrTokenInvalid
	}

	state := detachChallengeState(&cached)

	return &state, nil
}

// detachChallengeState copies state onto slices of its own. The cache holds
// values rather than serialized bytes, so without the copy the cached state and
// the one handed in or out would share backing arrays: the login flow appending
// a resolved challenge, or a second resolve racing on the same token, would
// write through into the other.
func detachChallengeState(state *ChallengeState) ChallengeState {
	detached := *state
	detached.Resolved = slices.Clone(state.Resolved)
	detached.Pending = slices.Clone(state.Pending)

	return detached
}
