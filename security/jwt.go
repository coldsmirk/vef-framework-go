package security

import (
	"cmp"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/coldsmirk/vef-framework-go/result"
)

const (
	JWTIssuer          = "vef"                                                              // Issuer
	DefaultJWTAudience = "vef-app"                                                          // Audience
	DefaultJWTSecret   = "af6675678bd81ad7c93c4a51d122ef61e9750fe5d42ceac1c33b293f36bc14c2" // Secret
)

var jwtParseOptions = []jwt.ParserOption{
	jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
	jwt.WithIssuer(JWTIssuer),
	jwt.WithLeeway(10 * time.Second),
	jwt.WithIssuedAt(),
	jwt.WithExpirationRequired(),
}

// JWT provides low-level JWT token operations.
// It handles token generation, parsing, and validation without business logic.
type JWT struct {
	config *JWTConfig
	secret []byte
}

// NewJWT creates a new JWT instance with the given configuration.
// Secret expects a hex-encoded string; invalid hex will cause an error.
// Audience will be defaulted when empty.
func NewJWT(config *JWTConfig) (*JWT, error) {
	secret, err := hex.DecodeString(cmp.Or(config.Secret, DefaultJWTSecret))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecodeJWTSecretFailed, err)
	}

	config.Audience = cmp.Or(config.Audience, DefaultJWTAudience)

	return &JWT{
		config: config,
		secret: secret,
	}, nil
}

// Generate creates a JWT token with the given claims and expires.
// The expiration is computed as now + expires; iat and nbf are set to now.
func (j *JWT) Generate(claimsBuilder *JWTClaimsBuilder, expires, notBefore time.Duration) (string, error) {
	claims := claimsBuilder.build()
	// Set standard claims
	now := time.Now()
	claims[claimIssuer] = JWTIssuer
	claims[claimAudience] = j.config.Audience
	claims[claimIssuedAt] = now.Unix()
	claims[claimNotBefore] = now.Add(notBefore).Unix()
	claims[claimExpiresAt] = now.Add(expires).Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(j.secret)
}

// Parse parses and validates a JWT token.
// It returns a read-only claims accessor which performs safe conversions and never panics.
func (j *JWT) Parse(tokenString string) (*JWTClaimsAccessor, error) {
	options := append(slices.Clone(jwtParseOptions), jwt.WithAudience(j.config.Audience))

	token, err := jwt.NewParser(options...).
		Parse(
			tokenString,
			func(_ *jwt.Token) (any, error) {
				return j.secret, nil
			},
		)
	if err != nil {
		return nil, mapJWTError(err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, result.ErrTokenInvalid
	}

	return NewJWTClaimsAccessor(claims), nil
}

// mapJWTError maps JWT library errors to framework errors.
func mapJWTError(err error) error {
	switch {
	case errors.Is(err, jwt.ErrTokenExpired):
		return result.ErrTokenExpired
	case errors.Is(err, jwt.ErrTokenNotValidYet):
		return result.ErrTokenNotValidYet
	case errors.Is(err, jwt.ErrTokenInvalidIssuer):
		return result.ErrTokenInvalidIssuer
	case errors.Is(err, jwt.ErrTokenInvalidAudience):
		return result.ErrTokenInvalidAudience
	default:
		return result.ErrTokenInvalid
	}
}
