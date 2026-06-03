//nolint:revive // package name is intentional
package shared

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
)

type contextKey uint

const (
	contextKeyRequest contextKey = iota
	contextKeyOperation
	contextKeyHandler
)

func Request(ctx fiber.Ctx) *api.Request {
	return fiber.Locals[*api.Request](ctx, contextKeyRequest)
}

func SetRequest(ctx fiber.Ctx, req *api.Request) {
	fiber.Locals(ctx, contextKeyRequest, req)
}

func Operation(ctx fiber.Ctx) *api.Operation {
	return fiber.Locals[*api.Operation](ctx, contextKeyOperation)
}

func SetOperation(ctx fiber.Ctx, op *api.Operation) {
	fiber.Locals(ctx, contextKeyOperation, op)
}

// Handler returns the resolved fiber.Handler stored in context by the RPC resolver.
func Handler(ctx fiber.Ctx) fiber.Handler {
	return fiber.Locals[fiber.Handler](ctx, contextKeyHandler)
}

// SetHandler stores the resolved fiber.Handler in the request context.
func SetHandler(ctx fiber.Ctx, h fiber.Handler) {
	fiber.Locals(ctx, contextKeyHandler, h)
}
