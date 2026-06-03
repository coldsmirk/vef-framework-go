package middleware

import (
	"fmt"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"github.com/coldsmirk/vef-framework-go/contextx"
	"github.com/coldsmirk/vef-framework-go/internal/app"
	"github.com/coldsmirk/vef-framework-go/internal/logx"
)

// NewLoggerMiddleware creates request-scoped loggers to correlate all log entries within a request.
func NewLoggerMiddleware() app.Middleware {
	return &SimpleMiddleware{
		handler: func(ctx fiber.Ctx) error {
			requestID := requestid.FromContext(ctx)
			logger := logx.Named(fmt.Sprintf("request_id:%s", requestID))

			// Write both lookup paths: the fiber.Ctx variants store into Locals
			// for fiber-handler lookups, while the embedded context.Context carries
			// the values for non-fiber consumers reached via ctx.Context().
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
