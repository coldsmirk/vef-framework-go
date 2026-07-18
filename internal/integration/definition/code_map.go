package definition

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"sync"

	"github.com/coldsmirk/vef-framework-go/hashx"
	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/lru"
)

// codeSetPattern bounds code set identifiers to a script- and tag-friendly
// charset: alphanumeric edges with _.- in between.
var codeSetPattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9_.-]*[A-Za-z0-9])?$`)

// maxCodeSetLength matches the code_set column width.
const maxCodeSetLength = 128

// codeMapCacheCapacity bounds the compiled lookup index cache; maps beyond it
// evict least recently used and rebuild on next use.
const codeMapCacheCapacity = 256

// ValidateCodeMap rejects a code map whose identifier, unmapped policy, or
// entries would make lookups fail or turn non-deterministic at runtime.
// Building the lookup index is the entry validation: it normalizes every
// value and rejects per-side collisions.
func ValidateCodeMap(m *integration.CodeMap) error {
	if len(m.CodeSet) > maxCodeSetLength || !codeSetPattern.MatchString(m.CodeSet) {
		return integration.ErrInvalidCodeMap("code set must match ^[A-Za-z0-9]([A-Za-z0-9_.-]*[A-Za-z0-9])?$ with at most 128 characters")
	}

	if !m.OnUnmapped.IsValid() {
		return integration.ErrInvalidCodeMap(fmt.Sprintf("unknown unmapped policy %q", m.OnUnmapped))
	}

	if err := validateFallbacks(m); err != nil {
		return err
	}

	if _, err := BuildCodeMapIndex(m); err != nil {
		return integration.ErrInvalidCodeMap(err.Error())
	}

	return nil
}

// validateFallbacks ties the fallback values to the fallback policy: the
// policy requires both sides (each direction needs a defined result), any
// other policy must not carry them (they would be dead configuration).
func validateFallbacks(m *integration.CodeMap) error {
	if m.OnUnmapped == integration.UnmappedPolicyFallback {
		if m.FallbackCanonical == nil || m.FallbackExternal == nil {
			return integration.ErrInvalidCodeMap("the fallback policy requires both fallback values")
		}

		for _, fallback := range []any{m.FallbackCanonical, m.FallbackExternal} {
			if _, err := NormalizeCodeValue(fallback); err != nil {
				return integration.ErrInvalidCodeMap(fmt.Sprintf("fallback value: %v", err))
			}
		}

		return nil
	}

	if m.FallbackCanonical != nil || m.FallbackExternal != nil {
		return integration.ErrInvalidCodeMap("fallback values require the fallback policy")
	}

	return nil
}

// NormalizeCodeValue folds a JSON scalar into the canonical string form
// lookups compare by, so 1 (number) and "1" (string) address the same entry
// regardless of how the value arrived (jsonb, API JSON, or a script value —
// goja exports integral numbers as int64).
func NormalizeCodeValue(v any) (string, error) {
	switch value := v.(type) {
	case string:
		if value == "" {
			return "", ErrEmptyCodeValue
		}

		return value, nil

	case bool:
		return strconv.FormatBool(value), nil
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64), nil
	case int64:
		return strconv.FormatInt(value, 10), nil
	default:
		return "", fmt.Errorf("%w, got %T", ErrNonScalarCodeValue, v)
	}
}

// CodeMapIndex is a code map compiled for constant-time bidirectional lookup:
// each side's primary and aliases fold into one normalized-string key space
// pointing at the opposite side's primary value.
type CodeMapIndex struct {
	entries           []integration.CodeMapEntry
	policy            integration.UnmappedPolicy
	fallbackCanonical any
	fallbackExternal  any
	external          map[string]any
	canonical         map[string]any
}

// BuildCodeMapIndex compiles m's entries into the bidirectional lookup index,
// rejecting non-scalar values and per-side collisions — a value reachable
// through two entries would make lookups order-dependent.
func BuildCodeMapIndex(m *integration.CodeMap) (*CodeMapIndex, error) {
	idx := &CodeMapIndex{
		entries:           m.Entries,
		policy:            m.OnUnmapped,
		fallbackCanonical: m.FallbackCanonical,
		fallbackExternal:  m.FallbackExternal,
		external:          make(map[string]any),
		canonical:         make(map[string]any),
	}

	for i, entry := range m.Entries {
		if err := indexSide(idx.external, entry.External, entry.Canonical, entry.CanonicalAliases); err != nil {
			return nil, fmt.Errorf("entry %d canonical side: %w", i+1, err)
		}

		if err := indexSide(idx.canonical, entry.Canonical, entry.External, entry.ExternalAliases); err != nil {
			return nil, fmt.Errorf("entry %d external side: %w", i+1, err)
		}
	}

	return idx, nil
}

// indexSide folds one side's primary and aliases into the lookup table that
// yields target for each of them.
func indexSide(table map[string]any, target, primary any, aliases []any) error {
	for _, value := range append([]any{primary}, aliases...) {
		key, err := NormalizeCodeValue(value)
		if err != nil {
			return err
		}

		if _, exists := table[key]; exists {
			return fmt.Errorf("%w: %q", ErrDuplicateCodeValue, key)
		}

		table[key] = target
	}

	return nil
}

// ExternalFor returns the external primary value the normalized canonical-side
// key maps to.
func (idx *CodeMapIndex) ExternalFor(key string) (any, bool) {
	value, ok := idx.external[key]

	return value, ok
}

// CanonicalFor returns the canonical primary value the normalized
// external-side key maps to.
func (idx *CodeMapIndex) CanonicalFor(key string) (any, bool) {
	value, ok := idx.canonical[key]

	return value, ok
}

// Entries returns the mapping pairs the index was built from.
func (idx *CodeMapIndex) Entries() []integration.CodeMapEntry {
	return idx.entries
}

// Policy returns the map's unmapped policy, an empty value resolved to reject
// (fail closed).
func (idx *CodeMapIndex) Policy() integration.UnmappedPolicy {
	if idx.policy == "" {
		return integration.UnmappedPolicyReject
	}

	return idx.policy
}

// FallbackCanonical returns the fallback value for the canonical side.
func (idx *CodeMapIndex) FallbackCanonical() any {
	return idx.fallbackCanonical
}

// FallbackExternal returns the fallback value for the external side.
func (idx *CodeMapIndex) FallbackExternal() any {
	return idx.fallbackExternal
}

// CodeMapIndexCache caches compiled lookup indexes keyed by the content that
// affects lookup semantics, so editing a code map invalidates its entry
// implicitly and unchanged maps never rebuild.
type CodeMapIndexCache struct {
	mu    sync.Mutex
	cache *lru.Cache[*CodeMapIndex]
}

// NewCodeMapIndexCache creates an empty compiled-index cache.
func NewCodeMapIndexCache() *CodeMapIndexCache {
	return &CodeMapIndexCache{cache: lru.New[*CodeMapIndex](codeMapCacheCapacity)}
}

// Get returns the compiled index for m, building and caching it on first
// sight.
func (c *CodeMapIndexCache) Get(m *integration.CodeMap) (*CodeMapIndex, error) {
	content, err := json.Marshal(struct {
		Entries           []integration.CodeMapEntry `json:"entries"`
		Policy            integration.UnmappedPolicy `json:"policy"`
		FallbackCanonical any                        `json:"fallbackCanonical"`
		FallbackExternal  any                        `json:"fallbackExternal"`
	}{m.Entries, m.OnUnmapped, m.FallbackCanonical, m.FallbackExternal})
	if err != nil {
		return nil, err
	}

	key := hashx.SHA256(string(content))

	c.mu.Lock()
	defer c.mu.Unlock()

	if idx, ok := c.cache.Get(key); ok {
		return idx, nil
	}

	idx, err := BuildCodeMapIndex(m)
	if err != nil {
		return nil, err
	}

	c.cache.Put(key, idx)

	return idx, nil
}
