package auth

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/internal/api/shared"
	"github.com/coldsmirk/vef-framework-go/security"
)

var defaultTokenExtractor = extractors.Chain(
	extractors.FromAuthHeader(security.AuthSchemeBearer),
	extractors.FromQuery(security.QueryKeyAccessToken),
)

// TokenAuthenticator validates a token and returns the principal.
type TokenAuthenticator interface {
	// Authenticate validates the bearer token and returns the authenticated principal, or an error if invalid.
	Authenticate(ctx context.Context, token string) (*security.Principal, error)
}

// BearerStrategy implements api.AuthStrategy for Bearer token authentication.
type BearerStrategy struct {
	extractor      extractors.Extractor
	authenticators []TokenAuthenticator
}

// BearerOption configures BearerStrategy.
type BearerOption func(*BearerStrategy)

// WithTokenExtractor sets a custom token extractor.
func WithTokenExtractor(e extractors.Extractor) BearerOption {
	return func(s *BearerStrategy) {
		s.extractor = e
	}
}

// NewBearer creates a new Bearer token authentication strategy.
func NewBearer(authenticators []TokenAuthenticator, opts ...BearerOption) api.AuthStrategy {
	s := &BearerStrategy{
		authenticators: authenticators,
		extractor:      defaultTokenExtractor,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Name returns the strategy name.
func (*BearerStrategy) Name() string {
	return api.AuthStrategyBearer
}

// Authenticate validates the bearer token and returns the principal.
func (s *BearerStrategy) Authenticate(ctx fiber.Ctx, _ map[string]any) (*security.Principal, error) {
	token, err := s.extractor.Extract(ctx)
	if err != nil {
		return nil, s.wrapExtractError(ctx, err)
	}

	for _, auth := range s.authenticators {
		principal, err := auth.Authenticate(ctx.Context(), token)
		if err != nil {
			return nil, err
		}

		if principal != nil {
			return principal, nil
		}
	}

	return nil, ErrInvalidToken
}

func (*BearerStrategy) wrapExtractError(ctx fiber.Ctx, err error) error {
	if errors.Is(err, extractors.ErrNotFound) {
		err = fiber.ErrUnauthorized
	}

	if op := shared.Operation(ctx); op != nil {
		return &shared.BaseError{Identifier: &op.Identifier, Err: err}
	}

	return err
}
