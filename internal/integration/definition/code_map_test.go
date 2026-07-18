package definition

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coldsmirk/vef-framework-go/integration"
)

// validCodeMap builds a well-formed map the invalid cases mutate.
func validCodeMap() *integration.CodeMap {
	return &integration.CodeMap{
		SystemID: "sys-1",
		CodeSet:  "gender",
		Name:     "Gender",
		Entries: []integration.CodeMapEntry{
			{Canonical: "1", External: "M", ExternalAliases: []any{"Male", "m"}},
			{Canonical: "2", External: "F"},
			{Canonical: "0", External: "U", CanonicalAliases: []any{"9"}},
		},
	}
}

func TestNormalizeCodeValue(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		want    string
		wantErr bool
	}{
		{name: "String", value: "M", want: "M"},
		{name: "EmptyStringRejected", value: "", wantErr: true},
		{name: "BoolTrue", value: true, want: "true"},
		{name: "BoolFalse", value: false, want: "false"},
		{name: "IntegralFloat", value: float64(1), want: "1"},
		{name: "FractionalFloat", value: 1.5, want: "1.5"},
		{name: "Int64", value: int64(7), want: "7"},
		{name: "NilRejected", value: nil, wantErr: true},
		{name: "ObjectRejected", value: map[string]any{"a": 1}, wantErr: true},
		{name: "ArrayRejected", value: []any{"a"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeCodeValue(tt.value)
			if tt.wantErr {
				require.Error(t, err, "normalization should reject the value")

				return
			}

			require.NoError(t, err, "normalization should accept the value")
			assert.Equal(t, tt.want, got, "normalized form should match")
		})
	}
}

func TestValidateCodeMap(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(m *integration.CodeMap)
		wantErr bool
	}{
		{name: "Valid", mutate: func(*integration.CodeMap) {}},
		{name: "EmptyEntriesAllowed", mutate: func(m *integration.CodeMap) { m.Entries = nil }},
		{name: "DottedAndDashedCodeSet", mutate: func(m *integration.CodeMap) { m.CodeSet = "his.gender-x_1" }},
		{
			name: "FallbackPolicyWithBothValues",
			mutate: func(m *integration.CodeMap) {
				m.OnUnmapped = integration.UnmappedPolicyFallback
				m.FallbackCanonical = "0"
				m.FallbackExternal = "U"
			},
		},
		{name: "EmptyCodeSetRejected", mutate: func(m *integration.CodeMap) { m.CodeSet = "" }, wantErr: true},
		{name: "EdgeDashCodeSetRejected", mutate: func(m *integration.CodeMap) { m.CodeSet = "gender-" }, wantErr: true},
		{name: "WhitespaceCodeSetRejected", mutate: func(m *integration.CodeMap) { m.CodeSet = "gen der" }, wantErr: true},
		{
			name:    "OverlongCodeSetRejected",
			mutate:  func(m *integration.CodeMap) { m.CodeSet = strings.Repeat("a", 129) },
			wantErr: true,
		},
		{name: "UnknownPolicyRejected", mutate: func(m *integration.CodeMap) { m.OnUnmapped = "ignore" }, wantErr: true},
		{
			name: "FallbackPolicyMissingValueRejected",
			mutate: func(m *integration.CodeMap) {
				m.OnUnmapped = integration.UnmappedPolicyFallback
				m.FallbackCanonical = "0"
			},
			wantErr: true,
		},
		{
			name: "FallbackValuesWithoutPolicyRejected",
			mutate: func(m *integration.CodeMap) {
				m.OnUnmapped = integration.UnmappedPolicyReject
				m.FallbackExternal = "U"
			},
			wantErr: true,
		},
		{
			name: "NonScalarFallbackRejected",
			mutate: func(m *integration.CodeMap) {
				m.OnUnmapped = integration.UnmappedPolicyFallback
				m.FallbackCanonical = map[string]any{"v": 1}
				m.FallbackExternal = "U"
			},
			wantErr: true,
		},
		{
			name: "DuplicateCanonicalPrimaryRejected",
			mutate: func(m *integration.CodeMap) {
				m.Entries = append(m.Entries, integration.CodeMapEntry{Canonical: "1", External: "X"})
			},
			wantErr: true,
		},
		{
			name: "AliasCollidingWithPrimaryRejected",
			mutate: func(m *integration.CodeMap) {
				m.Entries = append(m.Entries, integration.CodeMapEntry{
					Canonical: "3", External: "X", CanonicalAliases: []any{"1"},
				})
			},
			wantErr: true,
		},
		{
			name: "CrossTypeCollisionRejected",
			mutate: func(m *integration.CodeMap) {
				m.Entries = append(m.Entries, integration.CodeMapEntry{Canonical: float64(1), External: "X"})
			},
			wantErr: true,
		},
		{
			name: "NilCanonicalRejected",
			mutate: func(m *integration.CodeMap) {
				m.Entries = append(m.Entries, integration.CodeMapEntry{External: "X"})
			},
			wantErr: true,
		},
		{
			name: "EmptyExternalRejected",
			mutate: func(m *integration.CodeMap) {
				m.Entries = append(m.Entries, integration.CodeMapEntry{Canonical: "3", External: ""})
			},
			wantErr: true,
		},
		{
			name: "NonScalarAliasRejected",
			mutate: func(m *integration.CodeMap) {
				m.Entries = append(m.Entries, integration.CodeMapEntry{
					Canonical: "3", External: "X", ExternalAliases: []any{[]any{"nested"}},
				})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validCodeMap()
			tt.mutate(m)

			err := ValidateCodeMap(m)
			if tt.wantErr {
				require.ErrorIs(t, err, integration.ErrInvalidCodeMap(""), "validation should reject with the invalid-code-map error")

				return
			}

			require.NoError(t, err, "validation should accept the map")
		})
	}
}

func TestBuildCodeMapIndex(t *testing.T) {
	t.Run("BidirectionalPrimaryAndAliasLookup", func(t *testing.T) {
		idx, err := BuildCodeMapIndex(validCodeMap())
		require.NoError(t, err, "the valid map should build")

		external, ok := idx.ExternalFor("1")
		require.True(t, ok, "the canonical primary should match")
		assert.Equal(t, "M", external, "toExternal should emit the external primary")

		external, ok = idx.ExternalFor("9")
		require.True(t, ok, "a canonical alias should match")
		assert.Equal(t, "U", external, "an alias lookup should emit the primary, not the alias")

		canonical, ok := idx.CanonicalFor("Male")
		require.True(t, ok, "an external alias should match")
		assert.Equal(t, "1", canonical, "toCanonical should emit the canonical primary")

		canonical, ok = idx.CanonicalFor("U")
		require.True(t, ok, "the external primary should match")
		assert.Equal(t, "0", canonical, "the reverse of an aliased entry lands on the canonical primary")

		_, ok = idx.ExternalFor("Male")
		assert.False(t, ok, "external-side values must not match canonical-side lookups")
	})

	t.Run("NumericValuesKeepTheirType", func(t *testing.T) {
		idx, err := BuildCodeMapIndex(&integration.CodeMap{
			Entries: []integration.CodeMapEntry{{Canonical: "high", External: float64(1)}},
		})
		require.NoError(t, err, "the numeric map should build")

		external, ok := idx.ExternalFor("high")
		require.True(t, ok, "the canonical primary should match")
		assert.Equal(t, float64(1), external, "the emitted value should keep its JSON number type")

		canonical, ok := idx.CanonicalFor("1")
		require.True(t, ok, "a numeric external should be addressable by normalized string form")
		assert.Equal(t, "high", canonical, "the numeric entry should translate back")
	})

	t.Run("PolicyDefaultsToReject", func(t *testing.T) {
		idx, err := BuildCodeMapIndex(validCodeMap())
		require.NoError(t, err, "the valid map should build")
		assert.Equal(t, integration.UnmappedPolicyReject, idx.Policy(), "an empty policy resolves to reject")

		withPolicy := validCodeMap()
		withPolicy.OnUnmapped = integration.UnmappedPolicyPassthrough

		idx, err = BuildCodeMapIndex(withPolicy)
		require.NoError(t, err, "the map with an explicit policy should build")
		assert.Equal(t, integration.UnmappedPolicyPassthrough, idx.Policy(), "an explicit policy is kept")
	})
}

func TestCodeMapIndexCache(t *testing.T) {
	cache := NewCodeMapIndexCache()

	t.Run("SameContentSharesTheIndex", func(t *testing.T) {
		first, err := cache.Get(validCodeMap())
		require.NoError(t, err, "the first build should succeed")

		second, err := cache.Get(validCodeMap())
		require.NoError(t, err, "the cached read should succeed")
		assert.Same(t, first, second, "identical content should hit the same compiled index")
	})

	t.Run("EditedContentRebuilds", func(t *testing.T) {
		first, err := cache.Get(validCodeMap())
		require.NoError(t, err, "the first build should succeed")

		edited := validCodeMap()
		edited.Entries[0].External = "MALE"

		second, err := cache.Get(edited)
		require.NoError(t, err, "the edited map should build")
		assert.NotSame(t, first, second, "changed content must invalidate implicitly")
	})

	t.Run("InvalidContentFails", func(t *testing.T) {
		invalid := validCodeMap()
		invalid.Entries = append(invalid.Entries, integration.CodeMapEntry{Canonical: "1", External: "X"})

		_, err := cache.Get(invalid)
		require.Error(t, err, "a colliding map must not produce an index")
	})
}
