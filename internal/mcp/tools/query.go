package tools

import (
	"context"
	"encoding/json"
	"unicode/utf8"

	"github.com/coldsmirk/vef-framework-go/internal/orm/sqlguard"
	"github.com/coldsmirk/vef-framework-go/mcp"
	"github.com/coldsmirk/vef-framework-go/orm"
)

// QueryArgs defines the expected arguments for the database_query tool.
type QueryArgs struct {
	SQL    string `json:"sql" jsonschema:"required,description=The SQL query with placeholders (?) for parameters"`
	Params []any  `json:"params,omitempty" jsonschema:"description=Parameters for the SQL query placeholders"`
}

// QueryTool provides MCP tool for executing parameterized SQL queries.
type QueryTool struct {
	db orm.DB
}

// NewQueryTool creates a new QueryTool instance.
func NewQueryTool(db orm.DB) mcp.ToolProvider {
	return &QueryTool{db: db}
}

// Tools implements mcp.ToolProvider.
func (t *QueryTool) Tools() []mcp.ToolDefinition {
	return []mcp.ToolDefinition{
		{
			Tool: &mcp.Tool{
				Name:        "database_query",
				Description: "Execute a read-only (SELECT) parameterized SQL query against the database. Returns query results as JSON array.",
				InputSchema: mcp.MustSchemaFor[QueryArgs](),
			},
			Handler: t.handleQuery,
		},
	}
}

// handleQuery executes a parameterized SQL query.
func (t *QueryTool) handleQuery(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args QueryArgs
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		//nolint:nilerr // MCP handler should return error result with nil error
		return mcp.NewToolResultError("Failed to parse arguments: " + err.Error()), nil
	}

	if args.SQL == "" {
		return mcp.NewToolResultError("Sql parameter is required and must not be empty"), nil
	}

	if err := sqlguard.EnsureReadOnly(args.SQL); err != nil {
		//nolint:nilerr // MCP handler should return error result with nil error
		return mcp.NewToolResultError("Only read-only (SELECT) queries are permitted: " + err.Error()), nil
	}

	db := mcp.DBWithOperator(ctx, t.db)

	var results []map[string]any
	if err := db.NewRaw(args.SQL, args.Params...).Scan(ctx, &results); err != nil {
		//nolint:nilerr // MCP handler should return error result with nil error
		return mcp.NewToolResultError("Query execution failed: " + err.Error()), nil
	}

	for _, result := range results {
		for k, v := range result {
			result[k] = convertByteSlices(v)
		}
	}

	jsonBytes, err := json.Marshal(results)
	if err != nil {
		//nolint:nilerr // MCP handler should return error result with nil error
		return mcp.NewToolResultError("Failed to encode results: " + err.Error()), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// convertByteSlices recursively converts []byte values that hold valid UTF-8
// text (e.g. PostgreSQL char/varchar fields) into strings, descending into
// nested maps and slices so array/JSON columns are handled consistently with
// scalar text columns. Binary data (e.g. BYTEA/BLOB) is left as []byte and will
// be Base64-encoded in JSON. The converted value is returned.
func convertByteSlices(v any) any {
	switch val := v.(type) {
	case []byte:
		if utf8.Valid(val) {
			return string(val)
		}

		return val

	case map[string]any:
		for k, elem := range val {
			val[k] = convertByteSlices(elem)
		}

		return val

	case []any:
		for i, elem := range val {
			val[i] = convertByteSlices(elem)
		}

		return val

	default:
		return val
	}
}
