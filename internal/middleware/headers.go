package middleware

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/internal/app"
)

// NewHeadersMiddleware sets headers after handler execution to avoid being overwritten by application code.
func NewHeadersMiddleware() app.Middleware {
	return &SimpleMiddleware{
		handler: func(ctx fiber.Ctx) error {
			if err := ctx.Next(); err != nil {
				return err
			}

			ctx.Set(fiber.HeaderXContentTypeOptions, "nosniff")

			// Scheme() reports the request scheme (http/https), honoring
			// X-Forwarded-Proto only behind a trusted proxy; Protocol() returns
			// the HTTP wire version (e.g. "HTTP/1.1") and never equals "https".
			if ctx.Scheme() == "https" {
				ctx.Set(fiber.HeaderStrictTransportSecurity, "max-age=31536000; includeSubDomains")
			}

			if len(ctx.Response().Header.Peek(fiber.HeaderCacheControl)) == 0 {
				ctx.Set(fiber.HeaderCacheControl, "no-store, no-cache, must-revalidate, max-age=0")
			}

			return nil
		},
		name:  "headers",
		order: -900,
	}
}
