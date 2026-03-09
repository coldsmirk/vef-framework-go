package mcp

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/fx"

	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/security"
)

type Handler struct {
	httpHandler http.Handler
}

type HandlerParams struct {
	fx.In

	MCPConfig   *config.MCPConfig
	Server      *mcp.Server `optional:"true"`
	AuthManager security.AuthManager
}

func NewHandler(params HandlerParams) *Handler {
	if params.Server == nil {
		return nil
	}

	httpHandler := createHTTPHandler(params.Server)
	if params.MCPConfig.RequireAuth {
		httpHandler = applyAuthMiddleware(httpHandler, params.AuthManager)
	}

	return &Handler{httpHandler: httpHandler}
}

func createHTTPHandler(server *mcp.Server) http.Handler {
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{},
	)
}

func applyAuthMiddleware(handler http.Handler, authManager security.AuthManager) http.Handler {
	verifier := CreateTokenVerifier(authManager)

	return auth.RequireBearerToken(verifier, nil)(handler)
}

func (h *Handler) FiberHandler() fiber.Handler {
	return adaptor.HTTPHandler(h.httpHandler)
}
