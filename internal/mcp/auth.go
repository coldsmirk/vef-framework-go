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

		return &auth.TokenInfo{
			Expiration: time.Now().Add(24 * time.Hour),
			Extra: map[string]any{
				"principal": principal,
			},
		}, nil
	}
}
