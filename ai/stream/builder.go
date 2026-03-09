package stream

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"

	"github.com/gofiber/fiber/v3"
)

// CallbackWriter provides methods to push messages in callback-based sources.
type CallbackWriter interface {
	// WriteText sends a text content message as the assistant role.
	WriteText(content string)
	// WriteToolCall sends a tool call message with the given id, function name, and JSON arguments.
	WriteToolCall(id, name, arguments string)
	// WriteToolResult sends a tool result message for the specified tool call.
	WriteToolResult(toolCallID, content string)
	// WriteReasoning sends a reasoning/thinking message as the assistant role.
	WriteReasoning(reasoning string)
	// WriteData sends a custom data message with the specified type and payload.
	WriteData(dataType string, data any)
	// WriteMessage sends an arbitrary pre-built message to the stream.
	WriteMessage(msg Message)
}

// Builder provides a fluent interface for building UI message streams.
type Builder struct {
	opts      Options
	source    MessageSource
	messageID string
	headers   map[string]string
}

// New creates a new stream builder with default options.
func New() *Builder {
	return &Builder{
		opts:    DefaultOptions(),
		headers: make(map[string]string),
	}
}

func (b *Builder) WithSource(source MessageSource) *Builder {
	b.source = source

	return b
}

func (b *Builder) WithMessageID(id string) *Builder {
	b.messageID = id

	return b
}

func (b *Builder) WithReasoning(enabled bool) *Builder {
	b.opts.SendReasoning = enabled

	return b
}

func (b *Builder) WithSources(enabled bool) *Builder {
	b.opts.SendSources = enabled

	return b
}

func (b *Builder) WithStart(enabled bool) *Builder {
	b.opts.SendStart = enabled

	return b
}

func (b *Builder) WithFinish(enabled bool) *Builder {
	b.opts.SendFinish = enabled

	return b
}

func (b *Builder) OnError(handler func(err error) string) *Builder {
	b.opts.OnError = handler

	return b
}

func (b *Builder) OnFinish(handler func(content string)) *Builder {
	b.opts.OnFinish = handler

	return b
}

func (b *Builder) WithIDGenerator(gen func(prefix string) string) *Builder {
	b.opts.GenerateID = gen

	return b
}

func (b *Builder) WithHeader(key, value string) *Builder {
	b.headers[key] = value

	return b
}

// Stream executes the stream and writes to a Fiber context.
func (b *Builder) Stream(ctx fiber.Ctx) error {
	if b.source == nil {
		return ErrSourceRequired
	}

	for k, v := range SseHeaders {
		ctx.Set(k, v)
	}

	for k, v := range b.headers {
		ctx.Set(k, v)
	}

	return ctx.SendStreamWriter(func(w *bufio.Writer) {
		b.doStreamWrite(w)
	})
}

// StreamToWriter streams messages to a bufio.Writer.
func (b *Builder) StreamToWriter(w *bufio.Writer) {
	b.doStreamWrite(w)
}

func (b *Builder) doStreamWrite(w *bufio.Writer) {
	defer func() { _ = b.source.Close() }()

	writer := newSseWriter(w)

	generateID := b.opts.GenerateID
	if generateID == nil {
		generateID = defaultIDGenerator
	}

	messageID := b.messageID
	if messageID == "" {
		messageID = generateID("message")
	}

	textID := generateID("text")
	reasoningID := generateID("reasoning")
	textStarted := false
	reasoningStarted := false

	var fullContent string

	if b.opts.SendStart {
		_ = writer.WriteChunk(NewStartChunk(messageID))
		_ = writer.WriteChunk(NewStartStepChunk())
	}

	for {
		msg, err := b.source.Recv()
		if errors.Is(err, io.EOF) {
			if textStarted {
				_ = writer.WriteChunk(NewTextEndChunk(textID))
			}

			if reasoningStarted {
				_ = writer.WriteChunk(NewReasoningEndChunk(reasoningID))
			}

			if b.opts.SendFinish {
				_ = writer.WriteChunk(NewFinishStepChunk())
				_ = writer.WriteChunk(NewFinishChunk())
			}

			_ = writer.writeDone()

			if b.opts.OnFinish != nil {
				b.opts.OnFinish(fullContent)
			}

			return
		}

		if err != nil {
			errorText := err.Error()
			if b.opts.OnError != nil {
				errorText = b.opts.OnError(err)
			}

			_ = writer.WriteChunk(NewErrorChunk(errorText))
			_ = writer.writeDone()

			return
		}

		// Handle reasoning
		if b.opts.SendReasoning && msg.Reasoning != "" {
			if !reasoningStarted {
				_ = writer.WriteChunk(NewReasoningStartChunk(reasoningID))
				reasoningStarted = true
			}

			_ = writer.WriteChunk(NewReasoningDeltaChunk(reasoningID, msg.Reasoning))
		}

		// Handle tool calls
		for _, tc := range msg.ToolCalls {
			toolCallID := tc.ID
			if toolCallID == "" {
				toolCallID = generateID("call")
			}

			_ = writer.WriteChunk(NewToolInputStartChunk(toolCallID, tc.Name))

			var input any
			if err := json.Unmarshal([]byte(tc.Arguments), &input); err != nil {
				input = tc.Arguments
			}

			_ = writer.WriteChunk(NewToolInputAvailableChunk(toolCallID, tc.Name, input))
		}

		// Handle tool results
		if msg.Role == RoleTool && msg.ToolCallID != "" {
			var output any
			if err := json.Unmarshal([]byte(msg.Content), &output); err != nil {
				output = msg.Content
			}

			_ = writer.WriteChunk(NewToolOutputAvailableChunk(msg.ToolCallID, output))

			continue
		}

		// Handle custom data
		for dataType, data := range msg.Data {
			_ = writer.WriteChunk(NewDataChunk(dataType, data))
		}

		// Handle text content
		if msg.Content != "" {
			if reasoningStarted {
				_ = writer.WriteChunk(NewReasoningEndChunk(reasoningID))
				reasoningStarted = false
				reasoningID = generateID("reasoning")
			}

			if !textStarted {
				_ = writer.WriteChunk(NewTextStartChunk(textID))
				textStarted = true
			}

			_ = writer.WriteChunk(NewTextDeltaChunk(textID, msg.Content))
			fullContent += msg.Content
		}
	}
}
