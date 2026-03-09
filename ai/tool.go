package ai

import "context"

// ToolInfo contains metadata about a tool.
type ToolInfo struct {
	// Name is the unique identifier of the tool.
	Name string
	// Description explains what the tool does.
	Description string
	// Parameters defines the input schema for the tool.
	Parameters *ParameterSchema
}

// ParameterSchema defines the JSON Schema for tool parameters.
type ParameterSchema struct {
	// Type is the schema type, typically "object".
	Type string
	// Properties defines the parameter properties.
	Properties map[string]*PropertySchema
	// Required lists the required parameter names.
	Required []string
}

// PropertySchema defines a single parameter property.
type PropertySchema struct {
	// Type is the property type: string, number, integer, boolean, array, object.
	Type string
	// Description explains what this parameter is for.
	Description string
	// Enum lists allowed values if this is an enumeration.
	Enum []string
	// Items defines the schema for array items (only for array type).
	Items *PropertySchema
}

// Tool represents a callable tool that can be used by AI models.
type Tool interface {
	// Info returns the tool's metadata.
	Info() *ToolInfo
	// Invoke executes the tool with the given JSON-encoded arguments.
	// Returns the result as a string.
	Invoke(ctx context.Context, arguments string) (string, error)
}

// StreamableTool is a tool that supports streaming output.
type StreamableTool interface {
	Tool

	// InvokeStream executes the tool and returns a streaming result.
	InvokeStream(ctx context.Context, arguments string) (StringStream, error)
}
