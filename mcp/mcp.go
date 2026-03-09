package mcp

import "github.com/modelcontextprotocol/go-sdk/mcp"

// ToolProvider provides MCP tools to the server.
type ToolProvider interface {
	// Tools returns the list of tool definitions this provider registers.
	Tools() []ToolDefinition
}

// ResourceProvider provides static MCP resources to the server.
type ResourceProvider interface {
	// Resources returns the list of static resource definitions this provider registers.
	Resources() []ResourceDefinition
}

// ResourceTemplateProvider provides dynamic MCP resource templates to the server.
type ResourceTemplateProvider interface {
	// ResourceTemplates returns the list of resource template definitions this provider registers.
	ResourceTemplates() []ResourceTemplateDefinition
}

// PromptProvider provides MCP prompts to the server.
type PromptProvider interface {
	// Prompts returns the list of prompt definitions this provider registers.
	Prompts() []PromptDefinition
}

// ToolDefinition defines a tool and its handler.
type ToolDefinition struct {
	Tool    *Tool
	Handler ToolHandler
}

// ResourceDefinition defines a static resource and its handler.
type ResourceDefinition struct {
	Resource *Resource
	Handler  ResourceHandler
}

// ResourceTemplateDefinition defines a dynamic resource template and its handler.
type ResourceTemplateDefinition struct {
	Template *ResourceTemplate
	Handler  ResourceHandler
}

// PromptDefinition defines a prompt and its handler.
type PromptDefinition struct {
	Prompt  *Prompt
	Handler PromptHandler
}

// ServerInfo configures MCP server identification.
type ServerInfo struct {
	Name         string
	Version      string
	Instructions string
}

// Type aliases for MCP SDK types - users don't need to import the SDK directly.
type (
	Server         = mcp.Server
	ServerOptions  = mcp.ServerOptions
	ServerSession  = mcp.ServerSession
	Implementation = mcp.Implementation

	Tool            = mcp.Tool
	ToolHandler     = mcp.ToolHandler
	CallToolRequest = mcp.CallToolRequest
	CallToolResult  = mcp.CallToolResult

	Resource            = mcp.Resource
	ResourceTemplate    = mcp.ResourceTemplate
	ResourceHandler     = mcp.ResourceHandler
	ReadResourceRequest = mcp.ReadResourceRequest
	ReadResourceResult  = mcp.ReadResourceResult

	Prompt           = mcp.Prompt
	PromptHandler    = mcp.PromptHandler
	GetPromptRequest = mcp.GetPromptRequest
	GetPromptParams  = mcp.GetPromptParams
	GetPromptResult  = mcp.GetPromptResult
	PromptMessage    = mcp.PromptMessage
	PromptArgument   = mcp.PromptArgument

	Content      = mcp.Content
	TextContent  = mcp.TextContent
	ImageContent = mcp.ImageContent
	AudioContent = mcp.AudioContent

	Role        = mcp.Role
	Annotations = mcp.Annotations
)

// Function aliases.
var (
	ResourceNotFoundError = mcp.ResourceNotFoundError
)

// NewToolResultText creates a CallToolResult with text content.
func NewToolResultText(text string) *CallToolResult {
	return &CallToolResult{
		Content: []Content{&TextContent{Text: text}},
	}
}

// NewToolResultError creates a CallToolResult indicating an error.
func NewToolResultError(errMsg string) *CallToolResult {
	return &CallToolResult{
		Content: []Content{&TextContent{Text: errMsg}},
		IsError: true,
	}
}
