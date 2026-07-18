package exec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/integration"
	"github.com/coldsmirk/vef-framework-go/internal/integration/definition"
)

func TestParseUnmappedOverride(t *testing.T) {
	t.Run("NoOptions", func(t *testing.T) {
		override, err := parseUnmappedOverride(nil)
		require.NoError(t, err, "absent options are valid")
		assert.Nil(t, override, "absent options yield no override")
	})

	t.Run("Fallback", func(t *testing.T) {
		override, err := parseUnmappedOverride([]map[string]any{{"fallback": "U"}})
		require.NoError(t, err, "a fallback option is valid")
		assert.Equal(t, integration.UnmappedPolicyFallback, override.policy, "fallback selects the fallback policy")
		assert.Equal(t, "U", override.fallback, "the fallback value is carried")
	})

	t.Run("PassthroughAndReject", func(t *testing.T) {
		override, err := parseUnmappedOverride([]map[string]any{{"passthrough": true}})
		require.NoError(t, err, "passthrough: true is valid")
		assert.Equal(t, integration.UnmappedPolicyPassthrough, override.policy, "passthrough selects the passthrough policy")

		override, err = parseUnmappedOverride([]map[string]any{{"reject": true}})
		require.NoError(t, err, "reject: true is valid")
		assert.Equal(t, integration.UnmappedPolicyReject, override.policy, "reject selects the reject policy")
	})

	t.Run("InvalidShapes", func(t *testing.T) {
		_, err := parseUnmappedOverride([]map[string]any{{}, {}})
		require.Error(t, err, "two options objects are rejected")

		_, err = parseUnmappedOverride([]map[string]any{{}})
		require.Error(t, err, "an empty options object is rejected")

		_, err = parseUnmappedOverride([]map[string]any{{"fallback": "U", "passthrough": true}})
		require.Error(t, err, "two option keys are rejected")

		_, err = parseUnmappedOverride([]map[string]any{{"passthrough": false}})
		require.Error(t, err, "passthrough: false is rejected")

		_, err = parseUnmappedOverride([]map[string]any{{"reject": "yes"}})
		require.Error(t, err, "a non-boolean reject is rejected")

		_, err = parseUnmappedOverride([]map[string]any{{"bogus": true}})
		require.Error(t, err, "an unknown option key is rejected")
	})
}

func TestUnmappedResult(t *testing.T) {
	buildIndex := func(policy integration.UnmappedPolicy, fallbackCanonical, fallbackExternal any) *definition.CodeMapIndex {
		idx, err := definition.BuildCodeMapIndex(&integration.CodeMap{
			OnUnmapped:        policy,
			FallbackCanonical: fallbackCanonical,
			FallbackExternal:  fallbackExternal,
		})
		require.NoError(t, err, "the test index should build")

		return idx
	}

	t.Run("DefaultRejects", func(t *testing.T) {
		_, err := unmappedResult(buildIndex("", nil, nil), nil, "gender", "X", true)
		require.Error(t, err, "an empty policy rejects")

		var codeMapErr *codeMapError
		require.ErrorAs(t, err, &codeMapErr, "the reject raises a code map fault")
		assert.ErrorIs(t, codeMapErr.apiErr, integration.ErrUnmappedValue("gender", "X"), "the reject surfaces the unmapped-value error")
	})

	t.Run("PassthroughReturnsInput", func(t *testing.T) {
		value, err := unmappedResult(buildIndex(integration.UnmappedPolicyPassthrough, nil, nil), nil, "gender", "X", true)
		require.NoError(t, err, "passthrough succeeds")
		assert.Equal(t, "X", value, "passthrough returns the input unchanged")
	})

	t.Run("FallbackPicksTheTargetSide", func(t *testing.T) {
		idx := buildIndex(integration.UnmappedPolicyFallback, "0", "U")

		value, err := unmappedResult(idx, nil, "gender", "X", true)
		require.NoError(t, err, "fallback succeeds")
		assert.Equal(t, "U", value, "toExternal falls back to the external value")

		value, err = unmappedResult(idx, nil, "gender", "X", false)
		require.NoError(t, err, "fallback succeeds")
		assert.Equal(t, "0", value, "toCanonical falls back to the canonical value")
	})

	t.Run("OverrideBeatsStoredPolicy", func(t *testing.T) {
		idx := buildIndex(integration.UnmappedPolicyPassthrough, nil, nil)

		value, err := unmappedResult(idx, &unmappedOverride{policy: integration.UnmappedPolicyFallback, fallback: "Z"}, "gender", "X", true)
		require.NoError(t, err, "the override fallback succeeds")
		assert.Equal(t, "Z", value, "the per-call fallback value wins")

		_, err = unmappedResult(idx, &unmappedOverride{policy: integration.UnmappedPolicyReject}, "gender", "X", true)
		require.Error(t, err, "a reject override beats a lenient stored policy")
	})
}
