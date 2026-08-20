package fiberx

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

// GetIP returns the client IP resolved by Fiber. When the application configures
// trusted proxies (vef.app.trusted_proxies), this honors X-Forwarded-For from
// those proxies; otherwise it is the direct connection peer. A raw, client-supplied
// X-Forwarded-For is never trusted.
//
// The result is always a copy. Behind a trusted proxy Fiber returns a slice of
// the proxy header, and the framework builds Fiber with Immutable off, so that
// slice is a view into the pooled request buffer: the bytes belong to whatever
// request reuses the buffer once the handler returns. Copying here rather than
// at each retention point is what makes every caller safe by construction — a
// rate-limit key that is discarded within the request and an audit record that
// outlives it read the same accessor, and nothing about the second call site
// looks more dangerous than the first.
func GetIP(ctx fiber.Ctx) string {
	return strings.Clone(ctx.IP())
}
