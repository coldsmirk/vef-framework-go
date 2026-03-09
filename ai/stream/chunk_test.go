package stream

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewStartChunk tests new start chunk functionality.
func TestNewStartChunk(t *testing.T) {
	chunk := NewStartChunk("msg_123")

	assert.Equal(t, ChunkTypeStart, chunk["type"], "Should equal expected value")
	assert.Equal(t, "msg_123", chunk["messageID"], "Should equal expected value")
}

// TestNewFinishChunk tests new finish chunk functionality.
func TestNewFinishChunk(t *testing.T) {
	chunk := NewFinishChunk()

	assert.Equal(t, ChunkTypeFinish, chunk["type"], "Should equal expected value")
	assert.Len(t, chunk, 1, "Length should be 1")
}

// TestNewStartStepChunk tests new start step chunk functionality.
func TestNewStartStepChunk(t *testing.T) {
	chunk := NewStartStepChunk()

	assert.Equal(t, ChunkTypeStartStep, chunk["type"], "Should equal expected value")
	assert.Len(t, chunk, 1, "Length should be 1")
}

// TestNewFinishStepChunk tests new finish step chunk functionality.
func TestNewFinishStepChunk(t *testing.T) {
	chunk := NewFinishStepChunk()

	assert.Equal(t, ChunkTypeFinishStep, chunk["type"], "Should equal expected value")
	assert.Len(t, chunk, 1, "Length should be 1")
}

// TestNewErrorChunk tests new error chunk functionality.
func TestNewErrorChunk(t *testing.T) {
	chunk := NewErrorChunk("something went wrong")

	assert.Equal(t, ChunkTypeError, chunk["type"], "Should equal expected value")
	assert.Equal(t, "something went wrong", chunk["errorText"], "Should equal expected value")
}

// TestTextChunks tests text chunks functionality.
func TestTextChunks(t *testing.T) {
	tests := []struct {
		name     string
		fn       func() Chunk
		expected Chunk
	}{
		{
			name: "TextStart",
			fn:   func() Chunk { return NewTextStartChunk("text_1") },
			expected: Chunk{
				"type": ChunkTypeTextStart,
				"id":   "text_1",
			},
		},
		{
			name: "TextDelta",
			fn:   func() Chunk { return NewTextDeltaChunk("text_1", "Hello") },
			expected: Chunk{
				"type":  ChunkTypeTextDelta,
				"id":    "text_1",
				"delta": "Hello",
			},
		},
		{
			name: "TextEnd",
			fn:   func() Chunk { return NewTextEndChunk("text_1") },
			expected: Chunk{
				"type": ChunkTypeTextEnd,
				"id":   "text_1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk := tt.fn()
			assert.Equal(t, tt.expected, chunk, "Should equal expected value")
		})
	}
}

// TestReasoningChunks tests reasoning chunks functionality.
func TestReasoningChunks(t *testing.T) {
	tests := []struct {
		name     string
		fn       func() Chunk
		expected Chunk
	}{
		{
			name: "ReasoningStart",
			fn:   func() Chunk { return NewReasoningStartChunk("reasoning_1") },
			expected: Chunk{
				"type": ChunkTypeReasoningStart,
				"id":   "reasoning_1",
			},
		},
		{
			name: "ReasoningDelta",
			fn:   func() Chunk { return NewReasoningDeltaChunk("reasoning_1", "thinking...") },
			expected: Chunk{
				"type":  ChunkTypeReasoningDelta,
				"id":    "reasoning_1",
				"delta": "thinking...",
			},
		},
		{
			name: "ReasoningEnd",
			fn:   func() Chunk { return NewReasoningEndChunk("reasoning_1") },
			expected: Chunk{
				"type": ChunkTypeReasoningEnd,
				"id":   "reasoning_1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk := tt.fn()
			assert.Equal(t, tt.expected, chunk, "Should equal expected value")
		})
	}
}

// TestToolChunks tests tool chunks functionality.
func TestToolChunks(t *testing.T) {
	tests := []struct {
		name     string
		fn       func() Chunk
		expected Chunk
	}{
		{
			name: "ToolInputStart",
			fn:   func() Chunk { return NewToolInputStartChunk("call_1", "get_weather") },
			expected: Chunk{
				"type":       ChunkTypeToolInputStart,
				"toolCallID": "call_1",
				"toolName":   "get_weather",
			},
		},
		{
			name: "ToolInputDelta",
			fn:   func() Chunk { return NewToolInputDeltaChunk("call_1", `{"city":`) },
			expected: Chunk{
				"type":           ChunkTypeToolInputDelta,
				"toolCallID":     "call_1",
				"inputTextDelta": `{"city":`,
			},
		},
		{
			name: "ToolInputAvailable",
			fn: func() Chunk {
				return NewToolInputAvailableChunk("call_1", "get_weather", map[string]string{"city": "Beijing"})
			},
			expected: Chunk{
				"type":       ChunkTypeToolInputAvailable,
				"toolCallID": "call_1",
				"toolName":   "get_weather",
				"input":      map[string]string{"city": "Beijing"},
			},
		},
		{
			name: "ToolOutputAvailable",
			fn: func() Chunk {
				return NewToolOutputAvailableChunk("call_1", map[string]any{"temp": 25, "unit": "celsius"})
			},
			expected: Chunk{
				"type":       ChunkTypeToolOutputAvailable,
				"toolCallID": "call_1",
				"output":     map[string]any{"temp": 25, "unit": "celsius"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk := tt.fn()
			assert.Equal(t, tt.expected, chunk, "Should equal expected value")
		})
	}
}

// TestSourceChunks tests source chunks functionality.
func TestSourceChunks(t *testing.T) {
	t.Run("SourceURLWithTitle", func(t *testing.T) {
		chunk := NewSourceURLChunk("src_1", "https://example.com", "Example Site")

		assert.Equal(t, ChunkTypeSourceURL, chunk["type"], "Should equal expected value")
		assert.Equal(t, "src_1", chunk["sourceID"], "Should equal expected value")
		assert.Equal(t, "https://example.com", chunk["url"], "Should equal expected value")
		assert.Equal(t, "Example Site", chunk["title"], "Should equal expected value")
	})

	t.Run("SourceURLWithoutTitle", func(t *testing.T) {
		chunk := NewSourceURLChunk("src_1", "https://example.com", "")

		assert.Equal(t, ChunkTypeSourceURL, chunk["type"], "Should equal expected value")
		assert.Equal(t, "src_1", chunk["sourceID"], "Should equal expected value")
		assert.Equal(t, "https://example.com", chunk["url"], "Should equal expected value")
		assert.NotContains(t, chunk, "title", "Should not contain value")
	})

	t.Run("SourceDocumentWithTitle", func(t *testing.T) {
		chunk := NewSourceDocumentChunk("src_2", "application/pdf", "Report.pdf")

		assert.Equal(t, ChunkTypeSourceDocument, chunk["type"], "Should equal expected value")
		assert.Equal(t, "src_2", chunk["sourceID"], "Should equal expected value")
		assert.Equal(t, "application/pdf", chunk["mediaType"], "Should equal expected value")
		assert.Equal(t, "Report.pdf", chunk["title"], "Should equal expected value")
	})

	t.Run("SourceDocumentWithoutTitle", func(t *testing.T) {
		chunk := NewSourceDocumentChunk("src_2", "application/pdf", "")

		assert.Equal(t, ChunkTypeSourceDocument, chunk["type"], "Should equal expected value")
		assert.Equal(t, "src_2", chunk["sourceID"], "Should equal expected value")
		assert.Equal(t, "application/pdf", chunk["mediaType"], "Should equal expected value")
		assert.NotContains(t, chunk, "title", "Should not contain value")
	})
}

// TestNewFileChunk tests new file chunk functionality.
func TestNewFileChunk(t *testing.T) {
	chunk := NewFileChunk("file_1", "image/png", "https://cdn.example.com/image.png")

	assert.Equal(t, ChunkTypeFile, chunk["type"], "Should equal expected value")
	assert.Equal(t, "file_1", chunk["fileID"], "Should equal expected value")
	assert.Equal(t, "image/png", chunk["mediaType"], "Should equal expected value")
	assert.Equal(t, "https://cdn.example.com/image.png", chunk["url"], "Should equal expected value")
}

// TestNewDataChunk tests new data chunk functionality.
func TestNewDataChunk(t *testing.T) {
	tests := []struct {
		name     string
		dataType string
		data     any
	}{
		{
			name:     "StringData",
			dataType: "status",
			data:     "processing",
		},
		{
			name:     "MapData",
			dataType: "metadata",
			data:     map[string]any{"key": "value"},
		},
		{
			name:     "SliceData",
			dataType: "items",
			data:     []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk := NewDataChunk(tt.dataType, tt.data)

			assert.Equal(t, ChunkType("data-"+tt.dataType), chunk["type"], "Should equal expected value")
			assert.Equal(t, tt.data, chunk["data"], "Should equal expected value")
		})
	}
}
