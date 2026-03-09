package stream

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSSEWriterWriteChunk tests SSE Writer write chunk scenarios.
func TestSSEWriterWriteChunk(t *testing.T) {
	tests := []struct {
		name     string
		chunk    Chunk
		expected string
	}{
		{
			name:     "SimpleChunk",
			chunk:    Chunk{"type": "test"},
			expected: `data: {"type":"test"}` + "\n\n",
		},
		{
			name:     "ChunkWithStringValue",
			chunk:    Chunk{"type": "text-delta", "delta": "Hello"},
			expected: `data: {"delta":"Hello","type":"text-delta"}` + "\n\n",
		},
		{
			name:     "ChunkWithNestedObject",
			chunk:    Chunk{"type": "data", "data": map[string]any{"key": "value"}},
			expected: `data: {"data":{"key":"value"},"type":"data"}` + "\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			w := newSseWriter(bufio.NewWriter(&buf))

			err := w.WriteChunk(tt.chunk)

			require.NoError(t, err, "Should not return error")
			assert.Equal(t, tt.expected, buf.String(), "Should equal expected value")
		})
	}
}

// TestSSEWriterWriteDone tests SSE Writer write done scenarios.
func TestSSEWriterWriteDone(t *testing.T) {
	var buf bytes.Buffer

	w := newSseWriter(bufio.NewWriter(&buf))

	err := w.writeDone()

	require.NoError(t, err, "Should not return error")
	assert.Equal(t, "data: [DONE]\n\n", buf.String(), "Should equal expected value")
}

// TestSSEWriterFlush tests SSE Writer flush scenarios.
func TestSSEWriterFlush(t *testing.T) {
	var buf bytes.Buffer

	bw := bufio.NewWriter(&buf)
	w := newSseWriter(bw)

	_, _ = bw.WriteString("pending data")
	err := w.Flush()

	require.NoError(t, err, "Should not return error")
	assert.Equal(t, "pending data", buf.String(), "Should equal expected value")
}

// TestSSEHeaders tests SSE headers functionality.
func TestSSEHeaders(t *testing.T) {
	assert.Equal(t, "text/event-stream", SseHeaders["Content-Type"], "Should equal expected value")
	assert.Equal(t, "no-cache", SseHeaders["Cache-Control"], "Should equal expected value")
	assert.Equal(t, "keep-alive", SseHeaders["Connection"], "Should equal expected value")
	assert.Equal(t, "chunked", SseHeaders["Transfer-Encoding"], "Should equal expected value")
	assert.Equal(t, "v1", SseHeaders["X-Vercel-AI-UI-Message-Stream"], "Should equal expected value")
	assert.Equal(t, "no", SseHeaders["X-Accel-Buffering"], "Should equal expected value")
}

// TestDefaultIDGeneratorFormat tests default ID generator format scenarios.
func TestDefaultIDGeneratorFormat(t *testing.T) {
	prefixes := []string{"message", "text", "reasoning", "call"}

	for _, prefix := range prefixes {
		t.Run(prefix, func(t *testing.T) {
			id := defaultIDGenerator(prefix)

			assert.True(t, strings.HasPrefix(id, prefix+"_"), "Condition should be true")
			// UUID v7 format: prefix_xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
			parts := strings.SplitN(id, "_", 2)
			require.Len(t, parts, 2, "Length should be 2")
			assert.Len(t, parts[1], 36, "Length should match expected value") // UUID length
		})
	}
}

// TestDefaultIDGeneratorUniqueness tests default ID generator uniqueness scenarios.
func TestDefaultIDGeneratorUniqueness(t *testing.T) {
	ids := make(map[string]bool)

	for range 100 {
		id := defaultIDGenerator("test")
		assert.False(t, ids[id], "duplicate id generated: %s", id)
		ids[id] = true
	}
}
