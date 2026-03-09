package mcp

import (
	"github.com/gofiber/fiber/v3"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/internal/app"
)

const mcpPath = "/mcp"

// MCPMiddleware registers MCP routes if a handler is available.
type MCPMiddleware struct {
	handler *Handler
}

// MiddlewareParams contains dependencies for creating the middleware.
type MiddlewareParams struct {
	fx.In

	Handler *Handler `optional:"true"`
}

// NewMiddleware creates a new MCP middleware.
// Returns nil if no handler is available.
func NewMiddleware(params MiddlewareParams) app.Middleware {
	if params.Handler == nil {
		return nil
	}

	return &MCPMiddleware{handler: params.Handler}
}

func (*MCPMiddleware) Name() string {
	return "mcp"
}

func (*MCPMiddleware) Order() int {
	return 500
}

func (m *MCPMiddleware) Apply(router fiber.Router) {
	if m.handler == nil {
		return
	}

	router.All(mcpPath, m.handler.FiberHandler())
	logger.Infof("MCP endpoint registered at POST %s", mcpPath)
}
