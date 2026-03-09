package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/auth"

	"github.com/coldsmirk/vef-framework-go/orm"
	"github.com/coldsmirk/vef-framework-go/security"
)

// GetPrincipalFromContext extracts the Principal from MCP request context.
func GetPrincipalFromContext(ctx context.Context) *security.Principal {
	if tokenInfo := auth.TokenInfoFromContext(ctx); tokenInfo != nil && tokenInfo.Extra != nil {
		if principal, ok := tokenInfo.Extra["principal"].(*security.Principal); ok {
			return principal
		}
	}

	return security.PrincipalAnonymous
}

// DBWithOperator returns a database connection with the operator ID bound from the MCP context.
func DBWithOperator(ctx context.Context, db orm.DB) orm.DB {
	principal := GetPrincipalFromContext(ctx)

	return db.WithNamedArg(orm.PlaceholderKeyOperator, principal.ID)
}
