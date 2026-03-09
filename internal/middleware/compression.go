package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"

	"github.com/coldsmirk/vef-framework-go/internal/app"
)

func NewCompressionMiddleware() app.Middleware {
	handler := compress.New(compress.Config{
		Level: compress.LevelDefault,
		Next: func(c fiber.Ctx) bool {
			// Skip compression for SSE responses
			return strings.Contains(c.Get(fiber.HeaderAccept), "text/event-stream")
		},
	})

	return &SimpleMiddleware{
		handler: handler,
		name:    "compression",
		order:   -1000,
	}
}
