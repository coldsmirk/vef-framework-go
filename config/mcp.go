package config

// MCPConfig defines MCP server settings.
type MCPConfig struct {
	Enabled bool `config:"enabled"`
	// RequireAuth gates the MCP HTTP endpoint behind bearer-token authentication.
	// It is a pointer so an unset value defaults to secure (auth required); set it
	// to false explicitly to allow anonymous access.
	RequireAuth *bool `config:"require_auth"`
}
