package monad

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewRange tests the NewRange constructor with different types.
func TestNewRange(t *testing.T) {
	t.Run("IntRange", func(t *testing.T) {
		r := NewRange(1, 10)
		assert.Equal(t, 1, r.Start, "Should set Start to 1")
		assert.Equal(t, 10, r.End, "Should set End to 10")
	})

	t.Run("StringRange", func(t *testing.T) {
		r := NewRange("a", "z")
		assert.Equal(t, "a", r.Start, "Should set Start to 'a'")
		assert.Equal(t, "z", r.End, "Should set End to 'z'")
	})

	t.Run("FloatRange", func(t *testing.T) {
		r := NewRange(1.5, 9.5)
		assert.Equal(t, 1.5, r.Start, "Should set Start to 1.5")
		assert.Equal(t, 9.5, r.End, "Should set End to 9.5")
	})
}

// TestRangeContains tests the Contains method for boundary conditions.
func TestRangeContains(t *testing.T) {
	tests := []struct {
		name     string
		r        Range[int]
		value    int
		expected bool
	}{
		{"ValueInRange", NewRange(1, 10), 5, true},
		{"ValueAtStart", NewRange(1, 10), 1, true},
		{"ValueAtEnd", NewRange(1, 10), 10, true},
		{"ValueBelowRange", NewRange(1, 10), 0, false},
		{"ValueAboveRange", NewRange(1, 10), 11, false},
		{"SingleValueRangeContains", NewRange(5, 5), 5, true},
		{"SingleValueRangeNotContains", NewRange(5, 5), 4, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.r.Contains(tt.value), "Should return expected Contains result")
		})
	}
}

// TestRangeContainsString tests the Contains method with string values.
func TestRangeContainsString(t *testing.T) {
	strRange := NewRange("b", "y")

	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"BelowRange", "a", false},
		{"AtStart", "b", true},
		{"InMiddle", "m", true},
		{"AtEnd", "y", true},
		{"AboveRange", "z", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, strRange.Contains(tt.value), "Should return expected Contains result")
		})
	}
}

// TestRangeContainsFloat tests the Contains method with float values.
func TestRangeContainsFloat(t *testing.T) {
	floatRange := NewRange(1.5, 9.5)

	tests := []struct {
		name     string
		value    float64
		expected bool
	}{
		{"BelowRange", 1.0, false},
		{"AtStart", 1.5, true},
		{"InMiddle", 5.25, true},
		{"AtEnd", 9.5, true},
		{"AboveRange", 10.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, floatRange.Contains(tt.value), "Should return expected Contains result")
		})
	}
}

// TestRangeIsValid tests the IsValid method for various range configurations.
func TestRangeIsValid(t *testing.T) {
	tests := []struct {
		name     string
		r        Range[int]
		expected bool
	}{
		{"ValidRange", NewRange(1, 10), true},
		{"SingleValueRange", NewRange(5, 5), true},
		{"InvalidRange", NewRange(10, 1), false},
		{"ZeroRange", NewRange(0, 0), true},
		{"NegativeRange", NewRange(-10, -1), true},
		{"InvalidNegativeRange", NewRange(-1, -10), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.r.IsValid(), "Should return expected IsValid result")
		})
	}
}

// TestRangeIsEmpty tests the IsEmpty method for various range configurations.
func TestRangeIsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		r        Range[int]
		expected bool
	}{
		{"ValidRange", NewRange(1, 10), false},
		{"SingleValueRange", NewRange(5, 5), false},
		{"EmptyRange", NewRange(10, 1), true},
		{"ZeroRange", NewRange(0, 0), false},
		{"EmptyNegativeRange", NewRange(-1, -10), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.r.IsEmpty(), "Should return expected IsEmpty result")
		})
	}
}

// TestRangeOverlaps tests range overlap detection including edge cases.
func TestRangeOverlaps(t *testing.T) {
	tests := []struct {
		name     string
		r1       Range[int]
		r2       Range[int]
		expected bool
	}{
		{"CompleteOverlap", NewRange(1, 10), NewRange(3, 7), true},
		{"PartialOverlap", NewRange(1, 5), NewRange(3, 8), true},
		{"AdjacentRanges", NewRange(1, 5), NewRange(5, 10), true},
		{"NoOverlap", NewRange(1, 3), NewRange(5, 10), false},
		{"ReverseOverlap", NewRange(5, 10), NewRange(1, 7), true},
		{"SameRange", NewRange(1, 10), NewRange(1, 10), true},
		{"SinglePointOverlap", NewRange(1, 5), NewRange(5, 5), true},
		{"NoOverlapWithGap", NewRange(1, 3), NewRange(6, 10), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.r1.Overlaps(tt.r2), "Should return expected Overlaps result")
			assert.Equal(t, tt.expected, tt.r2.Overlaps(tt.r1), "Should be symmetric")
		})
	}
}

// TestRangeIntersection tests intersection calculation with symmetry verification.
func TestRangeIntersection(t *testing.T) {
	tests := []struct {
		name     string
		r1       Range[int]
		r2       Range[int]
		expected Range[int]
		wantOK   bool
	}{
		{"CompleteOverlap", NewRange(1, 10), NewRange(3, 7), NewRange(3, 7), true},
		{"PartialOverlap", NewRange(1, 5), NewRange(3, 8), NewRange(3, 5), true},
		{"AdjacentRanges", NewRange(1, 5), NewRange(5, 10), NewRange(5, 5), true},
		{"NoOverlap", NewRange(1, 3), NewRange(5, 10), Range[int]{}, false},
		{"SameRange", NewRange(1, 10), NewRange(1, 10), NewRange(1, 10), true},
		{"ReverseOverlap", NewRange(5, 10), NewRange(1, 7), NewRange(5, 7), true},
		{"SinglePoint", NewRange(1, 5), NewRange(5, 5), NewRange(5, 5), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := tt.r1.Intersection(tt.r2)
			reverseResult, reverseOK := tt.r2.Intersection(tt.r1)

			assert.Equal(t, tt.wantOK, ok, "Should report expected overlap")
			assert.Equal(t, tt.wantOK, reverseOK, "Should report symmetric overlap")

			if tt.wantOK {
				assert.Equal(t, tt.expected, result, "Should return expected intersection")
				assert.Equal(t, tt.expected, reverseResult, "Should return symmetric intersection")
			} else {
				assert.Equal(t, Range[int]{}, result, "Should return zero range when there is no overlap")
				assert.Equal(t, Range[int]{}, reverseResult, "Should return symmetric zero range when there is no overlap")
			}
		})
	}
}

// TestRangeIntersectionString tests intersection calculation with string ranges.
func TestRangeIntersectionString(t *testing.T) {
	t.Run("OverlappingStringRanges", func(t *testing.T) {
		intersection, ok := NewRange("c", "m").Intersection(NewRange("f", "z"))
		assert.True(t, ok, "Should report overlap for intersecting string ranges")
		assert.Equal(t, NewRange("f", "m"), intersection, "Should return correct string intersection")
	})

	t.Run("NonOverlappingStringRanges", func(t *testing.T) {
		noOverlap, ok := NewRange("a", "b").Intersection(NewRange("x", "z"))
		assert.False(t, ok, "Should report no overlap for non-overlapping string ranges")
		assert.Equal(t, Range[string]{}, noOverlap, "Should return zero range for non-overlapping string ranges")
	})
}

// TestRangeJSONMarshaling tests JSON marshaling and unmarshaling.
func TestRangeJSONMarshaling(t *testing.T) {
	r := NewRange(5, 15)

	data, err := json.Marshal(r)
	require.NoError(t, err, "Should marshal without error")

	var result Range[int]

	err = json.Unmarshal(data, &result)
	require.NoError(t, err, "Should unmarshal without error")

	assert.Equal(t, r, result, "Should round-trip through JSON")
}

// TestRangeWithDifferentTypes tests range operations with various numeric types.
func TestRangeWithDifferentTypes(t *testing.T) {
	t.Run("Int8Range", func(t *testing.T) {
		assert.True(t, NewRange[int8](1, 10).Contains(5), "Should contain value in int8 range")
	})

	t.Run("Int16Range", func(t *testing.T) {
		assert.True(t, NewRange[int16](100, 200).Contains(150), "Should contain value in int16 range")
	})

	t.Run("Int32Range", func(t *testing.T) {
		assert.True(t, NewRange[int32](1000, 2000).Contains(1500), "Should contain value in int32 range")
	})

	t.Run("Int64Range", func(t *testing.T) {
		assert.True(t, NewRange[int64](10000, 20000).Contains(15000), "Should contain value in int64 range")
	})

	t.Run("Uint8Range", func(t *testing.T) {
		assert.True(t, NewRange[uint8](10, 250).Contains(100), "Should contain value in uint8 range")
	})

	t.Run("Uint16Range", func(t *testing.T) {
		assert.True(t, NewRange[uint16](1000, 50000).Contains(25000), "Should contain value in uint16 range")
	})

	t.Run("Uint32Range", func(t *testing.T) {
		assert.True(t, NewRange[uint32](100000, 500000).Contains(300000), "Should contain value in uint32 range")
	})

	t.Run("Uint64Range", func(t *testing.T) {
		assert.True(t, NewRange[uint64](1000000, 5000000).Contains(3000000), "Should contain value in uint64 range")
	})

	t.Run("Float32Range", func(t *testing.T) {
		assert.True(t, NewRange[float32](1.1, 9.9).Contains(5.5), "Should contain value in float32 range")
	})

	t.Run("Float64Range", func(t *testing.T) {
		assert.True(t, NewRange(1.1, 9.9).Contains(5.5), "Should contain value in float64 range")
	})
}

// TestRangeEdgeCases tests edge cases including maximum values and negative ranges.
func TestRangeEdgeCases(t *testing.T) {
	t.Run("MaximumUint8Value", func(t *testing.T) {
		maxRange := NewRange[uint8](0, 255)
		assert.True(t, maxRange.IsValid(), "Should be valid for max uint8 range")
		assert.True(t, maxRange.Contains(255), "Should contain max uint8 value")
	})

	t.Run("NegativeRangeContains", func(t *testing.T) {
		negRange := NewRange(-100, -10)
		assert.True(t, negRange.IsValid(), "Should be valid for negative range")
		assert.True(t, negRange.Contains(-50), "Should contain value in negative range")
		assert.False(t, negRange.Contains(-5), "Should not contain value outside negative range")
	})

	t.Run("NegativeRangeIntersection", func(t *testing.T) {
		r1 := NewRange(-10, 0)
		r2 := NewRange(-5, 5)
		assert.True(t, r1.Overlaps(r2), "Should detect overlap in negative ranges")

		intersection, ok := r1.Intersection(r2)
		assert.True(t, ok, "Should report overlap in negative ranges")
		assert.Equal(t, NewRange(-5, 0), intersection, "Should return correct negative intersection")
	})
}

// TestRangeStringOperations tests comprehensive string range operations.
func TestRangeStringOperations(t *testing.T) {
	t.Run("StringRangeContains", func(t *testing.T) {
		r := NewRange("apple", "orange")
		assert.True(t, r.Contains("mango"), "Should contain 'mango' in [apple, orange]")
		assert.False(t, r.Contains("pear"), "Should not contain 'pear' in [apple, orange]")
	})

	t.Run("OverlappingStringRanges", func(t *testing.T) {
		r1 := NewRange("apple", "orange")
		r2 := NewRange("banana", "zebra")
		assert.True(t, r1.Overlaps(r2), "Should detect overlap in string ranges")

		intersection, ok := r1.Intersection(r2)
		assert.True(t, ok, "Should report overlap in string ranges")
		assert.Equal(t, "banana", intersection.Start, "Should have correct intersection Start")
		assert.Equal(t, "orange", intersection.End, "Should have correct intersection End")
	})

	t.Run("NonOverlappingStringRanges", func(t *testing.T) {
		r3 := NewRange("aaa", "bbb")
		r4 := NewRange("yyy", "zzz")
		assert.False(t, r3.Overlaps(r4), "Should not overlap for distant string ranges")

		_, ok := r3.Intersection(r4)
		assert.False(t, ok, "Should report no overlap for non-overlapping string ranges")
	})
}
