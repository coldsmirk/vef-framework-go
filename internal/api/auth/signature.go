package auth

import (
	"github.com/gofiber/fiber/v3"
	"github.com/spf13/cast"

	"github.com/coldsmirk/vef-framework-go/api"
	isecurity "github.com/coldsmirk/vef-framework-go/internal/security"
	"github.com/coldsmirk/vef-framework-go/result"
	"github.com/coldsmirk/vef-framework-go/security"
)

// SignatureStrategy implements api.AuthStrategy for HMAC signature authentication.
// It extracts credentials from HTTP headers and delegates authentication
// to the security.AuthManager, following the Spring Security pattern.
//
// Required headers:
//   - X-App-ID: Application identifier (used as Principal)
//   - X-Timestamp: Unix timestamp in seconds
//   - X-Nonce: Random string for replay attack prevention
//   - X-Signature: HMAC signature in hex encoding
type SignatureStrategy struct {
	authManager security.AuthManager
}

// NewSignature creates a new signature authentication strategy.
// The authManager is used to delegate the actual authentication to SignatureAuthenticator.
func NewSignature(authManager security.AuthManager) api.AuthStrategy {
	return &SignatureStrategy{
		authManager: authManager,
	}
}

// Name returns the strategy name.
func (*SignatureStrategy) Name() string {
	return api.AuthStrategySignature
}

// Authenticate extracts credentials from request headers and delegates
// authentication to the AuthManager.
// Headers are extracted and formatted as: Principal=AppID, Credentials="timestamp:nonce:signature".
func (s *SignatureStrategy) Authenticate(ctx fiber.Ctx, _ map[string]any) (*security.Principal, error) {
	appID := ctx.Get(api.HeaderXAppID)
	timestampStr := ctx.Get(api.HeaderXTimestamp)
	nonce := ctx.Get(api.HeaderXNonce)
	signature := ctx.Get(api.HeaderXSignature)

	if appID == "" {
		return nil, result.ErrAppIDRequired
	}

	if timestampStr == "" {
		return nil, result.ErrTimestampRequired
	}

	if nonce == "" {
		return nil, result.ErrNonceRequired
	}

	if signature == "" {
		return nil, result.ErrSignatureRequired
	}

	timestamp, err := cast.ToInt64E(timestampStr)
	if err != nil {
		return nil, result.ErrTimestampInvalid
	}

	return s.authManager.Authenticate(ctx.Context(), security.Authentication{
		Type:      isecurity.AuthTypeSignature,
		Principal: appID,
		Credentials: &security.SignatureCredentials{
			Timestamp: timestamp,
			Nonce:     nonce,
			Signature: signature,
		},
	})
}
