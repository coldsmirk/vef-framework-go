package config

// MCPConfig defines MCP server settings.
type MCPConfig struct {
	Enabled     bool `config:"enabled"`
	RequireAuth bool `config:"require_auth"`
}
