package param

import (
	"reflect"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
)

// handlerValueResolver resolves a handler parameter by returning a fixed value.
type handlerValueResolver[T any] struct {
	value T
}

func (*handlerValueResolver[T]) Type() reflect.Type {
	return reflect.TypeFor[T]()
}

func (r *handlerValueResolver[T]) Resolve(_ fiber.Ctx) (reflect.Value, error) {
	return reflect.ValueOf(r.value), nil
}

func newHandlerValueResolver[T any](value T) api.HandlerParamResolver {
	return &handlerValueResolver[T]{value: value}
}

// factoryValueResolver resolves a factory parameter by returning a fixed value.
type factoryValueResolver[T any] struct {
	value T
}

func (*factoryValueResolver[T]) Type() reflect.Type {
	return reflect.TypeFor[T]()
}

func (r *factoryValueResolver[T]) Resolve() (reflect.Value, error) {
	return reflect.ValueOf(r.value), nil
}

func newFactoryValueResolver[T any](value T) api.FactoryParamResolver {
	return &factoryValueResolver[T]{value: value}
}
