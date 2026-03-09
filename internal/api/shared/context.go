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
