package mapx

import (
	"cmp"
	"math"
	"reflect"
	"testing"

	"github.com/coldsmirk/go-collections"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertSliceToCollectionSetHappyPath covers the four set-family
// interfaces for a representative element type and confirms that JSON arrays
// land in concrete sets with the expected elements.
func TestConvertSliceToCollectionSetHappyPath(t *testing.T) {
	t.Run("SetString", func(t *testing.T) {
		var target struct {
			Tags collections.Set[string] `json:"tags"`
		}

		input := map[string]any{"tags": []any{"a", "b", "a"}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		require.NoError(t, decoder.Decode(input), "Decoding should succeed")

		require.NotNil(t, target.Tags, "Set should be non-nil")
		assert.Equal(t, 2, target.Tags.Size(), "Duplicates should be deduped")
		assert.True(t, target.Tags.Contains("a"), "Element 'a' should be present")
		assert.True(t, target.Tags.Contains("b"), "Element 'b' should be present")
	})

	t.Run("SortedSetString", func(t *testing.T) {
		var target struct {
			Tags collections.SortedSet[string] `json:"tags"`
		}

		input := map[string]any{"tags": []any{"banana", "apple", "cherry"}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		require.NoError(t, decoder.Decode(input), "Decoding should succeed")

		require.NotNil(t, target.Tags, "SortedSet should be non-nil")
		assert.Equal(t, []string{"apple", "banana", "cherry"}, target.Tags.ToSlice(),
			"SortedSet iteration should be ordered")
	})

	t.Run("ConcurrentSetString", func(t *testing.T) {
		var target struct {
			Tags collections.ConcurrentSet[string] `json:"tags"`
		}

		input := map[string]any{"tags": []any{"x", "y"}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		require.NoError(t, decoder.Decode(input), "Decoding should succeed")

		require.NotNil(t, target.Tags, "ConcurrentSet should be non-nil")
		assert.Equal(t, 2, target.Tags.Size(), "ConcurrentSet should hold both elements")
	})

	t.Run("ConcurrentSortedSetString", func(t *testing.T) {
		var target struct {
			Tags collections.ConcurrentSortedSet[string] `json:"tags"`
		}

		input := map[string]any{"tags": []any{"z", "a", "m"}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		require.NoError(t, decoder.Decode(input), "Decoding should succeed")

		require.NotNil(t, target.Tags, "ConcurrentSortedSet should be non-nil")
		assert.Equal(t, []string{"a", "m", "z"}, target.Tags.ToSlice(),
			"ConcurrentSortedSet iteration should be ordered")
	})
}

// TestConvertSliceToCollectionSetNumericTypes verifies that all numeric T
// (int family, uint family, float family) work end-to-end via decoder, and
// that JSON-style float64 inputs are accepted when the value is integral and
// fits the target width.
func TestConvertSliceToCollectionSetNumericTypes(t *testing.T) {
	t.Run("SetInt", func(t *testing.T) {
		var target struct {
			IDs collections.Set[int] `json:"ids"`
		}

		// JSON numbers decode as float64; ensure integral float64 -> int works.
		input := map[string]any{"ids": []any{float64(1), float64(2), float64(2)}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		require.NoError(t, decoder.Decode(input), "Decoding should succeed")

		assert.Equal(t, 2, target.IDs.Size(), "Duplicates should be deduped")
		assert.True(t, target.IDs.Contains(1), "Element 1 should be present")
		assert.True(t, target.IDs.Contains(2), "Element 2 should be present")
	})

	t.Run("SetInt64FromTypedSlice", func(t *testing.T) {
		// Direct []int64 source (e.g. internal callers) must also work.
		var target struct {
			IDs collections.Set[int64] `json:"ids"`
		}

		input := map[string]any{"ids": []int64{10, 20, 30}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		require.NoError(t, decoder.Decode(input), "Decoding should succeed")

		assert.Equal(t, 3, target.IDs.Size(), "All distinct elements should be present")
	})

	t.Run("SetUint16WithinBounds", func(t *testing.T) {
		var target struct {
			Codes collections.Set[uint16] `json:"codes"`
		}

		input := map[string]any{"codes": []any{float64(0), float64(65535)}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		require.NoError(t, decoder.Decode(input), "Decoding should succeed")

		assert.True(t, target.Codes.Contains(0), "Lower bound should be present")
		assert.True(t, target.Codes.Contains(65535), "Upper bound should be present")
	})

	t.Run("SetFloat32", func(t *testing.T) {
		var target struct {
			Vals collections.Set[float32] `json:"vals"`
		}

		input := map[string]any{"vals": []any{float64(1.5), float64(2.5)}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		require.NoError(t, decoder.Decode(input), "Decoding should succeed")

		assert.Equal(t, 2, target.Vals.Size(), "Both elements should be present")
	})
}

// TestConvertSliceToCollectionSetRejections covers numeric-safety boundaries
// that must produce errors rather than silently corrupting data.
func TestConvertSliceToCollectionSetRejections(t *testing.T) {
	t.Run("FractionalFloatToInt", func(t *testing.T) {
		var target struct {
			IDs collections.Set[int] `json:"ids"`
		}

		input := map[string]any{"ids": []any{float64(1.5)}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		assert.Error(t, decoder.Decode(input), "Fractional float should be rejected")
	})

	t.Run("OverflowFloatToInt8", func(t *testing.T) {
		var target struct {
			Vals collections.Set[int8] `json:"vals"`
		}

		input := map[string]any{"vals": []any{float64(300)}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		assert.Error(t, decoder.Decode(input), "Value 300 should overflow int8")
	})

	t.Run("NegativeFloatToUint", func(t *testing.T) {
		var target struct {
			Vals collections.Set[uint16] `json:"vals"`
		}

		input := map[string]any{"vals": []any{float64(-1)}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		assert.Error(t, decoder.Decode(input), "Negative value should be rejected for uint")
	})

	t.Run("NegativeIntToUint", func(t *testing.T) {
		var target struct {
			Vals collections.Set[uint32] `json:"vals"`
		}

		input := map[string]any{"vals": []int{-5}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		assert.Error(t, decoder.Decode(input), "Negative int should be rejected for uint target")
	})

	t.Run("NaNToInt", func(t *testing.T) {
		var target struct {
			Vals collections.Set[int] `json:"vals"`
		}

		input := map[string]any{"vals": []any{math.NaN()}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		assert.Error(t, decoder.Decode(input), "NaN should be rejected")
	})

	t.Run("InfinityToInt", func(t *testing.T) {
		var target struct {
			Vals collections.Set[int] `json:"vals"`
		}

		input := map[string]any{"vals": []any{math.Inf(1)}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		assert.Error(t, decoder.Decode(input), "+Inf should be rejected")
	})

	t.Run("StringElementToIntSet", func(t *testing.T) {
		var target struct {
			IDs collections.Set[int] `json:"ids"`
		}

		input := map[string]any{"ids": []any{"abc"}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		assert.Error(t, decoder.Decode(input), "String element should not coerce to int")
	})

	t.Run("NumericElementToStringSet", func(t *testing.T) {
		var target struct {
			Tags collections.Set[string] `json:"tags"`
		}

		input := map[string]any{"tags": []any{float64(1)}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		assert.Error(t, decoder.Decode(input), "Numeric element should not coerce to string")
	})

	t.Run("NilElement", func(t *testing.T) {
		var target struct {
			Tags collections.Set[string] `json:"tags"`
		}

		input := map[string]any{"tags": []any{nil}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		assert.Error(t, decoder.Decode(input), "Nil element should be rejected")
	})

	// Float64 boundary against int64. math.MaxInt64 cannot be exactly
	// represented in float64; float64(math.MaxInt64) rounds up to 2^63.
	// A naive `f > math.MaxInt64` check therefore lets f == 2^63 slip
	// through, after which `int64(f)` is implementation-defined and
	// silently produces MaxInt64. The fix uses an exclusive upper bound
	// against the next representable float (2^63).
	t.Run("Float64Pow63ToInt64Overflow", func(t *testing.T) {
		var target struct {
			Vals collections.Set[int64] `json:"vals"`
		}

		input := map[string]any{"vals": []any{math.Pow(2, 63)}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		assert.Error(t, decoder.Decode(input), "2^63 must overflow int64")
	})

	// Same boundary for uint64: math.MaxUint64 rounds up to 2^64 in
	// float64, and `f > MaxUint64` would miss f == 2^64.
	t.Run("Float64Pow64ToUint64Overflow", func(t *testing.T) {
		var target struct {
			Vals collections.Set[uint64] `json:"vals"`
		}

		input := map[string]any{"vals": []any{math.Pow(2, 64)}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		assert.Error(t, decoder.Decode(input), "2^64 must overflow uint64")
	})

	// MinInt64 (-2^63) is exactly representable in float64 and must
	// continue to round-trip cleanly after the boundary fix.
	t.Run("Float64MinInt64Accepted", func(t *testing.T) {
		var target struct {
			Vals collections.Set[int64] `json:"vals"`
		}

		input := map[string]any{"vals": []any{float64(math.MinInt64)}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		require.NoError(t, decoder.Decode(input), "MinInt64 must round-trip")
		assert.True(t, target.Vals.Contains(math.MinInt64), "MinInt64 should be present")
	})
}

// TestConvertSliceToCollectionSetEdgeCases covers benign edge cases that must
// continue to work and inputs that should fall through unchanged.
func TestConvertSliceToCollectionSetEdgeCases(t *testing.T) {
	t.Run("EmptyArray", func(t *testing.T) {
		var target struct {
			Tags collections.Set[string] `json:"tags"`
		}

		input := map[string]any{"tags": []any{}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		require.NoError(t, decoder.Decode(input), "Empty array should decode cleanly")

		require.NotNil(t, target.Tags, "Empty Set should still be initialized")
		assert.Equal(t, 0, target.Tags.Size(), "Set should be empty")
	})

	t.Run("UnregisteredTargetFallsThrough", func(t *testing.T) {
		// []string -> []string is not handled by this hook and must be
		// preserved by the rest of the chain (mapstructure default).
		var target struct {
			Tags []string `json:"tags"`
		}

		input := map[string]any{"tags": []any{"a", "b"}}

		decoder, err := NewDecoder(&target)
		require.NoError(t, err, "Decoder construction should succeed")
		require.NoError(t, decoder.Decode(input), "Plain []string target should still decode")

		assert.Equal(t, []string{"a", "b"}, target.Tags, "Slice content should be preserved")
	})

	t.Run("NonSliceSourceFallsThrough", func(t *testing.T) {
		// String source for a string-typed Set field is invalid input by
		// our contract, but our hook should not be triggered (from is not
		// slice/array). We expect mapstructure to fail decoding.
		from := reflect.TypeFor[string]()
		to := reflect.TypeFor[collections.Set[string]]()

		out, err := convertSliceToCollectionSet(from, to, "abc")
		require.NoError(t, err, "Hook should not error on non-slice source")
		assert.Equal(t, "abc", out, "Non-slice source must pass through unchanged")
	})
}

// probeRegistry verifies that each of the four set-family interfaces for a
// given element type T is registered AND that the registered builder returns
// a value implementing the expected interface. This catches both omissions
// and mis-wiring (e.g. a HashSet builder bound to the SortedSet[T] key).
func probeRegistry[T cmp.Ordered](t *testing.T) {
	t.Helper()

	emptySource := reflect.ValueOf([]any{})
	families := []struct {
		name string
		typ  reflect.Type
	}{
		{"Set", reflect.TypeFor[collections.Set[T]]()},
		{"SortedSet", reflect.TypeFor[collections.SortedSet[T]]()},
		{"ConcurrentSet", reflect.TypeFor[collections.ConcurrentSet[T]]()},
		{"ConcurrentSortedSet", reflect.TypeFor[collections.ConcurrentSortedSet[T]]()},
	}

	elemType := reflect.TypeFor[T]()

	for _, f := range families {
		builder, ok := collectionSetBuilders[f.typ]
		require.True(t, ok, "%s[%s] must be registered", f.name, elemType)

		out, err := builder(emptySource)
		require.NoError(t, err, "%s[%s] builder must accept empty source", f.name, elemType)
		assert.True(t, reflect.TypeOf(out).Implements(f.typ),
			"%s[%s] builder returned %T which does not implement %s",
			f.name, elemType, out, f.typ)
	}
}

// TestRegistryCoverage asserts that every (interface family × supported T)
// pair is wired into the registry AND that each registered builder returns
// a value of the right interface type. Element types T are enumerated
// explicitly so a missing registerCollectionSet call shows up as a concrete
// failing assertion instead of a silent gap that the size check might miss.
func TestRegistryCoverage(t *testing.T) {
	probeRegistry[string](t)
	probeRegistry[int](t)
	probeRegistry[int8](t)
	probeRegistry[int16](t)
	probeRegistry[int32](t)
	probeRegistry[int64](t)
	probeRegistry[uint](t)
	probeRegistry[uint8](t)
	probeRegistry[uint16](t)
	probeRegistry[uint32](t)
	probeRegistry[uint64](t)
	probeRegistry[float32](t)
	probeRegistry[float64](t)

	// 4 families × 13 element types = 52 entries.
	assert.Equal(t, 52, len(collectionSetBuilders),
		"Registry size should match families × supported element types")
}
