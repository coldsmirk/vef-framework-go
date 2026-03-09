package middleware

import (
	"github.com/gofiber/fiber/v3"
)

type SimpleMiddleware struct {
	path    string
	handler fiber.Handler
	name    string
	order   int
}

func (m *SimpleMiddleware) Name() string {
	return m.name
}

func (m *SimpleMiddleware) Order() int {
	return m.order
}

func (m *SimpleMiddleware) Apply(router fiber.Router) {
	if m.path == "" {
		router.Use(m.handler)

		return
	}

	router.Use(m.path, m.handler)
}
