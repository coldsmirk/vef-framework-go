package app

import "github.com/gofiber/fiber/v3"

// CustomCtx is the framework's custom Fiber context.
// It embeds DefaultCtx so fiber.NewWithCustomCtx can register it as the
// per-request context type. Request-scoped state (logger, principal, db) is
// stored in the request's context.Context via the contextx package rather
// than as struct fields here.
type CustomCtx struct {
	fiber.DefaultCtx
}
