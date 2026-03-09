package middleware

import (
	"runtime/debug"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"

	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/app"
)

func NewRecoveryMiddleware() app.Middleware {
	handler := recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(ctx fiber.Ctx, err any) {
			logger := contextx.Logger(ctx)
			logger.Errorf("Panic: %v\n%s", err, debug.Stack())
		},
	})

	return &SimpleMiddleware{
		handler: handler,
		name:    "recovery",
		order:   -500,
	}
}
