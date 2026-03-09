package middleware

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/httpx"
	"github.com/coldsmirk/vef-framework-go/internal/app"
)

// NewContentTypeMiddleware enforces JSON/multipart for POST/PUT requests.
func NewContentTypeMiddleware() app.Middleware {
	return &SimpleMiddleware{
		handler: func(ctx fiber.Ctx) error {
			method := ctx.Method()

			isStateChanging := method == fiber.MethodPost || method == fiber.MethodPut
			if !isStateChanging || httpx.IsJSON(ctx) || httpx.IsMultipart(ctx) {
				return ctx.Next()
			}

			return fiber.ErrUnsupportedMediaType
		},
		name:  "content_type",
		order: -700,
	}
}
