package mcp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"

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

		// AuthManager.Authenticate is the source of truth: it parses the JWT and
		// rejects expired tokens (via the exp claim) before we reach this point,
		// so the credential is already known to be valid here. The SDK still
		// requires a non-zero, future Expiration as a structural gate
		// (auth.verify rejects a zero or past value), but it does not expose the
		// underlying token's real exp through AuthManager. We therefore set a
		// far-future sentinel to satisfy the gate without asserting a misleading
		// lifetime; real expiry stays owned by AuthManager.
		return &auth.TokenInfo{
			Expiration: validatedUpstream,
			Extra: map[string]any{
				"principal": principal,
			},
		}, nil
	}
}

// validatedUpstream is the SDK Expiration sentinel used when the token has
// already been validated by AuthManager. It must be non-zero and in the future
// to pass auth.verify; its absolute value carries no meaning.
var validatedUpstream = time.Date(9999, time.December, 31, 23, 59, 59, 0, time.UTC)
