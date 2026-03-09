package ai

import "context"

// ChatModel defines the interface for chat-based language models.
type ChatModel interface {
	// Generate produces a response from the given messages synchronously.
	Generate(ctx context.Context, messages []*Message, opts ...Option) (*Message, error)
	// Stream produces a streaming response from the given messages.
	Stream(ctx context.Context, messages []*Message, opts ...Option) (MessageStream, error)
}

// ToolableChatModel is a chat model that supports tool calling.
type ToolableChatModel interface {
	ChatModel

	// WithTools returns a new model instance with the specified tools bound.
	// This follows the immutable pattern - it does not modify the current instance.
	WithTools(tools ...Tool) ToolableChatModel
}

// ModelInfo contains information about a model.
type ModelInfo struct {
	// Provider is the name of the model provider.
	Provider string
	// Model is the name of the model.
	Model string
	// MaxTokens is the maximum context length.
	MaxTokens int
	// Temperature is the default temperature setting.
	Temperature float64
}
