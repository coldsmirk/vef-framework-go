package auth

import (
	"crypto/subtle"
	"encoding/base64"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/contextx"
	isecurity "github.com/coldsmirk/vef-framework-go/internal/security"
	"github.com/coldsmirk/vef-framework-go/security"
)

// basicAuthPrefix is the RFC 7617 Authorization scheme prefix.
const basicAuthPrefix = "Basic "

// HTTPBasicStrategy implements api.AuthStrategy for HTTP Basic authentication
// (RFC 7617): the request presents "Authorization: Basic base64(user:pass)"
// and the strategy resolves the username through the
// security.BasicAccountLoader, comparing the stored secret in constant time.
//
// Every authentication failure denies with
// security.ErrBasicCredentialsInvalid (fail closed) without revealing whether
// the username or the password was wrong. Loader errors propagate as-is: an
// unavailable backing store is an infrastructure fault, not a credential
// rejection.
type HTTPBasicStrategy struct {
	loader security.BasicAccountLoader
}

// NewHTTPBasic creates a new HTTP Basic authentication strategy. The loader
// is optional: when the application registers no security.BasicAccountLoader,
// the strategy falls back to the configuration-backed loader serving
// vef.security.basic_accounts (whose construction validates the configured
// accounts eagerly and fails start-up on a fault).
func NewHTTPBasic(loader security.BasicAccountLoader, cfg *config.SecurityConfig) (api.AuthStrategy, error) {
	if loader == nil {
		var err error
		if loader, err = isecurity.NewConfigBasicAccountLoader(cfg); err != nil {
			return nil, err
		}
	}

	return &HTTPBasicStrategy{loader: loader}, nil
}

// Name returns the strategy name.
func (*HTTPBasicStrategy) Name() string {
	return api.AuthStrategyHTTPBasic
}

// Authenticate decodes the Basic credentials, resolves the username through
// the loader, and compares the stored secret in constant time.
func (s *HTTPBasicStrategy) Authenticate(ctx fiber.Ctx, _ map[string]any) (*security.Principal, error) {
	username, password, ok := decodeBasicCredentials(ctx.Get(fiber.HeaderAuthorization))
	if !ok {
		return nil, security.ErrBasicCredentialsInvalid
	}

	principal, secret, err := s.loader.LoadByUsername(ctx.Context(), username)
	if err != nil {
		return nil, err
	}

	if principal == nil || secret == "" {
		contextx.Logger(ctx, logger).Warnf("HTTP Basic authentication failed: unknown account %q", username)

		return nil, security.ErrBasicCredentialsInvalid
	}

	if subtle.ConstantTimeCompare([]byte(secret), []byte(password)) != 1 {
		contextx.Logger(ctx, logger).Warnf("HTTP Basic authentication failed: secret mismatch for account %q", username)

		return nil, security.ErrBasicCredentialsInvalid
	}

	return principal, nil
}

// decodeBasicCredentials extracts the username and password from an
// "Authorization: Basic" header value. The scheme comparison is
// case-insensitive per RFC 7617; the username must be non-empty and must not
// contain a colon (the credential separator).
func decodeBasicCredentials(header string) (username, password string, ok bool) {
	if len(header) <= len(basicAuthPrefix) || !strings.EqualFold(header[:len(basicAuthPrefix)], basicAuthPrefix) {
		return "", "", false
	}

	decoded, err := base64.StdEncoding.DecodeString(header[len(basicAuthPrefix):])
	if err != nil {
		return "", "", false
	}

	username, password, found := strings.Cut(string(decoded), ":")
	if !found || username == "" {
		return "", "", false
	}

	return username, password, true
}
