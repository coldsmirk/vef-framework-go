package sequence

import (
	"context"
	"sync"

	"github.com/coldsmirk/go-collections"

	"github.com/coldsmirk/vef-framework-go/timex"
)

// MemoryStore implements Store using in-memory storage.
// Suitable for single-instance deployments, development, and testing.
type MemoryStore struct {
	rules collections.ConcurrentMap[string, *Rule]
	locks collections.ConcurrentMap[string, *sync.Mutex]
}

// NewMemoryStore creates a new in-memory sequence store.
func NewMemoryStore() Store {
	return &MemoryStore{
		rules: collections.NewConcurrentHashMap[string, *Rule](),
		locks: collections.NewConcurrentHashMap[string, *sync.Mutex](),
	}
}

// Register preloads rules into the memory store.
// Existing rules with the same key will be overwritten.
// A deep copy of each rule is stored to prevent external mutation.
func (s *MemoryStore) Register(rules ...*Rule) {
	for _, rule := range rules {
		s.rules.Put(rule.Key, rule.Clone())
	}
}

func (s *MemoryStore) Reserve(_ context.Context, key string, count int, now timex.DateTime) (*Rule, int, error) {
	mu, _ := s.locks.GetOrCompute(key, func() *sync.Mutex { return new(sync.Mutex) })

	mu.Lock()
	defer mu.Unlock()

	rule, ok := s.rules.Get(key)
	if !ok || !rule.IsActive {
		return nil, 0, ErrRuleNotFound
	}

	resetNeeded, err := evaluateReserve(rule, count, now)
	if err != nil {
		return nil, 0, err
	}

	if resetNeeded {
		rule.CurrentValue = rule.StartValue
		resetAt := now
		rule.LastResetAt = &resetAt
	}

	rule.CurrentValue += rule.SeqStep * count

	return rule.Clone(), rule.CurrentValue, nil
}
