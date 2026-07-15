package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/fiberx"
	"github.com/coldsmirk/vef-framework-go/internal/app"
)

// apiPathPrefix mirrors the API routers' default mount path; the JSON guard
// applies inside it only.
const apiPathPrefix = "/api"

// NewContentTypeMiddleware enforces JSON/multipart for state-changing
// requests on the API surface, where the dispatcher parses JSON bodies.
// Other surfaces (MCP, the storage proxy, the integration inbound gateway)
// own their body formats — a vendor callback may legitimately POST XML.
func NewContentTypeMiddleware() app.Middleware {
	return &SimpleMiddleware{
		handler: func(ctx fiber.Ctx) error {
			method := ctx.Method()

			isStateChanging := method == fiber.MethodPost || method == fiber.MethodPut
			if !isStateChanging || !isAPIPath(ctx.Path()) || fiberx.IsJSON(ctx) || fiberx.IsMultipart(ctx) {
				return ctx.Next()
			}

			return fiber.ErrUnsupportedMediaType
		},
		name:  "content_type",
		order: -700,
	}
}

// isAPIPath reports whether the request path belongs to the API surface.
func isAPIPath(path string) bool {
	return path == apiPathPrefix || strings.HasPrefix(path, apiPathPrefix+"/")
}
