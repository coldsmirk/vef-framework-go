package security

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cast"
)

// Custom and standard JWT claim keys.
// Short keys are used for custom claims to keep token size small.
const (
	claimJWTID     = "jti" // JWT ID
	claimSubject   = "sub" // Subject
	claimIssuer    = "iss" // Issuer
	claimAudience  = "aud" // Audience
	claimIssuedAt  = "iat" // Issued At
	claimNotBefore = "nbf" // Not Before
	claimExpiresAt = "exp" // Expires At
	claimType      = "typ" // Token Type
	claimRoles     = "rls" // User Roles
	claimDetails   = "det" // User Details
)

// JWTConfig is the configuration for the JWT token.
type JWTConfig struct {
	Secret   string `config:"secret"`   // Secret key for JWT signing
	Audience string `config:"audience"` // JWT audience
}

// JWTClaimsBuilder helps build JWT claims for different token types.
type JWTClaimsBuilder struct {
	claims jwt.MapClaims
}

// NewJWTClaimsBuilder creates a new JWT claims builder.
func NewJWTClaimsBuilder() *JWTClaimsBuilder {
	return &JWTClaimsBuilder{
		claims: make(jwt.MapClaims),
	}
}

func (b *JWTClaimsBuilder) WithID(id string) *JWTClaimsBuilder {
	b.claims[claimJWTID] = id

	return b
}

// ID returns the JWT ID claim.
func (b *JWTClaimsBuilder) ID() (string, bool) {
	id, ok := b.claims[claimJWTID]

	return cast.ToString(id), ok
}

func (b *JWTClaimsBuilder) WithSubject(subject string) *JWTClaimsBuilder {
	b.claims[claimSubject] = subject

	return b
}

// Subject returns the subject claim.
func (b *JWTClaimsBuilder) Subject() (string, bool) {
	subject, ok := b.claims[claimSubject]

	return cast.ToString(subject), ok
}

func (b *JWTClaimsBuilder) WithRoles(roles []string) *JWTClaimsBuilder {
	b.claims[claimRoles] = roles

	return b
}

// Roles returns the roles claim.
func (b *JWTClaimsBuilder) Roles() ([]string, bool) {
	roles, ok := b.claims[claimRoles]

	return cast.ToStringSlice(roles), ok
}

func (b *JWTClaimsBuilder) WithDetails(details any) *JWTClaimsBuilder {
	b.claims[claimDetails] = details

	return b
}

// Details returns the details claim.
func (b *JWTClaimsBuilder) Details() (any, bool) {
	details, ok := b.claims[claimDetails]

	return details, ok
}

func (b *JWTClaimsBuilder) WithType(typ string) *JWTClaimsBuilder {
	b.claims[claimType] = typ

	return b
}

// Type returns the token type claim.
func (b *JWTClaimsBuilder) Type() (string, bool) {
	typ, ok := b.claims[claimType]

	return cast.ToString(typ), ok
}

func (b *JWTClaimsBuilder) WithClaim(key string, value any) *JWTClaimsBuilder {
	b.claims[key] = value

	return b
}

// Claim returns a custom claim.
func (b *JWTClaimsBuilder) Claim(key string) (any, bool) {
	claim, ok := b.claims[key]

	return claim, ok
}

// build returns the built claims.
func (b *JWTClaimsBuilder) build() jwt.MapClaims {
	return b.claims
}

type JWTClaimsAccessor struct {
	claims jwt.MapClaims
}

// NewJWTClaimsAccessor creates a new JWT claims accessor.
func NewJWTClaimsAccessor(claims jwt.MapClaims) *JWTClaimsAccessor {
	return &JWTClaimsAccessor{
		claims: claims,
	}
}

// ID returns the JWT ID claim.
// Returns empty string if the claim is missing or not a string.
func (a *JWTClaimsAccessor) ID() string {
	return cast.ToString(a.claims[claimJWTID])
}

// Subject returns the subject claim.
// Returns empty string if the claim is missing or not a string.
func (a *JWTClaimsAccessor) Subject() string {
	return cast.ToString(a.claims[claimSubject])
}

// Roles returns the roles claim.
// Supports both []string and []any payloads; returns empty slice if absent.
func (a *JWTClaimsAccessor) Roles() []string {
	return cast.ToStringSlice(a.claims[claimRoles])
}

// Details returns the details claim.
func (a *JWTClaimsAccessor) Details() any {
	return a.claims[claimDetails]
}

// Type returns the token type claim.
// Returns empty string if the claim is missing or not a string.
func (a *JWTClaimsAccessor) Type() string {
	return cast.ToString(a.claims[claimType])
}

// Claim returns the claim.
func (a *JWTClaimsAccessor) Claim(key string) any {
	return a.claims[key]
}
