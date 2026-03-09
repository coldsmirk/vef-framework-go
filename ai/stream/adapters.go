package stream

import (
	"io"

	"github.com/coldsmirk/vef-framework-go/ai"
)

// Role represents the role of a message sender.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
	RoleSystem    Role = "system"
)

// Message represents a single message in the stream.
type Message struct {
	Role       Role
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
	Reasoning  string
	Data       map[string]any
}

// ToolCall represents a tool invocation by the AI.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// MessageSource produces streaming messages. Returns io.EOF when complete.
type MessageSource interface {
	// Recv receives the next message from the stream. Returns io.EOF when no more messages are available.
	Recv() (Message, error)
	// Close releases resources associated with this message source.
	Close() error
}

// Channel adapter

type channelSource struct {
	ch     <-chan Message
	closed bool
}

// NewChannelSource creates an adapter for a Go channel.
func NewChannelSource(ch <-chan Message) MessageSource {
	return &channelSource{ch: ch}
}

func (c *channelSource) Recv() (Message, error) {
	if c.closed {
		return Message{}, io.EOF
	}

	msg, ok := <-c.ch
	if !ok {
		c.closed = true

		return Message{}, io.EOF
	}

	return msg, nil
}

func (c *channelSource) Close() error {
	c.closed = true

	return nil
}

// FromChannel creates a Builder with a channel as the source.
func FromChannel(ch <-chan Message) *Builder {
	return New().WithSource(NewChannelSource(ch))
}

// Callback adapter

type callbackSource struct {
	messages chan Message
	done     chan struct{}
	err      error
}

// NewCallbackSource creates a callback-based message source.
func NewCallbackSource(execute func(writer CallbackWriter) error) MessageSource {
	s := &callbackSource{
		messages: make(chan Message, 16),
		done:     make(chan struct{}),
	}
	go func() {
		defer close(s.messages)
		defer close(s.done)

		s.err = execute(callbackWriterImpl{ch: s.messages})
	}()

	return s
}

func (c *callbackSource) Recv() (Message, error) {
	msg, ok := <-c.messages
	if !ok {
		if c.err != nil {
			return Message{}, c.err
		}

		return Message{}, io.EOF
	}

	return msg, nil
}

func (c *callbackSource) Close() error {
	<-c.done

	return nil
}

// FromCallback creates a Builder with a callback-based source.
func FromCallback(execute func(writer CallbackWriter) error) *Builder {
	return New().WithSource(NewCallbackSource(execute))
}

type callbackWriterImpl struct {
	ch chan<- Message
}

func (w callbackWriterImpl) WriteText(content string) {
	w.ch <- Message{Role: RoleAssistant, Content: content}
}

func (w callbackWriterImpl) WriteToolCall(id, name, arguments string) {
	w.ch <- Message{
		Role: RoleAssistant,
		ToolCalls: []ToolCall{{
			ID:        id,
			Name:      name,
			Arguments: arguments,
		}},
	}
}

func (w callbackWriterImpl) WriteToolResult(toolCallID, content string) {
	w.ch <- Message{
		Role:       RoleTool,
		ToolCallID: toolCallID,
		Content:    content,
	}
}

func (w callbackWriterImpl) WriteReasoning(reasoning string) {
	w.ch <- Message{Role: RoleAssistant, Reasoning: reasoning}
}

func (w callbackWriterImpl) WriteData(dataType string, data any) {
	w.ch <- Message{
		Role: RoleAssistant,
		Data: map[string]any{dataType: data},
	}
}

func (w callbackWriterImpl) WriteMessage(msg Message) {
	w.ch <- msg
}

// AI MessageStream adapter

type aiMessageStreamSource struct {
	stream ai.MessageStream
}

// NewAiMessageStreamSource creates an adapter for ai.MessageStream.
func NewAiMessageStreamSource(stream ai.MessageStream) MessageSource {
	return &aiMessageStreamSource{stream: stream}
}

func (a *aiMessageStreamSource) Recv() (Message, error) {
	chunk, err := a.stream.Recv()
	if err != nil {
		return Message{}, err
	}

	msg := Message{
		Role:    RoleAssistant,
		Content: chunk.Content,
	}

	if len(chunk.ToolCalls) > 0 {
		msg.ToolCalls = make([]ToolCall, len(chunk.ToolCalls))
		for i, tc := range chunk.ToolCalls {
			msg.ToolCalls[i] = ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			}
		}
	}

	return msg, nil
}

func (a *aiMessageStreamSource) Close() error {
	return a.stream.Close()
}

// FromAiMessageStream creates a Builder with an ai.MessageStream as the source.
func FromAiMessageStream(stream ai.MessageStream) *Builder {
	return New().WithSource(NewAiMessageStreamSource(stream))
}
