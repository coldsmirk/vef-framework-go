package monad

import (
	"encoding/json"
	"testing"
)

// TestNewRange tests the NewRange constructor with different types.
func TestNewRange(t *testing.T) {
	t.Run("IntRange", func(t *testing.T) {
		r := NewRange(1, 10)
		if r.Start != 1 {
			t.Errorf("Expected Start to be 1, got %d", r.Start)
		}

		if r.End != 10 {
			t.Errorf("Expected End to be 10, got %d", r.End)
		}
	})

	t.Run("StringRange", func(t *testing.T) {
		r := NewRange("a", "z")
		if r.Start != "a" {
			t.Errorf("Expected Start to be 'a', got %s", r.Start)
		}

		if r.End != "z" {
			t.Errorf("Expected End to be 'z', got %s", r.End)
		}
	})

	t.Run("FloatRange", func(t *testing.T) {
		r := NewRange(1.5, 9.5)
		if r.Start != 1.5 {
			t.Errorf("Expected Start to be 1.5, got %f", r.Start)
		}

		if r.End != 9.5 {
			t.Errorf("Expected End to be 9.5, got %f", r.End)
		}
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
			result := tt.r.Contains(tt.value)
			if result != tt.expected {
				t.Errorf("Expected Contains(%d) to be %t, got %t", tt.value, tt.expected, result)
			}
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
			result := strRange.Contains(tt.value)
			if result != tt.expected {
				t.Errorf("Expected Contains(%s) to be %t, got %t", tt.value, tt.expected, result)
			}
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
			result := floatRange.Contains(tt.value)
			if result != tt.expected {
				t.Errorf("Expected Contains(%f) to be %t, got %t", tt.value, tt.expected, result)
			}
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
			result := tt.r.IsValid()
			if result != tt.expected {
				t.Errorf("Expected IsValid() to be %t, got %t", tt.expected, result)
			}
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
			result := tt.r.IsEmpty()
			if result != tt.expected {
				t.Errorf("Expected IsEmpty() to be %t, got %t", tt.expected, result)
			}
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
			result := tt.r1.Overlaps(tt.r2)
			if result != tt.expected {
				t.Errorf("Expected %v.Overlaps(%v) to be %t, got %t", tt.r1, tt.r2, tt.expected, result)
			}

			reverseResult := tt.r2.Overlaps(tt.r1)
			if reverseResult != tt.expected {
				t.Errorf("Expected symmetry: %v.Overlaps(%v) should also be %t, got %t", tt.r2, tt.r1, tt.expected, reverseResult)
			}
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
		isEmpty  bool
	}{
		{
			"CompleteOverlap",
			NewRange(1, 10),
			NewRange(3, 7),
			NewRange(3, 7),
			false,
		},
		{
			"PartialOverlap",
			NewRange(1, 5),
			NewRange(3, 8),
			NewRange(3, 5),
			false,
		},
		{
			"AdjacentRanges",
			NewRange(1, 5),
			NewRange(5, 10),
			NewRange(5, 5),
			false,
		},
		{
			"NoOverlap",
			NewRange(1, 3),
			NewRange(5, 10),
			NewRange(3, 1),
			true,
		},
		{
			"SameRange",
			NewRange(1, 10),
			NewRange(1, 10),
			NewRange(1, 10),
			false,
		},
		{
			"ReverseOverlap",
			NewRange(5, 10),
			NewRange(1, 7),
			NewRange(5, 7),
			false,
		},
		{
			"SinglePoint",
			NewRange(1, 5),
			NewRange(5, 5),
			NewRange(5, 5),
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.r1.Intersection(tt.r2)

			if tt.isEmpty {
				if !result.IsEmpty() {
					t.Errorf("Expected empty intersection, got %v", result)
				}
			} else {
				if result != tt.expected {
					t.Errorf("Expected intersection %v, got %v", tt.expected, result)
				}
			}

			reverseResult := tt.r2.Intersection(tt.r1)
			if tt.isEmpty {
				if !reverseResult.IsEmpty() {
					t.Errorf("Expected symmetric empty intersection, got %v", reverseResult)
				}
			} else {
				if reverseResult != tt.expected {
					t.Errorf("Expected symmetric intersection %v, got %v", tt.expected, reverseResult)
				}
			}
		})
	}
}

// TestRangeIntersectionString tests intersection calculation with string ranges.
func TestRangeIntersectionString(t *testing.T) {
	t.Run("OverlappingStringRanges", func(t *testing.T) {
		r1 := NewRange("c", "m")
		r2 := NewRange("f", "z")

		intersection := r1.Intersection(r2)
		expected := NewRange("f", "m")

		if intersection != expected {
			t.Errorf("Expected string intersection %v, got %v", expected, intersection)
		}
	})

	t.Run("NonOverlappingStringRanges", func(t *testing.T) {
		r3 := NewRange("a", "b")
		r4 := NewRange("x", "z")

		noOverlap := r3.Intersection(r4)
		if !noOverlap.IsEmpty() {
			t.Errorf("Expected empty intersection for non-overlapping string ranges, got %v", noOverlap)
		}
	})
}

// TestRangeJSONMarshaling tests JSON marshaling and unmarshaling.
func TestRangeJSONMarshaling(t *testing.T) {
	r := NewRange(5, 15)

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var result Range[int]

	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if result != r {
		t.Errorf("Expected %v, got %v", r, result)
	}
}

// TestRangeWithDifferentTypes tests range operations with various numeric types.
func TestRangeWithDifferentTypes(t *testing.T) {
	t.Run("Int8Range", func(t *testing.T) {
		r := NewRange[int8](1, 10)
		if !r.Contains(5) {
			t.Error("int8 range should contain 5")
		}
	})

	t.Run("Int16Range", func(t *testing.T) {
		r := NewRange[int16](100, 200)
		if !r.Contains(150) {
			t.Error("int16 range should contain 150")
		}
	})

	t.Run("Int32Range", func(t *testing.T) {
		r := NewRange[int32](1000, 2000)
		if !r.Contains(1500) {
			t.Error("int32 range should contain 1500")
		}
	})

	t.Run("Int64Range", func(t *testing.T) {
		r := NewRange[int64](10000, 20000)
		if !r.Contains(15000) {
			t.Error("int64 range should contain 15000")
		}
	})

	t.Run("Uint8Range", func(t *testing.T) {
		r := NewRange[uint8](10, 250)
		if !r.Contains(100) {
			t.Error("uint8 range should contain 100")
		}
	})

	t.Run("Uint16Range", func(t *testing.T) {
		r := NewRange[uint16](1000, 50000)
		if !r.Contains(25000) {
			t.Error("uint16 range should contain 25000")
		}
	})

	t.Run("Uint32Range", func(t *testing.T) {
		r := NewRange[uint32](100000, 500000)
		if !r.Contains(300000) {
			t.Error("uint32 range should contain 300000")
		}
	})

	t.Run("Uint64Range", func(t *testing.T) {
		r := NewRange[uint64](1000000, 5000000)
		if !r.Contains(3000000) {
			t.Error("uint64 range should contain 3000000")
		}
	})

	t.Run("Float32Range", func(t *testing.T) {
		r := NewRange[float32](1.1, 9.9)
		if !r.Contains(5.5) {
			t.Error("float32 range should contain 5.5")
		}
	})

	t.Run("Float64Range", func(t *testing.T) {
		r := NewRange[float64](1.1, 9.9)
		if !r.Contains(5.5) {
			t.Error("float64 range should contain 5.5")
		}
	})
}

// TestRangeEdgeCases tests edge cases including maximum values and negative ranges.
func TestRangeEdgeCases(t *testing.T) {
	t.Run("MaximumUint8Value", func(t *testing.T) {
		maxRange := NewRange[uint8](0, 255)
		if !maxRange.IsValid() {
			t.Error("Max uint8 range should be valid")
		}

		if !maxRange.Contains(255) {
			t.Error("Max uint8 range should contain 255")
		}
	})

	t.Run("NegativeRangeContains", func(t *testing.T) {
		negRange := NewRange(-100, -10)
		if !negRange.IsValid() {
			t.Error("Negative range should be valid")
		}

		if !negRange.Contains(-50) {
			t.Error("Negative range should contain -50")
		}

		if negRange.Contains(-5) {
			t.Error("Negative range should not contain -5")
		}
	})

	t.Run("NegativeRangeIntersection", func(t *testing.T) {
		r1 := NewRange(-10, 0)
		r2 := NewRange(-5, 5)

		if !r1.Overlaps(r2) {
			t.Error("Negative ranges should overlap")
		}

		intersection := r1.Intersection(r2)

		expected := NewRange(-5, 0)
		if intersection != expected {
			t.Errorf("Expected negative intersection %v, got %v", expected, intersection)
		}
	})
}

// TestRangeStringOperations tests comprehensive string range operations.
func TestRangeStringOperations(t *testing.T) {
	t.Run("StringRangeContains", func(t *testing.T) {
		r1 := NewRange("apple", "orange")

		if !r1.Contains("mango") {
			t.Error("Range [apple, orange] should contain 'mango'")
		}

		if r1.Contains("pear") {
			t.Error("Range [apple, orange] should not contain 'pear'")
		}
	})

	t.Run("OverlappingStringRanges", func(t *testing.T) {
		r1 := NewRange("apple", "orange")
		r2 := NewRange("banana", "zebra")

		if !r1.Overlaps(r2) {
			t.Error("String ranges should overlap")
		}

		intersection := r1.Intersection(r2)
		expectedStart := "banana"
		expectedEnd := "orange"

		if intersection.Start != expectedStart || intersection.End != expectedEnd {
			t.Errorf("Expected string intersection [%s, %s], got [%s, %s]",
				expectedStart, expectedEnd, intersection.Start, intersection.End)
		}
	})

	t.Run("NonOverlappingStringRanges", func(t *testing.T) {
		r3 := NewRange("aaa", "bbb")
		r4 := NewRange("yyy", "zzz")

		if r3.Overlaps(r4) {
			t.Error("Non-overlapping string ranges should not overlap")
		}

		noOverlap := r3.Intersection(r4)
		if !noOverlap.IsEmpty() {
			t.Error("Non-overlapping string ranges should have empty intersection")
		}
	})
}
