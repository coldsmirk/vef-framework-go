package httpx

import "github.com/gofiber/fiber/v3"

// GetIP returns the client IP resolved by Fiber. When the application configures
// trusted proxies (vef.app.trusted_proxies), this honors X-Forwarded-For from
// those proxies; otherwise it is the direct connection peer. A raw, client-supplied
// X-Forwarded-For is never trusted.
func GetIP(ctx fiber.Ctx) string {
	return ctx.IP()
}
