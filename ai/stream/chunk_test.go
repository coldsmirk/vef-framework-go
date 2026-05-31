package stream

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewStartChunk tests new start chunk functionality.
func TestNewStartChunk(t *testing.T) {
	chunk := NewStartChunk("msg_123")

	assert.Equal(t, ChunkTypeStart, chunk["type"], "Start chunk should use the start type")
	assert.Equal(t, "msg_123", chunk["messageID"], "Start chunk should include the message ID")
}

// TestNewFinishChunk tests new finish chunk functionality.
func TestNewFinishChunk(t *testing.T) {
	chunk := NewFinishChunk()

	assert.Equal(t, ChunkTypeFinish, chunk["type"], "Finish chunk should use the finish type")
	assert.Len(t, chunk, 1, "Finish chunk should only include the type field")
}

// TestNewStartStepChunk tests new start step chunk functionality.
func TestNewStartStepChunk(t *testing.T) {
	chunk := NewStartStepChunk()

	assert.Equal(t, ChunkTypeStartStep, chunk["type"], "Start step chunk should use the start-step type")
	assert.Len(t, chunk, 1, "Start step chunk should only include the type field")
}

// TestNewFinishStepChunk tests new finish step chunk functionality.
func TestNewFinishStepChunk(t *testing.T) {
	chunk := NewFinishStepChunk()

	assert.Equal(t, ChunkTypeFinishStep, chunk["type"], "Finish step chunk should use the finish-step type")
	assert.Len(t, chunk, 1, "Finish step chunk should only include the type field")
}

// TestNewErrorChunk tests new error chunk functionality.
func TestNewErrorChunk(t *testing.T) {
	chunk := NewErrorChunk("something went wrong")

	assert.Equal(t, ChunkTypeError, chunk["type"], "Error chunk should use the error type")
	assert.Equal(t, "something went wrong", chunk["errorText"], "Error chunk should include the error text")
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
			assert.Equal(t, tt.expected, chunk, "Text chunk should match the protocol payload")
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
			assert.Equal(t, tt.expected, chunk, "Reasoning chunk should match the protocol payload")
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
			assert.Equal(t, tt.expected, chunk, "Tool chunk should match the protocol payload")
		})
	}
}

// TestSourceChunks tests source chunks functionality.
func TestSourceChunks(t *testing.T) {
	t.Run("SourceURLWithTitle", func(t *testing.T) {
		chunk := NewSourceURLChunk("src_1", "https://example.com", "Example Site")

		assert.Equal(t, ChunkTypeSourceURL, chunk["type"], "Source URL chunk should use the source-url type")
		assert.Equal(t, "src_1", chunk["sourceID"], "Source URL chunk should include the source ID")
		assert.Equal(t, "https://example.com", chunk["url"], "Source URL chunk should include the URL")
		assert.Equal(t, "Example Site", chunk["title"], "Source URL chunk should include a non-empty title")
	})

	t.Run("SourceURLWithoutTitle", func(t *testing.T) {
		chunk := NewSourceURLChunk("src_1", "https://example.com", "")

		assert.Equal(t, ChunkTypeSourceURL, chunk["type"], "Source URL chunk should use the source-url type")
		assert.Equal(t, "src_1", chunk["sourceID"], "Source URL chunk should include the source ID")
		assert.Equal(t, "https://example.com", chunk["url"], "Source URL chunk should include the URL")
		assert.NotContains(t, chunk, "title", "Source URL chunk should omit an empty title")
	})

	t.Run("SourceDocumentWithTitle", func(t *testing.T) {
		chunk := NewSourceDocumentChunk("src_2", "application/pdf", "Report.pdf")

		assert.Equal(t, ChunkTypeSourceDocument, chunk["type"], "Source document chunk should use the source-document type")
		assert.Equal(t, "src_2", chunk["sourceID"], "Source document chunk should include the source ID")
		assert.Equal(t, "application/pdf", chunk["mediaType"], "Source document chunk should include the media type")
		assert.Equal(t, "Report.pdf", chunk["title"], "Source document chunk should include a non-empty title")
	})

	t.Run("SourceDocumentWithoutTitle", func(t *testing.T) {
		chunk := NewSourceDocumentChunk("src_2", "application/pdf", "")

		assert.Equal(t, ChunkTypeSourceDocument, chunk["type"], "Source document chunk should use the source-document type")
		assert.Equal(t, "src_2", chunk["sourceID"], "Source document chunk should include the source ID")
		assert.Equal(t, "application/pdf", chunk["mediaType"], "Source document chunk should include the media type")
		assert.NotContains(t, chunk, "title", "Source document chunk should omit an empty title")
	})
}

// TestNewFileChunk tests new file chunk functionality.
func TestNewFileChunk(t *testing.T) {
	chunk := NewFileChunk("file_1", "image/png", "https://cdn.example.com/image.png")

	assert.Equal(t, ChunkTypeFile, chunk["type"], "File chunk should use the file type")
	assert.Equal(t, "file_1", chunk["fileID"], "File chunk should include the file ID")
	assert.Equal(t, "image/png", chunk["mediaType"], "File chunk should include the media type")
	assert.Equal(t, "https://cdn.example.com/image.png", chunk["url"], "File chunk should include the file URL")
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

			assert.Equal(t, ChunkType("data-"+tt.dataType), chunk["type"], "Data chunk should derive its type from dataType")
			assert.Equal(t, tt.data, chunk["data"], "Data chunk should include the provided data payload")
		})
	}
}
