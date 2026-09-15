package security

// Authentication constants.
const (
	AuthSchemeBearer    = "Bearer"
	QueryKeyAccessToken = "__accessToken"
)

// Login mechanism constants: the authentication types of the logins the
// framework authenticates itself. Any other login mechanism is host-defined: a
// type an Authenticator the host registers supports.
const (
	AuthTypePassword  = "password"
	AuthTypeTrustCode = "trust_code"
)

// JWT token type constants.
const (
	TokenTypeAccess    = "access"
	TokenTypeRefresh   = "refresh"
	TokenTypeChallenge = "challenge"
)
