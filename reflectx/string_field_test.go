package reflectx

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsStringType tests IsStringType for string, *string and other types.
func TestIsStringType(t *testing.T) {
	tests := []struct {
		name     string
		input    reflect.Type
		expected bool
	}{
		{"String", reflect.TypeFor[string](), true},
		{"PointerToString", reflect.TypeFor[*string](), true},

		{"Int", reflect.TypeFor[int](), false},
		{"PointerToInt", reflect.TypeFor[*int](), false},
		{"StringSlice", reflect.TypeFor[[]string](), false},
		{"StringMap", reflect.TypeFor[map[string]string](), false},
		{"DoublePointerToString", reflect.TypeFor[**string](), false},
		{"Struct", reflect.TypeFor[struct{}](), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsStringType(tt.input), "IsStringType result should match expected")
		})
	}
}

// TestIsStringSliceType tests IsStringSliceType for []string and other types.
func TestIsStringSliceType(t *testing.T) {
	tests := []struct {
		name     string
		input    reflect.Type
		expected bool
	}{
		{"StringSlice", reflect.TypeFor[[]string](), true},

		{"String", reflect.TypeFor[string](), false},
		{"IntSlice", reflect.TypeFor[[]int](), false},
		{"PointerToStringSlice", reflect.TypeFor[*[]string](), false},
		{"StringArray", reflect.TypeFor[[3]string](), false},
		{"StringMap", reflect.TypeFor[map[string]string](), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsStringSliceType(tt.input), "IsStringSliceType result should match expected")
		})
	}
}

// TestIsStringMapType tests IsStringMapType for map[string]string and other types.
func TestIsStringMapType(t *testing.T) {
	tests := []struct {
		name     string
		input    reflect.Type
		expected bool
	}{
		{"StringMap", reflect.TypeFor[map[string]string](), true},

		{"String", reflect.TypeFor[string](), false},
		{"StringSlice", reflect.TypeFor[[]string](), false},
		{"IntKeyStringValueMap", reflect.TypeFor[map[int]string](), false},
		{"StringKeyIntValueMap", reflect.TypeFor[map[string]int](), false},
		{"PointerToStringMap", reflect.TypeFor[*map[string]string](), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsStringMapType(tt.input), "IsStringMapType result should match expected")
		})
	}
}

// TestGetStringValue tests GetStringValue for string and *string fields.
func TestGetStringValue(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		s := "hello"
		v, ok := GetStringValue(reflect.ValueOf(s))
		assert.True(t, ok, "Reading string should succeed")
		assert.Equal(t, "hello", v, "Should return the string value")
	})

	t.Run("EmptyString", func(t *testing.T) {
		v, ok := GetStringValue(reflect.ValueOf(""))
		assert.True(t, ok, "Reading empty string is still valid")
		assert.Empty(t, v, "Should return empty string")
	})

	t.Run("NonNilStringPointer", func(t *testing.T) {
		s := "world"
		v, ok := GetStringValue(reflect.ValueOf(&s))
		assert.True(t, ok, "Reading non-nil *string should succeed")
		assert.Equal(t, "world", v, "Should return the dereferenced value")
	})

	t.Run("NilStringPointer", func(t *testing.T) {
		var s *string
		v, ok := GetStringValue(reflect.ValueOf(s))
		assert.False(t, ok, "Reading nil *string should report not-ok")
		assert.Empty(t, v, "Should return zero string for nil pointer")
	})

	t.Run("UnsupportedType", func(t *testing.T) {
		v, ok := GetStringValue(reflect.ValueOf(42))
		assert.False(t, ok, "Reading int should report not-ok")
		assert.Empty(t, v, "Should return zero string for unsupported type")
	})
}

// TestSetStringValue tests SetStringValue for string and *string fields.
func TestSetStringValue(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		type holder struct{ S string }
		h := &holder{}
		SetStringValue(reflect.ValueOf(h).Elem().FieldByName("S"), "abc")
		assert.Equal(t, "abc", h.S, "String field should be updated")
	})

	t.Run("StringPointer", func(t *testing.T) {
		type holder struct{ S *string }
		h := &holder{}
		SetStringValue(reflect.ValueOf(h).Elem().FieldByName("S"), "xyz")
		require.NotNil(t, h.S, "Pointer should be allocated")
		assert.Equal(t, "xyz", *h.S, "Pointer should reference the new value")
	})

	t.Run("StringPointerAllocatesFreshPointer", func(t *testing.T) {
		type holder struct{ S *string }
		original := "old"
		h := &holder{S: &original}

		SetStringValue(reflect.ValueOf(h).Elem().FieldByName("S"), "new")

		assert.Equal(t, "new", *h.S, "Field should now point to the new value")
		assert.Equal(t, "old", original, "Original pointee must remain unchanged")
	})

	t.Run("UnsupportedTypeIsNoop", func(t *testing.T) {
		type holder struct{ N int }
		h := &holder{N: 7}
		SetStringValue(reflect.ValueOf(h).Elem().FieldByName("N"), "ignored")
		assert.Equal(t, 7, h.N, "Int field must remain unchanged")
	})
}

// TestGetStringSliceValue tests GetStringSliceValue for []string fields.
func TestGetStringSliceValue(t *testing.T) {
	t.Run("NonEmptySlice", func(t *testing.T) {
		s := []string{"a", "b"}
		v, ok := GetStringSliceValue(reflect.ValueOf(s))
		assert.True(t, ok, "Reading non-empty []string should succeed")
		assert.Equal(t, []string{"a", "b"}, v, "Should return the slice")
	})

	t.Run("EmptySlice", func(t *testing.T) {
		v, ok := GetStringSliceValue(reflect.ValueOf([]string{}))
		assert.True(t, ok, "Empty (non-nil) slice is still valid")
		assert.Empty(t, v, "Should return empty slice")
	})

	t.Run("NilSlice", func(t *testing.T) {
		var s []string
		v, ok := GetStringSliceValue(reflect.ValueOf(s))
		assert.False(t, ok, "Reading nil slice should report not-ok")
		assert.Nil(t, v, "Should return nil for nil slice")
	})

	t.Run("UnsupportedType", func(t *testing.T) {
		v, ok := GetStringSliceValue(reflect.ValueOf([]int{1}))
		assert.False(t, ok, "Reading []int should report not-ok")
		assert.Nil(t, v, "Should return nil for unsupported type")
	})
}

// TestSetStringSliceValue tests SetStringSliceValue for []string fields.
func TestSetStringSliceValue(t *testing.T) {
	t.Run("Slice", func(t *testing.T) {
		type holder struct{ S []string }
		h := &holder{}
		SetStringSliceValue(reflect.ValueOf(h).Elem().FieldByName("S"), []string{"x", "y"})
		assert.Equal(t, []string{"x", "y"}, h.S, "Slice field should be updated")
	})

	t.Run("UnsupportedTypeIsNoop", func(t *testing.T) {
		type holder struct{ N []int }
		h := &holder{N: []int{1}}
		SetStringSliceValue(reflect.ValueOf(h).Elem().FieldByName("N"), []string{"ignored"})
		assert.Equal(t, []int{1}, h.N, "Int slice must remain unchanged")
	})
}

// TestGetStringMapValue tests GetStringMapValue for map[string]string fields.
func TestGetStringMapValue(t *testing.T) {
	t.Run("NonEmptyMap", func(t *testing.T) {
		m := map[string]string{"front": "a.jpg", "back": "b.jpg"}
		v, ok := GetStringMapValue(reflect.ValueOf(m))
		assert.True(t, ok, "Reading non-empty map should succeed")
		assert.Equal(t, m, v, "Should return the map")
	})

	t.Run("EmptyMap", func(t *testing.T) {
		v, ok := GetStringMapValue(reflect.ValueOf(map[string]string{}))
		assert.True(t, ok, "Empty (non-nil) map is still valid")
		assert.Empty(t, v, "Should return empty map")
	})

	t.Run("NilMap", func(t *testing.T) {
		var m map[string]string
		v, ok := GetStringMapValue(reflect.ValueOf(m))
		assert.False(t, ok, "Reading nil map should report not-ok")
		assert.Nil(t, v, "Should return nil for nil map")
	})

	t.Run("UnsupportedType", func(t *testing.T) {
		v, ok := GetStringMapValue(reflect.ValueOf(map[string]int{"a": 1}))
		assert.False(t, ok, "Reading map[string]int should report not-ok")
		assert.Nil(t, v, "Should return nil for unsupported type")
	})
}

// TestSetStringMapValue tests SetStringMapValue for map[string]string fields.
func TestSetStringMapValue(t *testing.T) {
	t.Run("Map", func(t *testing.T) {
		type holder struct {
			M map[string]string
		}
		h := &holder{}
		SetStringMapValue(reflect.ValueOf(h).Elem().FieldByName("M"), map[string]string{"k": "v"})
		assert.Equal(t, map[string]string{"k": "v"}, h.M, "Map field should be updated")
	})

	t.Run("OverwriteExistingMap", func(t *testing.T) {
		type holder struct {
			M map[string]string
		}
		h := &holder{M: map[string]string{"old": "1"}}
		SetStringMapValue(reflect.ValueOf(h).Elem().FieldByName("M"), map[string]string{"new": "2"})
		assert.Equal(t, map[string]string{"new": "2"}, h.M, "Map field should be replaced")
	})

	t.Run("UnsupportedTypeIsNoop", func(t *testing.T) {
		type holder struct {
			M map[string]int
		}
		h := &holder{M: map[string]int{"a": 1}}
		SetStringMapValue(reflect.ValueOf(h).Elem().FieldByName("M"), map[string]string{"ignored": "x"})
		assert.Equal(t, map[string]int{"a": 1}, h.M, "Map must remain unchanged")
	})
}
