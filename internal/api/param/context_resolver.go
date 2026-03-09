package param

import (
	"reflect"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
)

// contextResolver is a generic resolver that extracts a value from the request context.
type contextResolver[T any] struct {
	extract func(fiber.Ctx) T
}

func (*contextResolver[T]) Type() reflect.Type {
	return reflect.TypeFor[T]()
}

func (r *contextResolver[T]) Resolve(ctx fiber.Ctx) (reflect.Value, error) {
	return reflect.ValueOf(r.extract(ctx)), nil
}

// newContextResolver creates a HandlerParamResolver that extracts a value from fiber.Ctx.
func newContextResolver[T any](extract func(fiber.Ctx) T) api.HandlerParamResolver {
	return &contextResolver[T]{extract: extract}
}
