package security

// Authentication constants.
const (
	AuthSchemeBearer    = "Bearer"
	QueryKeyAccessToken = "__accessToken"
)

// JWT token type constants.
const (
	TokenTypeAccess    = "access"
	TokenTypeRefresh   = "refresh"
	TokenTypeChallenge = "challenge"
)
