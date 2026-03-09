package security

import (
	"context"
	"time"

	"github.com/coldsmirk/vef-framework-go/internal/log"
)

var logger = log.Named("security")

// AuthTokens holds the access and refresh token pair issued after successful authentication.
type AuthTokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// Authentication carries the client-supplied authentication payload.
type Authentication struct {
	Type        string `json:"type"`
	Principal   string `json:"principal"`
	Credentials any    `json:"credentials"`
}

// ExternalAppConfig holds configuration for an external application.
type ExternalAppConfig struct {
	Enabled     bool   `json:"enabled"`
	IPWhitelist string `json:"ipWhitelist"`
}

// Authenticator validates credentials and returns a Principal on success.
// Multiple authenticators can be registered to support different authentication methods
// (e.g., JWT, password, OAuth). Each authenticator declares which authentication
// kinds it supports via the Supports method.
type Authenticator interface {
	// Supports returns true if this authenticator can handle the given authentication type.
	Supports(authType string) bool
	// Authenticate validates the credentials and returns the authenticated Principal.
	Authenticate(ctx context.Context, authentication Authentication) (*Principal, error)
}

// TokenGenerator creates access and refresh tokens for an authenticated Principal.
// Used after successful authentication to issue JWT tokens or similar credentials.
type TokenGenerator interface {
	// Generate creates a new token pair for the given Principal.
	Generate(principal *Principal) (*AuthTokens, error)
}

// AuthManager orchestrates authentication by delegating to registered Authenticators.
// It iterates through available authenticators to find one that supports the
// authentication kind and can successfully validate the credentials.
type AuthManager interface {
	// Authenticate finds a suitable authenticator and validates the credentials.
	Authenticate(ctx context.Context, authentication Authentication) (*Principal, error)
}

// UserLoader retrieves user information for authentication and authorization.
// Implementations typically query a database or external identity provider.
type UserLoader interface {
	// LoadByUsername retrieves a user by username, returning the Principal,
	// hashed password, and any error. Used for password-based authentication.
	LoadByUsername(ctx context.Context, username string) (*Principal, string, error)
	// LoadByID retrieves a user by their unique identifier.
	// Used for token refresh and session validation.
	LoadByID(ctx context.Context, id string) (*Principal, error)
}

// ExternalAppLoader retrieves external application credentials for API authentication.
// Used by OpenAPI authenticator to validate app-based signature authentication.
type ExternalAppLoader interface {
	// LoadByID retrieves an external app by its ID, returning the Principal,
	// secret key for signature verification, and any error.
	LoadByID(ctx context.Context, id string) (*Principal, string, error)
}

// PasswordDecryptor decrypts client-side encrypted passwords before verification.
// Used when passwords are encrypted during transmission for additional security.
type PasswordDecryptor interface {
	// Decrypt transforms an encrypted password back to plaintext for verification.
	Decrypt(encryptedPassword string) (string, error)
}

// NonceStore manages nonce lifecycle for replay attack prevention.
// Stores used nonces with TTL to detect and reject duplicate requests.
// Implementations must be thread-safe for concurrent access.
type NonceStore interface {
	// StoreIfAbsent atomically stores a nonce only when it does not already exist.
	// It returns true when the nonce was newly stored, false when the nonce already existed.
	// The TTL should be slightly longer than timestamp tolerance to ensure
	// nonces remain valid while their corresponding timestamps are accepted.
	StoreIfAbsent(ctx context.Context, appID, nonce string, ttl time.Duration) (bool, error)
}
