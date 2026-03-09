package ai

import "context"

// Agent represents an AI agent that can reason and use tools.
type Agent interface {
	// Run executes the agent synchronously with the given input.
	Run(ctx context.Context, input string, opts ...Option) (*Message, error)
	// Stream executes the agent and returns a streaming response.
	Stream(ctx context.Context, input string, opts ...Option) (MessageStream, error)
}

// AgentConfig contains configuration for creating an agent.
type AgentConfig struct {
	// Model is the chat model to use for reasoning.
	Model ToolableChatModel
	// Tools are the tools available to the agent.
	Tools []Tool
	// SystemPrompt is the system prompt that guides the agent's behavior.
	SystemPrompt string
	// MaxIterations limits the maximum number of reasoning iterations.
	MaxIterations int
}

// AgentBuilder provides a fluent interface for building agents.
type AgentBuilder interface {
	// WithModel sets the chat model for the agent.
	WithModel(model ToolableChatModel) AgentBuilder
	// WithTools adds tools to the agent.
	WithTools(tools ...Tool) AgentBuilder
	// WithSystemPrompt sets the system prompt.
	WithSystemPrompt(prompt string) AgentBuilder
	// WithMaxIterations sets the maximum number of iterations.
	WithMaxIterations(n int) AgentBuilder
	// Build creates the agent instance.
	Build(ctx context.Context) (Agent, error)
}
