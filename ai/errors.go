package ai

import "errors"

var (
	// ErrStreamClosed is returned when attempting to read from a closed stream.
	ErrStreamClosed = errors.New("ai: stream is closed")
	// ErrNoContent is returned when the model returns no content.
	ErrNoContent = errors.New("ai: no content in response")
	// ErrMaxIterationsReached is returned when an agent exceeds its iteration limit.
	ErrMaxIterationsReached = errors.New("ai: maximum iterations reached")
	// ErrToolNotFound is returned when a requested tool is not available.
	ErrToolNotFound = errors.New("ai: tool not found")
	// ErrInvalidArguments is returned when tool arguments are invalid.
	ErrInvalidArguments = errors.New("ai: invalid tool arguments")
	// ErrProviderNotFound is returned when a model provider is not registered.
	ErrProviderNotFound = errors.New("ai: provider not found")
	// ErrModelNotSupported is returned when a model is not supported by the provider.
	ErrModelNotSupported = errors.New("ai: model not supported")
	// ErrAgentNotFound is returned when an agent type is not registered.
	ErrAgentNotFound = errors.New("ai: agent type not found")
)

// ToolError represents an error that occurred during tool execution.
type ToolError struct {
	ToolName string
	Err      error
}

func (e *ToolError) Error() string {
	return "ai: tool " + e.ToolName + ": " + e.Err.Error()
}

func (e *ToolError) Unwrap() error {
	return e.Err
}

// NewToolError creates a new ToolError.
func NewToolError(toolName string, err error) *ToolError {
	return &ToolError{
		ToolName: toolName,
		Err:      err,
	}
}

// ModelError represents an error from the model API.
type ModelError struct {
	Provider   string
	StatusCode int
	Message    string
}

func (e *ModelError) Error() string {
	return "ai: " + e.Provider + ": " + e.Message
}

// NewModelError creates a new ModelError.
func NewModelError(provider string, statusCode int, message string) *ModelError {
	return &ModelError{
		Provider:   provider,
		StatusCode: statusCode,
		Message:    message,
	}
}
