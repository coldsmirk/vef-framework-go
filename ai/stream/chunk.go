package stream

// ChunkType represents the type of a UI message stream chunk.
type ChunkType string

// Chunk types as defined by AI SDK UI Message Stream Protocol.
const (
	ChunkTypeStart      ChunkType = "start"
	ChunkTypeFinish     ChunkType = "finish"
	ChunkTypeStartStep  ChunkType = "start-step"
	ChunkTypeFinishStep ChunkType = "finish-step"
	ChunkTypeError      ChunkType = "error"

	ChunkTypeTextStart ChunkType = "text-start"
	ChunkTypeTextDelta ChunkType = "text-delta"
	ChunkTypeTextEnd   ChunkType = "text-end"

	ChunkTypeReasoningStart ChunkType = "reasoning-start"
	ChunkTypeReasoningDelta ChunkType = "reasoning-delta"
	ChunkTypeReasoningEnd   ChunkType = "reasoning-end"

	ChunkTypeToolInputStart      ChunkType = "tool-input-start"
	ChunkTypeToolInputDelta      ChunkType = "tool-input-delta"
	ChunkTypeToolInputAvailable  ChunkType = "tool-input-available"
	ChunkTypeToolOutputAvailable ChunkType = "tool-output-available"

	ChunkTypeSourceURL      ChunkType = "source-url"
	ChunkTypeSourceDocument ChunkType = "source-document"

	ChunkTypeFile ChunkType = "file"
)

// Chunk represents a single chunk in the UI message stream.
type Chunk map[string]any

// Source represents a reference source (URL or document).
type Source struct {
	Type      string
	ID        string
	URL       string
	Title     string
	MediaType string
}

func NewStartChunk(messageID string) Chunk {
	return Chunk{"type": ChunkTypeStart, "messageID": messageID}
}

func NewFinishChunk() Chunk {
	return Chunk{"type": ChunkTypeFinish}
}

func NewStartStepChunk() Chunk {
	return Chunk{"type": ChunkTypeStartStep}
}

func NewFinishStepChunk() Chunk {
	return Chunk{"type": ChunkTypeFinishStep}
}

func NewErrorChunk(errorText string) Chunk {
	return Chunk{"type": ChunkTypeError, "errorText": errorText}
}

func NewTextStartChunk(id string) Chunk {
	return Chunk{"type": ChunkTypeTextStart, "id": id}
}

func NewTextDeltaChunk(id, delta string) Chunk {
	return Chunk{"type": ChunkTypeTextDelta, "id": id, "delta": delta}
}

func NewTextEndChunk(id string) Chunk {
	return Chunk{"type": ChunkTypeTextEnd, "id": id}
}

func NewReasoningStartChunk(id string) Chunk {
	return Chunk{"type": ChunkTypeReasoningStart, "id": id}
}

func NewReasoningDeltaChunk(id, delta string) Chunk {
	return Chunk{"type": ChunkTypeReasoningDelta, "id": id, "delta": delta}
}

func NewReasoningEndChunk(id string) Chunk {
	return Chunk{"type": ChunkTypeReasoningEnd, "id": id}
}

func NewToolInputStartChunk(toolCallID, toolName string) Chunk {
	return Chunk{
		"type":       ChunkTypeToolInputStart,
		"toolCallID": toolCallID,
		"toolName":   toolName,
	}
}

func NewToolInputDeltaChunk(toolCallID, delta string) Chunk {
	return Chunk{
		"type":           ChunkTypeToolInputDelta,
		"toolCallID":     toolCallID,
		"inputTextDelta": delta,
	}
}

func NewToolInputAvailableChunk(toolCallID, toolName string, input any) Chunk {
	return Chunk{
		"type":       ChunkTypeToolInputAvailable,
		"toolCallID": toolCallID,
		"toolName":   toolName,
		"input":      input,
	}
}

func NewToolOutputAvailableChunk(toolCallID string, output any) Chunk {
	return Chunk{
		"type":       ChunkTypeToolOutputAvailable,
		"toolCallID": toolCallID,
		"output":     output,
	}
}

func NewSourceURLChunk(sourceID, url, title string) Chunk {
	c := Chunk{
		"type":     ChunkTypeSourceURL,
		"sourceID": sourceID,
		"url":      url,
	}
	if title != "" {
		c["title"] = title
	}

	return c
}

func NewSourceDocumentChunk(sourceID, mediaType, title string) Chunk {
	c := Chunk{
		"type":      ChunkTypeSourceDocument,
		"sourceID":  sourceID,
		"mediaType": mediaType,
	}
	if title != "" {
		c["title"] = title
	}

	return c
}

func NewFileChunk(fileID, mediaType, url string) Chunk {
	return Chunk{
		"type":      ChunkTypeFile,
		"fileID":    fileID,
		"mediaType": mediaType,
		"url":       url,
	}
}

// NewDataChunk creates a custom data chunk. Type will be "data-{dataType}".
func NewDataChunk(dataType string, data any) Chunk {
	return Chunk{
		"type": ChunkType("data-" + dataType),
		"data": data,
	}
}
