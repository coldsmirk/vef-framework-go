package adapter

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
)

type FiberHandler struct{}

func NewFiberHandler() api.HandlerAdapter {
	return new(FiberHandler)
}

func (*FiberHandler) Adapt(handler any, _ *api.Operation) (fiber.Handler, error) {
	if h, ok := handler.(fiber.Handler); ok {
		return h, nil
	}

	return nil, nil
}
