package expression

import (
	"reflect"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/expression"
)

// engineResolver injects the expression.Engine into API handlers.
type engineResolver struct {
	engine expression.Engine
}

// NewEngineResolver creates a handler parameter resolver for expression.Engine.
func NewEngineResolver(engine expression.Engine) api.HandlerParamResolver {
	return &engineResolver{engine: engine}
}

func (*engineResolver) Type() reflect.Type {
	return reflect.TypeFor[expression.Engine]()
}

func (r *engineResolver) Resolve(fiber.Ctx) (reflect.Value, error) {
	return reflect.ValueOf(r.engine), nil
}
