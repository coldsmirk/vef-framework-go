package mcp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/security"
)

// CreateTokenVerifier creates an auth.TokenVerifier that bridges MCP SDK auth
// with the vef's AuthManager, dispatching the deployment's configured token
// mechanism (jwt_token or opaque_token) so MCP accepts the same tokens the /api
// surface issues.
func CreateTokenVerifier(authManager security.AuthManager, authType string) auth.TokenVerifier {
	return func(ctx context.Context, tokenString string, _ *http.Request) (*auth.TokenInfo, error) {
		principal, err := authManager.Authenticate(ctx, security.Authentication{
			Type:      authType,
			Principal: tokenString,
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %w", auth.ErrInvalidToken, err)
		}

		// Under jwt_token the (already fully validated) JWT is re-parsed without
		// signature verification just to report its real exp claim to the MCP SDK.
		// An opaque token is not a JWT, so it takes the far-future fallback: the
		// SDK rejects a zero expiration, and per-request verification above — a
		// live session-store lookup — is what actually bounds its lifetime.
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
// Returns a far-future fallback time if the claim is absent or unparseable —
// including for opaque tokens, which are not JWTs at all.
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
