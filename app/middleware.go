package app

import "github.com/gofiber/fiber/v3"

// Middleware is the route-assembly contract the framework applies to the HTTP
// router at boot. It is the extension point for anything an application needs
// to mount outside the /api surface — a webhook receiver, a single-sign-on
// gateway, a health probe — and is registered with vef.ProvideMiddleware.
//
// The contract lives in this public package rather than beside its
// implementation because fx collects the middleware group by exact type: a
// contract an application cannot name is a contract it cannot satisfy, and the
// group would silently drop whatever it provided instead.
type Middleware interface {
	// Name returns the name of the middleware.
	Name() string
	// Order returns the order of the middleware. Negative orders register
	// before the route handlers and positive orders after, each sorted
	// ascending; the zero default registers in the before group.
	Order() int
	// Apply applies the middleware to the router.
	Apply(router fiber.Router)
}
