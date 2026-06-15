package api_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/api"
)

// TestAuthConfigClone covers the deep-copy semantics of AuthConfig.Clone,
// including the nil receiver and Options-map independence.
func TestAuthConfigClone(t *testing.T) {
	t.Run("NilReceiver", func(t *testing.T) {
		var c *api.AuthConfig

		assert.Nil(t, c.Clone(), "cloning a nil config should yield nil")
	})

	t.Run("NilOptions", func(t *testing.T) {
		original := &api.AuthConfig{Strategy: api.AuthStrategyBearer}

		clone := original.Clone()
		require.NotNil(t, clone, "clone should not be nil")
		assert.Equal(t, api.AuthStrategyBearer, clone.Strategy, "strategy should be copied")
		assert.Nil(t, clone.Options, "nil Options should remain nil after clone")
	})

	t.Run("PopulatedOptionsAreIndependent", func(t *testing.T) {
		original := &api.AuthConfig{
			Strategy: api.AuthStrategySignature,
			Options:  map[string]any{"key": "value", "ttl": 30},
		}

		clone := original.Clone()
		require.NotNil(t, clone, "clone should not be nil")
		assert.Equal(t, original.Strategy, clone.Strategy, "strategy should be copied")
		assert.Equal(t, original.Options, clone.Options, "Options contents should be equal after clone")

		// Mutating the clone's map must not affect the original.
		clone.Options["key"] = "mutated"
		clone.Options["new"] = true
		assert.Equal(t, "value", original.Options["key"], "original Options must not change when clone is mutated")
		assert.NotContains(t, original.Options, "new", "original Options must not gain keys added to the clone")

		// Mutating the original's map must not affect the clone.
		original.Options["key2"] = "added"
		assert.NotContains(t, clone.Options, "key2", "clone Options must not gain keys added to the original")
	})

	t.Run("StrategyMutationIsIndependent", func(t *testing.T) {
		original := &api.AuthConfig{Strategy: api.AuthStrategyBearer}

		clone := original.Clone()
		clone.Strategy = api.AuthStrategyNone
		assert.Equal(t, api.AuthStrategyBearer, original.Strategy, "mutating the clone's strategy must not affect the original")
	})
}

// TestAuthConfigConstructors verifies the strategy each public constructor sets.
func TestAuthConfigConstructors(t *testing.T) {
	tests := []struct {
		name string
		cfg  *api.AuthConfig
		want string
	}{
		{"Public", api.Public(), api.AuthStrategyNone},
		{"BearerAuth", api.BearerAuth(), api.AuthStrategyBearer},
		{"SignatureAuth", api.SignatureAuth(), api.AuthStrategySignature},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.NotNil(t, tc.cfg, "constructor should return a non-nil config")
			assert.Equal(t, tc.want, tc.cfg.Strategy, "constructor should set the expected strategy")
		})
	}
}
