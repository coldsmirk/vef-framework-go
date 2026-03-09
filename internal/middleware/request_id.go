package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"github.com/coldsmirk/vef-framework-go/id"
	"github.com/coldsmirk/vef-framework-go/internal/app"
)

func NewRequestIDMiddleware() app.Middleware {
	handler := requestid.New(requestid.Config{
		Generator: id.GenerateUUID,
		Header:    fiber.HeaderXRequestID,
	})

	return &SimpleMiddleware{
		handler: handler,
		name:    "request_id",
		order:   -650,
	}
}
