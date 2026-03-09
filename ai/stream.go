package ai

import "io"

// MessageStream represents a stream of message chunks.
type MessageStream interface {
	io.Closer

	// Recv receives the next message chunk from the stream.
	// Returns io.EOF when the stream is exhausted.
	Recv() (*MessageChunk, error)
	// Collect collects all chunks and merges them into a complete message.
	Collect() (*Message, error)
}

// MessageChunk represents a chunk of a streaming message.
type MessageChunk struct {
	// Content is the incremental text content.
	Content string
	// ToolCalls contains tool calls (may be partial in streaming).
	ToolCalls []ToolCall
	// Done indicates whether the stream is complete.
	Done bool
}

// StringStream represents a stream of string chunks.
type StringStream interface {
	io.Closer

	// Recv receives the next string chunk from the stream.
	// Returns io.EOF when the stream is exhausted.
	Recv() (string, error)
	// Collect collects all chunks and concatenates them.
	Collect() (string, error)
}
