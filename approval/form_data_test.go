package approval

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewFormData tests new form data functionality.
func TestNewFormData(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]any
		wantLen int
	}{
		{"NilInput", nil, 0},
		{"NonNilInput", map[string]any{"key": "value"}, 1},
		{"EmptyMap", map[string]any{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fd := NewFormData(tt.input)
			require.NotNil(t, fd, "Should never return nil")
			assert.Len(t, fd, tt.wantLen, "Should have expected length")
		})
	}
}

// TestFormDataGet tests form data get functionality.
func TestFormDataGet(t *testing.T) {
	fd := NewFormData(map[string]any{"name": "alice", "age": 30})

	tests := []struct {
		name     string
		key      string
		expected any
	}{
		{"ExistingKey", "name", "alice"},
		{"NonexistentKey", "missing", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, fd.Get(tt.key), "Should return expected value")
		})
	}
}

// TestFormDataSet tests form data set functionality.
func TestFormDataSet(t *testing.T) {
	fd := NewFormData(nil)
	fd.Set("key", "value")
	assert.Equal(t, "value", fd.Get("key"), "Should store and retrieve the value")
}

// TestFormDataToMap tests form data to map functionality.
func TestFormDataToMap(t *testing.T) {
	original := map[string]any{"a": 1, "b": "two"}
	fd := NewFormData(original)
	assert.Equal(t, original, fd.ToMap(), "Should return the underlying map")
}

// TestFormDataClone tests form data clone functionality.
func TestFormDataClone(t *testing.T) {
	t.Run("DeepCopy", func(t *testing.T) {
		fd := NewFormData(map[string]any{
			"name":   "alice",
			"nested": map[string]any{"key": "value"},
		})
		cloned := fd.Clone()

		cloned.Set("name", "bob")
		assert.Equal(t, "alice", fd.Get("name"), "Should not affect original after modifying clone")
		assert.Equal(t, "bob", cloned.Get("name"), "Should reflect change in clone")
	})

	t.Run("EmptyFormData", func(t *testing.T) {
		fd := NewFormData(nil)
		cloned := fd.Clone()
		require.NotNil(t, cloned, "Should return non-nil for empty clone")
		assert.Empty(t, cloned, "Should be empty")
	})

	t.Run("MarshalError", func(t *testing.T) {
		fd := FormData{"bad": make(chan int)}
		cloned := fd.Clone()
		require.NotNil(t, cloned, "Should return empty FormData on marshal error")
		assert.Empty(t, cloned, "Should be empty when marshal fails")
	})

	t.Run("NestedMapClone", func(t *testing.T) {
		fd := NewFormData(map[string]any{
			"items": []any{
				map[string]any{"id": 1, "name": "item1"},
				map[string]any{"id": 2, "name": "item2"},
			},
		})
		cloned := fd.Clone()

		items := cloned.Get("items").([]any)
		firstItem := items[0].(map[string]any)
		firstItem["name"] = "modified"

		origItems := fd.Get("items").([]any)
		origFirst := origItems[0].(map[string]any)
		assert.Equal(t, "item1", origFirst["name"], "Should not affect original nested data")
	})
}
