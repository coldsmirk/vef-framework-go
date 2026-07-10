package mapx

import (
	"cmp"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/coldsmirk/go-collections"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeWithDefaults decodes input into out through NewDecoder with the
// package default options, failing the test on decoder construction errors.
func decodeWithDefaults(t *testing.T, input map[string]any, out any) error {
	t.Helper()

	decoder, err := NewDecoder(out)
	require.NoError(t, err, "Decoder construction should succeed")

	return decoder.Decode(input)
}

// assertNoJSONNumber walks a decoded value tree (maps and slices) and fails
// the test if any json.Number survived decoding.
func assertNoJSONNumber(t *testing.T, value any, path string) {
	t.Helper()

	switch v := value.(type) {
	case json.Number:
		t.Errorf("json.Number leaked at %s: %v", path, v)
	case map[string]any:
		for key, elem := range v {
			assertNoJSONNumber(t, elem, path+"."+key)
		}
	case []any:
		for i, elem := range v {
			assertNoJSONNumber(t, elem, fmt.Sprintf("%s[%d]", path, i))
		}
	}
}

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
		assert.Error(t, decoder.Decode(input), "Positive infinity should be rejected")
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
		assert.Error(t, decoder.Decode(input), "Value 2^63 must overflow int64")
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
		assert.Error(t, decoder.Decode(input), "Value 2^64 must overflow uint64")
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

// probeRegistry verifies that each set-family interface for a given element
// type T is registered AND that the registered builder returns a value
// implementing the expected interface. This catches both omissions and
// mis-wiring (e.g. a HashSet builder bound to the SortedSet[T] key). It returns
// the number of family entries it verified so callers can derive the expected
// total registry size instead of hard-coding it.
func probeRegistry[T cmp.Ordered](t *testing.T) int {
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
		require.True(t, ok, "Registry entry %s[%s] must be registered", f.name, elemType)

		out, err := builder(emptySource)
		require.NoError(t, err, "Registry builder %s[%s] must accept empty source", f.name, elemType)
		assert.True(t, reflect.TypeOf(out).Implements(f.typ),
			"Registry builder %s[%s] should return %T implementing %s",
			f.name, elemType, out, f.typ)
	}

	return len(families)
}

// TestRegistryCoverage asserts that every (interface family × supported T)
// pair is wired into the registry AND that each registered builder returns
// a value of the right interface type. Element types T are enumerated
// explicitly so a missing registerCollectionSet call shows up as a concrete
// failing assertion instead of a silent gap that the size check might miss.
func TestRegistryCoverage(t *testing.T) {
	verified := probeRegistry[string](t) +
		probeRegistry[int](t) +
		probeRegistry[int8](t) +
		probeRegistry[int16](t) +
		probeRegistry[int32](t) +
		probeRegistry[int64](t) +
		probeRegistry[uint](t) +
		probeRegistry[uint8](t) +
		probeRegistry[uint16](t) +
		probeRegistry[uint32](t) +
		probeRegistry[uint64](t) +
		probeRegistry[float32](t) +
		probeRegistry[float64](t)

	// The registry must hold exactly the verified (family × element type) pairs
	// and nothing more, so any unverified extra entry is caught here.
	assert.Equal(t, verified, len(collectionSetBuilders),
		"Registry size should match the verified families × supported element types")
}

// TestConvertJSONNumberTypedTargets covers json.Number conversion into every
// typed target the hook handles explicitly: all int/uint kinds, both float
// kinds, json.Number identity, pointer targets, and named numeric types.
func TestConvertJSONNumberTypedTargets(t *testing.T) {
	t.Run("IntegerKinds", func(t *testing.T) {
		var target struct {
			I   int   `json:"i"`
			I8  int8  `json:"i8"`
			I16 int16 `json:"i16"`
			I32 int32 `json:"i32"`
			I64 int64 `json:"i64"`
		}

		input := map[string]any{
			"i":   json.Number("-7"),
			"i8":  json.Number("127"),
			"i16": json.Number("-32768"),
			"i32": json.Number("2147483647"),
			"i64": json.Number("9007199254740993"),
		}

		require.NoError(t, decodeWithDefaults(t, input, &target), "Integer decoding should succeed")
		assert.Equal(t, -7, target.I, "int should decode exactly")
		assert.Equal(t, int8(127), target.I8, "int8 boundary should decode exactly")
		assert.Equal(t, int16(-32768), target.I16, "int16 boundary should decode exactly")
		assert.Equal(t, int32(2147483647), target.I32, "int32 boundary should decode exactly")
		assert.Equal(t, int64(9007199254740993), target.I64, "int64 beyond 2^53 must keep exact digits")
	})

	t.Run("UnsignedKinds", func(t *testing.T) {
		var target struct {
			U   uint   `json:"u"`
			U8  uint8  `json:"u8"`
			U16 uint16 `json:"u16"`
			U32 uint32 `json:"u32"`
			U64 uint64 `json:"u64"`
		}

		input := map[string]any{
			"u":   json.Number("7"),
			"u8":  json.Number("255"),
			"u16": json.Number("65535"),
			"u32": json.Number("4294967295"),
			"u64": json.Number("18446744073709551615"),
		}

		require.NoError(t, decodeWithDefaults(t, input, &target), "Unsigned decoding should succeed")
		assert.Equal(t, uint(7), target.U, "uint should decode exactly")
		assert.Equal(t, uint8(255), target.U8, "uint8 boundary should decode exactly")
		assert.Equal(t, uint16(65535), target.U16, "uint16 boundary should decode exactly")
		assert.Equal(t, uint32(4294967295), target.U32, "uint32 boundary should decode exactly")
		assert.Equal(t, uint64(18446744073709551615), target.U64, "uint64 max must keep exact digits")
	})

	t.Run("FloatKinds", func(t *testing.T) {
		var target struct {
			F32 float32 `json:"f32"`
			F64 float64 `json:"f64"`
			Int float64 `json:"int"`
		}

		input := map[string]any{
			"f32": json.Number("1.5"),
			"f64": json.Number("2.25"),
			"int": json.Number("3"),
		}

		require.NoError(t, decodeWithDefaults(t, input, &target), "Float decoding should succeed")
		assert.Equal(t, float32(1.5), target.F32, "float32 should decode exactly")
		assert.Equal(t, 2.25, target.F64, "float64 should decode exactly")
		assert.Equal(t, 3.0, target.Int, "integer form into float target should decode")
	})

	t.Run("JSONNumberIdentity", func(t *testing.T) {
		var target struct {
			V json.Number `json:"v"`
		}

		input := map[string]any{"v": json.Number("9007199254740993")}

		require.NoError(t, decodeWithDefaults(t, input, &target), "json.Number identity decoding should succeed")
		assert.Equal(t, json.Number("9007199254740993"), target.V, "json.Number field must keep the literal digits")
	})

	t.Run("PointerTargets", func(t *testing.T) {
		var target struct {
			I64 *int64       `json:"i64"`
			F64 *float64     `json:"f64"`
			Num *json.Number `json:"num"`
		}

		input := map[string]any{
			"i64": json.Number("9007199254740993"),
			"f64": json.Number("1.5"),
			"num": json.Number("42"),
		}

		require.NoError(t, decodeWithDefaults(t, input, &target), "Pointer decoding should succeed")
		require.NotNil(t, target.I64, "*int64 should be set")
		assert.Equal(t, int64(9007199254740993), *target.I64, "*int64 beyond 2^53 must keep exact digits")
		require.NotNil(t, target.F64, "*float64 should be set")
		assert.Equal(t, 1.5, *target.F64, "*float64 should decode exactly")
		require.NotNil(t, target.Num, "*json.Number should be set")
		assert.Equal(t, json.Number("42"), *target.Num, "*json.Number should keep the literal")
	})

	t.Run("NamedNumericTypes", func(t *testing.T) {
		type TestPriority int8

		var target struct {
			P TestPriority `json:"p"`
		}

		input := map[string]any{"p": json.Number("5")}

		require.NoError(t, decodeWithDefaults(t, input, &target), "Named numeric type decoding should succeed")
		assert.Equal(t, TestPriority(5), target.P, "named int kind should decode exactly")
	})

	t.Run("AnyTargetIsFloat64", func(t *testing.T) {
		var target struct {
			Fraction any `json:"fraction"`
			BigInt   any `json:"bigInt"`
		}

		input := map[string]any{
			"fraction": json.Number("1.5"),
			"bigInt":   json.Number("9007199254740993"),
		}

		require.NoError(t, decodeWithDefaults(t, input, &target), "any-target decoding should succeed")
		assert.Equal(t, 1.5, target.Fraction, "any target must receive float64, not json.Number")
		// Untyped targets keep the pre-json.Number float64 contract, so
		// integers beyond 2^53 remain lossy there by design.
		assert.Equal(t, 9007199254740992.0, target.BigInt, "any target keeps the float64 contract even when lossy")
	})
}

// TestConvertJSONNumberErrors covers rejection paths: fraction/exponent into
// integer targets, overflow of every numeric width, negatives into unsigned,
// and the preserved mismatch errors for string/bool targets.
func TestConvertJSONNumberErrors(t *testing.T) {
	t.Run("Sentinels", func(t *testing.T) {
		tests := []struct {
			name    string
			decode  func(map[string]any) error
			input   map[string]any
			wantErr error
		}{
			{
				name: "FractionIntoInt",
				decode: func(in map[string]any) error {
					var target struct {
						V int `json:"v"`
					}

					return decodeWithDefaults(t, in, &target)
				},
				input:   map[string]any{"v": json.Number("1.5")},
				wantErr: ErrJSONNumberNotInteger,
			},
			{
				name: "ExponentIntoInt",
				decode: func(in map[string]any) error {
					var target struct {
						V int `json:"v"`
					}

					return decodeWithDefaults(t, in, &target)
				},
				input:   map[string]any{"v": json.Number("1e3")},
				wantErr: ErrJSONNumberNotInteger,
			},
			{
				name: "OverflowInt8",
				decode: func(in map[string]any) error {
					var target struct {
						V int8 `json:"v"`
					}

					return decodeWithDefaults(t, in, &target)
				},
				input:   map[string]any{"v": json.Number("300")},
				wantErr: ErrJSONNumberOverflow,
			},
			{
				name: "OverflowInt64",
				decode: func(in map[string]any) error {
					var target struct {
						V int64 `json:"v"`
					}

					return decodeWithDefaults(t, in, &target)
				},
				input:   map[string]any{"v": json.Number("9223372036854775808")},
				wantErr: ErrJSONNumberOverflow,
			},
			{
				name: "NegativeIntoUint",
				decode: func(in map[string]any) error {
					var target struct {
						V uint `json:"v"`
					}

					return decodeWithDefaults(t, in, &target)
				},
				input:   map[string]any{"v": json.Number("-1")},
				wantErr: ErrJSONNumberOverflow,
			},
			{
				name: "FractionIntoUint",
				decode: func(in map[string]any) error {
					var target struct {
						V uint `json:"v"`
					}

					return decodeWithDefaults(t, in, &target)
				},
				input:   map[string]any{"v": json.Number("1.5")},
				wantErr: ErrJSONNumberNotInteger,
			},
			{
				name: "OverflowUint8",
				decode: func(in map[string]any) error {
					var target struct {
						V uint8 `json:"v"`
					}

					return decodeWithDefaults(t, in, &target)
				},
				input:   map[string]any{"v": json.Number("300")},
				wantErr: ErrJSONNumberOverflow,
			},
			{
				name: "OverflowFloat32",
				decode: func(in map[string]any) error {
					var target struct {
						V float32 `json:"v"`
					}

					return decodeWithDefaults(t, in, &target)
				},
				input:   map[string]any{"v": json.Number("3.5e38")},
				wantErr: ErrJSONNumberOverflow,
			},
			{
				name: "OverflowFloat64",
				decode: func(in map[string]any) error {
					var target struct {
						V float64 `json:"v"`
					}

					return decodeWithDefaults(t, in, &target)
				},
				input:   map[string]any{"v": json.Number("1e400")},
				wantErr: ErrJSONNumberOverflow,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.decode(tt.input)
				require.Error(t, err, "Decoding should fail")
				assert.ErrorIs(t, err, tt.wantErr, "Error should wrap the expected sentinel")
			})
		}
	})

	t.Run("StringTargetRejected", func(t *testing.T) {
		var target struct {
			V string `json:"v"`
		}

		err := decodeWithDefaults(t, map[string]any{"v": json.Number("42")}, &target)
		require.Error(t, err, "A JSON number must not silently widen into a string field")
		// The hook surfaces float64 for mismatched targets so the failure is
		// identical to the pre-json.Number behavior.
		assert.ErrorContains(t, err, "float64", "Mismatch error should mention float64, matching the old contract")
	})

	t.Run("BoolTargetRejected", func(t *testing.T) {
		var target struct {
			V bool `json:"v"`
		}

		err := decodeWithDefaults(t, map[string]any{"v": json.Number("1")}, &target)
		require.Error(t, err, "A JSON number must not convert into a bool field")
		assert.ErrorContains(t, err, "float64", "Mismatch error should mention float64, matching the old contract")
	})
}

// TestConvertJSONNumberNestedNormalization proves that no json.Number leaks
// into untyped targets regardless of nesting depth, and that normalization
// never mutates the source map.
func TestConvertJSONNumberNestedNormalization(t *testing.T) {
	buildForm := func() map[string]any {
		return map[string]any{
			"amount": json.Number("1.5"),
			"detail": map[string]any{
				"qty":  json.Number("2"),
				"deep": map[string]any{"n": json.Number("9007199254740993")},
			},
			"rows": []any{
				map[string]any{"n": json.Number("3")},
				json.Number("4"),
				[]any{json.Number("5")},
			},
			"note": "text",
		}
	}

	t.Run("MapStringAnyFieldDeep", func(t *testing.T) {
		var target struct {
			FormData map[string]any `json:"formData"`
		}

		input := map[string]any{"formData": buildForm()}

		require.NoError(t, decodeWithDefaults(t, input, &target), "Nested decoding should succeed")
		assertNoJSONNumber(t, target.FormData, "formData")

		assert.Equal(t, 1.5, target.FormData["amount"], "top-level number should be float64")

		detail, ok := target.FormData["detail"].(map[string]any)
		require.True(t, ok, "nested map should stay map[string]any")
		assert.Equal(t, 2.0, detail["qty"], "nested map value should be float64")

		deep, ok := detail["deep"].(map[string]any)
		require.True(t, ok, "deep nested map should stay map[string]any")
		assert.Equal(t, 9007199254740992.0, deep["n"], "deep untyped value keeps the float64 contract")

		rows, ok := target.FormData["rows"].([]any)
		require.True(t, ok, "nested slice should stay []any")

		row0, ok := rows[0].(map[string]any)
		require.True(t, ok, "slice element map should stay map[string]any")
		assert.Equal(t, 3.0, row0["n"], "map inside slice should hold float64")
		assert.Equal(t, 4.0, rows[1], "scalar slice element should be float64")

		inner, ok := rows[2].([]any)
		require.True(t, ok, "nested slice element should stay []any")
		assert.Equal(t, 5.0, inner[0], "slice inside slice should hold float64")
	})

	t.Run("AnyFieldContainer", func(t *testing.T) {
		var target struct {
			V any `json:"v"`
		}

		input := map[string]any{"v": buildForm()}

		require.NoError(t, decodeWithDefaults(t, input, &target), "any-field decoding should succeed")
		assertNoJSONNumber(t, target.V, "v")
	})

	t.Run("AnySliceField", func(t *testing.T) {
		var target struct {
			V []any `json:"v"`
		}

		input := map[string]any{"v": []any{json.Number("1"), map[string]any{"n": json.Number("2")}}}

		require.NoError(t, decodeWithDefaults(t, input, &target), "[]any decoding should succeed")
		assertNoJSONNumber(t, target.V, "v")
		assert.Equal(t, 1.0, target.V[0], "slice scalar should be float64")
	})

	t.Run("SourceMapNotMutated", func(t *testing.T) {
		form := buildForm()
		input := map[string]any{"formData": form}

		var normalized struct {
			FormData map[string]any `json:"formData"`
		}

		require.NoError(t, decodeWithDefaults(t, input, &normalized), "First decode should succeed")

		detail := form["detail"].(map[string]any)
		deep := detail["deep"].(map[string]any)
		assert.Equal(t, json.Number("9007199254740993"), deep["n"],
			"normalization must operate on a copy, never mutate the source map")

		// A later raw capture from the SAME source must still see the exact digits.
		var captured struct {
			FormData json.RawMessage `json:"formData"`
		}

		require.NoError(t, decodeWithDefaults(t, input, &captured), "Second decode should succeed")
		assert.Contains(t, string(captured.FormData), "9007199254740993",
			"raw capture after a normalizing decode must keep exact digits")
	})
}

// TestConvertJSONNumberRawMessageCapture proves json.RawMessage targets
// re-marshal json.Number literals digit-exact, both for container values and
// for a number captured directly.
func TestConvertJSONNumberRawMessageCapture(t *testing.T) {
	t.Run("ContainerWithBigInt", func(t *testing.T) {
		var target struct {
			Payload json.RawMessage `json:"payload"`
		}

		input := map[string]any{
			"payload": map[string]any{"n": json.Number("9007199254740993")},
		}

		require.NoError(t, decodeWithDefaults(t, input, &target), "RawMessage decoding should succeed")
		assert.JSONEq(t, `{"n":9007199254740993}`, string(target.Payload), "captured JSON should be semantically equal")
		assert.Contains(t, string(target.Payload), "9007199254740993", "captured JSON must keep the exact digits")
	})

	t.Run("DirectNumber", func(t *testing.T) {
		var target struct {
			Payload json.RawMessage `json:"payload"`
		}

		input := map[string]any{"payload": json.Number("9007199254740993")}

		require.NoError(t, decodeWithDefaults(t, input, &target), "RawMessage decoding should succeed")
		assert.Equal(t, "9007199254740993", string(target.Payload), "a directly captured number must be the exact literal")
	})
}

// TestConvertSliceToCollectionSetJSONNumber covers json.Number elements
// flowing into collections set targets: exact integer parsing beyond 2^53,
// float elements, and the rejection paths (string sets, overflow, fraction).
func TestConvertSliceToCollectionSetJSONNumber(t *testing.T) {
	t.Run("SetInt64ExactBeyondFloat64", func(t *testing.T) {
		var target struct {
			IDs collections.Set[int64] `json:"ids"`
		}

		input := map[string]any{"ids": []any{json.Number("9007199254740993"), json.Number("2")}}

		require.NoError(t, decodeWithDefaults(t, input, &target), "Decoding should succeed")
		assert.Equal(t, 2, target.IDs.Size(), "Both elements should be present")
		assert.True(t, target.IDs.Contains(9007199254740993), "int64 element beyond 2^53 must keep exact digits")
	})

	t.Run("SetFloat64", func(t *testing.T) {
		var target struct {
			Values collections.Set[float64] `json:"values"`
		}

		input := map[string]any{"values": []any{json.Number("1.5"), json.Number("2")}}

		require.NoError(t, decodeWithDefaults(t, input, &target), "Decoding should succeed")
		assert.True(t, target.Values.Contains(1.5), "float element should be present")
		assert.True(t, target.Values.Contains(2.0), "integer-form element should be present")
	})

	t.Run("SetStringRejected", func(t *testing.T) {
		var target struct {
			Tags collections.Set[string] `json:"tags"`
		}

		input := map[string]any{"tags": []any{json.Number("42")}}

		err := decodeWithDefaults(t, input, &target)
		require.Error(t, err, "A JSON number must not silently become a string set element")
		assert.ErrorIs(t, err, ErrCollectionSetIncompatibleKind, "Error should wrap the incompatible-kind sentinel")
	})

	t.Run("SetUint8Overflow", func(t *testing.T) {
		var target struct {
			Values collections.Set[uint8] `json:"values"`
		}

		input := map[string]any{"values": []any{json.Number("300")}}

		err := decodeWithDefaults(t, input, &target)
		require.Error(t, err, "Overflowing element should fail")
		assert.ErrorIs(t, err, ErrJSONNumberOverflow, "Error should wrap the overflow sentinel")
	})

	t.Run("SetIntFraction", func(t *testing.T) {
		var target struct {
			Values collections.Set[int] `json:"values"`
		}

		input := map[string]any{"values": []any{json.Number("1.5")}}

		err := decodeWithDefaults(t, input, &target)
		require.Error(t, err, "Fractional element should fail")
		assert.ErrorIs(t, err, ErrJSONNumberNotInteger, "Error should wrap the non-integer sentinel")
	})
}
