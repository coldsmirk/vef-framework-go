package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/app"
)

func NewCORSMiddleware(config *config.CORSConfig) app.Middleware {
	handler := cors.New(cors.Config{
		Next: func(fiber.Ctx) bool {
			return !config.Enabled
		},
		AllowOrigins: config.AllowOrigins,
		AllowMethods: []string{
			fiber.MethodHead,
			fiber.MethodGet,
			fiber.MethodPost,
			fiber.MethodPut,
			fiber.MethodDelete,
		},
		AllowHeaders: []string{
			fiber.HeaderContentType,
			fiber.HeaderContentEncoding,
			fiber.HeaderAuthorization,
			fiber.HeaderXRequestedWith,
			fiber.HeaderXRequestID,
			api.HeaderXAppID,
			api.HeaderXTimestamp,
			api.HeaderXNonce,
			api.HeaderXSignature,
			api.HeaderXBodyEncoding,
		},
		AllowCredentials: false,
		// A cross-origin fetch() can only read headers listed here. The
		// storage proxy advertises the uploaded file's original name in
		// Content-Disposition, so a browser download built on fetch (rather
		// than plain navigation) needs it exposed.
		ExposeHeaders: []string{
			fiber.HeaderContentDisposition,
		},
		MaxAge: 7200,
	})

	return &SimpleMiddleware{
		handler: handler,
		name:    "cors",
		order:   -800,
	}
}
