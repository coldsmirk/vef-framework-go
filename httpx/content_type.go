package httpx

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

// IsJSON checks if the request content type is JSON.
func IsJSON(ctx fiber.Ctx) bool {
	return ctx.Is("json")
}

// IsMultipart checks if the request content type is multipart/form-data.
func IsMultipart(ctx fiber.Ctx) bool {
	return strings.HasPrefix(ctx.Get(fiber.HeaderContentType), fiber.MIMEMultipartForm)
}
