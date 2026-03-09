package middleware

import (
	"fmt"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/internal/log"
)

// NewLoggerMiddleware creates request-scoped loggers to correlate all log entries within a request.
func NewLoggerMiddleware() app.Middleware {
	return &SimpleMiddleware{
		handler: func(ctx fiber.Ctx) error {
			requestID := requestid.FromContext(ctx)
			logger := log.Named(fmt.Sprintf("request_id:%s", requestID))
			contextx.SetLogger(ctx, logger)
			contextx.SetRequestID(ctx, requestID)

			ctx.SetContext(
				contextx.SetLogger(
					contextx.SetRequestID(ctx.Context(), requestID),
					logger,
				),
			)

			return ctx.Next()
		},
		name:  "logger",
		order: -600,
	}
}
