package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/internal/app"
)

func NewCorsMiddleware(config *config.CorsConfig) app.Middleware {
	handler := cors.New(cors.Config{
		Next: func(_ fiber.Ctx) bool {
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
			fiber.HeaderAuthorization,
			fiber.HeaderXRequestedWith,
			fiber.HeaderXRequestID,
			api.HeaderXAppID,
			api.HeaderXTimestamp,
			api.HeaderXNonce,
			api.HeaderXSignature,
		},
		AllowCredentials: false,
		ExposeHeaders:    []string{},
		MaxAge:           7200,
	})

	return &SimpleMiddleware{
		handler: handler,
		name:    "cors",
		order:   -800,
	}
}
