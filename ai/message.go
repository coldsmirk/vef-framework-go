package ai

// Role defines the message role type.
type Role string

const (
	// RoleSystem represents a system message that sets context or instructions.
	RoleSystem Role = "system"
	// RoleUser represents a message from the user.
	RoleUser Role = "user"
	// RoleAssistant represents a message from the AI assistant.
	RoleAssistant Role = "assistant"
	// RoleTool represents a message containing tool execution results.
	RoleTool Role = "tool"
)

// ToolCall represents a tool invocation request from the model.
type ToolCall struct {
	// ID is the unique identifier for this tool call.
	ID string
	// Name is the name of the tool to invoke.
	Name string
	// Arguments contains the JSON-encoded arguments for the tool.
	Arguments string
}

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	// CallID is the identifier of the corresponding ToolCall.
	CallID string
	// Content is the result content from the tool execution.
	Content string
}

// TokenUsage represents token consumption statistics.
type TokenUsage struct {
	// PromptTokens is the number of tokens in the prompt.
	PromptTokens int
	// CompletionTokens is the number of tokens in the completion.
	CompletionTokens int
	// TotalTokens is the total number of tokens used.
	TotalTokens int
}

// Message represents a chat message in a conversation.
type Message struct {
	// Role indicates who sent this message.
	Role Role
	// Content is the text content of the message.
	Content string
	// ToolCalls contains tool invocation requests (only for Assistant role).
	ToolCalls []ToolCall
	// ToolResult contains tool execution result (only for Tool role).
	ToolResult *ToolResult
	// Usage contains token usage statistics (only for response messages).
	Usage *TokenUsage
}

// NewSystemMessage creates a new system message.
func NewSystemMessage(content string) *Message {
	return &Message{
		Role:    RoleSystem,
		Content: content,
	}
}

// NewUserMessage creates a new user message.
func NewUserMessage(content string) *Message {
	return &Message{
		Role:    RoleUser,
		Content: content,
	}
}

// NewAssistantMessage creates a new assistant message.
func NewAssistantMessage(content string) *Message {
	return &Message{
		Role:    RoleAssistant,
		Content: content,
	}
}

// NewAssistantMessageWithToolCalls creates a new assistant message with tool calls.
func NewAssistantMessageWithToolCalls(content string, toolCalls []ToolCall) *Message {
	return &Message{
		Role:      RoleAssistant,
		Content:   content,
		ToolCalls: toolCalls,
	}
}

// NewToolMessage creates a new tool result message.
func NewToolMessage(callID, content string) *Message {
	return &Message{
		Role: RoleTool,
		ToolResult: &ToolResult{
			CallID:  callID,
			Content: content,
		},
	}
}

// IsSystem returns true if this is a system message.
func (m *Message) IsSystem() bool {
	return m.Role == RoleSystem
}

// IsUser returns true if this is a user message.
func (m *Message) IsUser() bool {
	return m.Role == RoleUser
}

// IsAssistant returns true if this is an assistant message.
func (m *Message) IsAssistant() bool {
	return m.Role == RoleAssistant
}

// IsTool returns true if this is a tool result message.
func (m *Message) IsTool() bool {
	return m.Role == RoleTool
}

// HasToolCalls returns true if this message contains tool calls.
func (m *Message) HasToolCalls() bool {
	return len(m.ToolCalls) > 0
}
