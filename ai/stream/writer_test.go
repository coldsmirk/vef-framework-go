package stream

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type FailingWriter struct {
	err error
}

func (w FailingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

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

			require.NoError(t, err, "WriteChunk should encode and flush a valid chunk")
			assert.Equal(t, tt.expected, buf.String(), "WriteChunk should emit the expected SSE frame")
		})
	}
}

// TestSSEWriterWriteChunkErrors tests SSE Writer write chunk error scenarios.
func TestSSEWriterWriteChunkErrors(t *testing.T) {
	t.Run("MarshalError", func(t *testing.T) {
		var buf bytes.Buffer

		w := newSseWriter(bufio.NewWriter(&buf))

		err := w.WriteChunk(Chunk{"type": "bad", "value": make(chan int)})

		require.ErrorContains(t, err, "failed to marshal chunk", "WriteChunk should wrap JSON marshal failures")
	})

	t.Run("WriteError", func(t *testing.T) {
		expectedErr := errors.New("write failed")
		w := newSseWriter(bufio.NewWriterSize(FailingWriter{err: expectedErr}, 1))

		err := w.WriteChunk(Chunk{"type": "test"})

		require.ErrorIs(t, err, expectedErr, "WriteChunk should preserve the underlying write error")
		assert.ErrorContains(t, err, "failed to write sse data", "WriteChunk should identify SSE write failures")
	})

	t.Run("FlushError", func(t *testing.T) {
		expectedErr := errors.New("flush failed")
		w := newSseWriter(bufio.NewWriterSize(FailingWriter{err: expectedErr}, 1024))

		err := w.WriteChunk(Chunk{"type": "test"})

		require.ErrorIs(t, err, expectedErr, "WriteChunk should preserve the trailing flush error")
		assert.NotContains(t, err.Error(), "failed to write sse data", "WriteChunk trailing flush errors should not be reported as write failures")
	})
}

// TestSSEWriterWriteDone tests SSE Writer write done scenarios.
func TestSSEWriterWriteDone(t *testing.T) {
	var buf bytes.Buffer

	w := newSseWriter(bufio.NewWriter(&buf))

	err := w.writeDone()

	require.NoError(t, err, "WriteDone should flush the DONE frame")
	assert.Equal(t, "data: [DONE]\n\n", buf.String(), "WriteDone should emit the DONE SSE frame")
}

// TestSSEWriterWriteDoneErrors tests SSE Writer write done error scenarios.
func TestSSEWriterWriteDoneErrors(t *testing.T) {
	t.Run("WriteError", func(t *testing.T) {
		expectedErr := errors.New("done write failed")
		w := newSseWriter(bufio.NewWriterSize(FailingWriter{err: expectedErr}, 1))

		err := w.writeDone()

		require.ErrorIs(t, err, expectedErr, "WriteDone should preserve the underlying write error")
	})

	t.Run("FlushError", func(t *testing.T) {
		expectedErr := errors.New("done flush failed")
		w := newSseWriter(bufio.NewWriterSize(FailingWriter{err: expectedErr}, 1024))

		err := w.writeDone()

		require.ErrorIs(t, err, expectedErr, "WriteDone should preserve the trailing flush error")
	})
}

// TestSSEWriterFlush tests SSE Writer flush scenarios.
func TestSSEWriterFlush(t *testing.T) {
	var buf bytes.Buffer

	bw := bufio.NewWriter(&buf)
	w := newSseWriter(bw)

	_, _ = bw.WriteString("pending data")
	err := w.Flush()

	require.NoError(t, err, "Flush should write buffered data")
	assert.Equal(t, "pending data", buf.String(), "Flush should move buffered data to the underlying writer")
}

// TestSSEWriterFlushErrors tests SSE Writer flush error scenarios.
func TestSSEWriterFlushErrors(t *testing.T) {
	expectedErr := errors.New("flush failed")
	bw := bufio.NewWriterSize(FailingWriter{err: expectedErr}, 1024)
	w := newSseWriter(bw)

	_, _ = bw.WriteString("pending data")
	err := w.Flush()

	require.ErrorIs(t, err, expectedErr, "Flush should preserve the underlying writer error")
}

// TestSSEHeaders tests SSE headers functionality.
func TestSSEHeaders(t *testing.T) {
	assert.Equal(t, "text/event-stream", SseHeaders["Content-Type"], "SSE headers should set the content type")
	assert.Equal(t, "no-cache", SseHeaders["Cache-Control"], "SSE headers should disable response caching")
	assert.Equal(t, "keep-alive", SseHeaders["Connection"], "SSE headers should keep the connection alive")
	assert.Equal(t, "chunked", SseHeaders["Transfer-Encoding"], "SSE headers should use chunked transfer encoding")
	assert.Equal(t, "v1", SseHeaders["X-Vercel-AI-UI-Message-Stream"], "SSE headers should advertise UI message stream version")
	assert.Equal(t, "no", SseHeaders["X-Accel-Buffering"], "SSE headers should disable proxy buffering")
}

// TestDefaultIDGeneratorFormat tests default ID generator format scenarios.
func TestDefaultIDGeneratorFormat(t *testing.T) {
	prefixes := []string{"message", "text", "reasoning", "call"}

	for _, prefix := range prefixes {
		t.Run(prefix, func(t *testing.T) {
			id := defaultIDGenerator(prefix)

			assert.True(t, strings.HasPrefix(id, prefix+"_"), "Generated ID should include the requested prefix")
			// UUID v7 format: prefix_xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
			parts := strings.SplitN(id, "_", 2)
			require.Len(t, parts, 2, "Generated ID should contain a prefix and UUID segment")
			assert.Len(t, parts[1], 36, "Generated ID should include a UUID-length suffix")
		})
	}
}

// TestDefaultIDGeneratorUniqueness tests default ID generator uniqueness scenarios.
func TestDefaultIDGeneratorUniqueness(t *testing.T) {
	ids := make(map[string]bool)

	for range 100 {
		id := defaultIDGenerator("test")
		assert.False(t, ids[id], "Generated ID should be unique: %s", id)
		ids[id] = true
	}
}
