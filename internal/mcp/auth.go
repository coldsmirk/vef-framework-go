package mcp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/spf13/cast"

	isecurity "github.com/coldsmirk/vef-framework-go/internal/security"
	"github.com/coldsmirk/vef-framework-go/security"
)

// CreateTokenVerifier creates an auth.TokenVerifier that bridges MCP SDK auth
// with the vef's AuthManager.
func CreateTokenVerifier(authManager security.AuthManager) auth.TokenVerifier {
	return func(ctx context.Context, tokenString string, _ *http.Request) (*auth.TokenInfo, error) {
		principal, err := authManager.Authenticate(ctx, security.Authentication{
			Type:      isecurity.AuthTypeToken,
			Principal: tokenString,
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %w", auth.ErrInvalidToken, err)
		}

		// authManager.Authenticate already validated the JWT (signature, exp, issuer,
		// audience). Parse it without re-verifying the signature so we can read the real
		// exp claim and report it accurately to the MCP SDK.
		expiration := jwtExpiration(tokenString)

		return &auth.TokenInfo{
			Expiration: expiration,
			Extra: map[string]any{
				"principal": principal,
			},
		}, nil
	}
}

// jwtExpiration parses the exp claim from a JWT without verifying its signature.
// The caller must have already authenticated the token via AuthManager; this is
// a read-only claim extraction performed after validation succeeds.
// Returns a far-future fallback time if the claim is absent or unparseable.
func jwtExpiration(tokenString string) time.Time {
	var claims jwt.MapClaims

	_, _, err := jwt.NewParser().ParseUnverified(tokenString, &claims)
	if err != nil {
		return time.Date(9999, time.December, 31, 23, 59, 59, 0, time.UTC)
	}

	exp := cast.ToInt64(claims["exp"])
	if exp <= 0 {
		return time.Date(9999, time.December, 31, 23, 59, 59, 0, time.UTC)
	}

	return time.Unix(exp, 0)
}
