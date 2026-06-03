package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConvertByteSlices verifies that UTF-8 []byte values are converted to
// strings at every nesting level (top-level, nested maps, slices, and slices of
// maps), that binary []byte is preserved, and that non-byte values pass through
// unchanged.
func TestConvertByteSlices(t *testing.T) {
	invalidUTF8 := []byte{0xff, 0xfe, 0xfd}

	tests := []struct {
		name string
		in   any
		want any
	}{
		{
			name: "ScalarTextBytes",
			in:   []byte("hello"),
			want: "hello",
		},
		{
			name: "BinaryBytesPreserved",
			in:   invalidUTF8,
			want: invalidUTF8,
		},
		{
			name: "NonByteScalarPassthrough",
			in:   42,
			want: 42,
		},
		{
			name: "NilPassthrough",
			in:   nil,
			want: nil,
		},
		{
			name: "TopLevelMap",
			in:   map[string]any{"name": []byte("alice"), "age": 30},
			want: map[string]any{"name": "alice", "age": 30},
		},
		{
			name: "NestedMap",
			in:   map[string]any{"meta": map[string]any{"label": []byte("tag")}},
			want: map[string]any{"meta": map[string]any{"label": "tag"}},
		},
		{
			name: "TextBytesInsideSlice",
			in:   []any{[]byte("a"), []byte("b")},
			want: []any{"a", "b"},
		},
		{
			name: "SliceOfMaps",
			in:   []any{map[string]any{"k": []byte("v")}},
			want: []any{map[string]any{"k": "v"}},
		},
		{
			name: "BinaryBytesInsideSlicePreserved",
			in:   []any{[]byte("ok"), invalidUTF8},
			want: []any{"ok", invalidUTF8},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertByteSlices(tt.in)
			assert.Equal(t, tt.want, got, "convertByteSlices should convert UTF-8 bytes at every level and preserve everything else")
		})
	}
}
