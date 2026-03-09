package auth

import (
	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/security"
)

// NoneStrategy implements api.AuthStrategy for public endpoints.
type NoneStrategy struct{}

// NewNone creates a new none authentication strategy.
func NewNone() api.AuthStrategy {
	return &NoneStrategy{}
}

// Name returns the strategy name.
func (*NoneStrategy) Name() string {
	return api.AuthStrategyNone
}

// Authenticate returns anonymous principal.
func (*NoneStrategy) Authenticate(fiber.Ctx, map[string]any) (*security.Principal, error) {
	return security.PrincipalAnonymous, nil
}
