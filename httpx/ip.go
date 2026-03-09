package httpx

import "github.com/gofiber/fiber/v3"

// GetIP retrieves X-Forwarded-For header or falls back to direct IP.
func GetIP(ctx fiber.Ctx) string {
	return ctx.Get(fiber.HeaderXForwardedFor, ctx.IP())
}
